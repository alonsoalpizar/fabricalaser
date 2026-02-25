package quote

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/alonsoalpizar/fabricalaser/internal/services/pricing"
	"github.com/alonsoalpizar/fabricalaser/internal/services/svgengine"
	"github.com/go-chi/chi/v5"
)

const maxSVGSize = 5 * 1024 * 1024 // 5MB max SVG file size

// Handler handles quote-related HTTP requests
type Handler struct {
	svgAnalysisRepo *repository.SVGAnalysisRepository
	quoteRepo       *repository.QuoteRepository
	userRepo        *repository.UserRepository
	analyzer        *svgengine.Analyzer
	configLoader    *pricing.ConfigLoader
	calculator      *pricing.Calculator
}

// NewHandler creates a new quote handler
func NewHandler() *Handler {
	db := database.Get()
	configLoader := pricing.NewConfigLoader(db)
	return &Handler{
		svgAnalysisRepo: repository.NewSVGAnalysisRepository(),
		quoteRepo:       repository.NewQuoteRepository(),
		userRepo:        repository.NewUserRepository(),
		analyzer:        svgengine.NewAnalyzer(),
		configLoader:    configLoader,
		calculator:      pricing.NewCalculator(configLoader),
	}
}

// AnalyzeSVG handles POST /api/v1/quotes/analyze
// Uploads and analyzes an SVG file
func (h *Handler) AnalyzeSVG(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uint)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxSVGSize); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "Error parsing form data")
		return
	}

	// Get the SVG file
	file, header, err := r.FormFile("svg")
	if err != nil {
		respondError(w, http.StatusBadRequest, "NO_FILE", "No SVG file provided")
		return
	}
	defer file.Close()

	// Validate file extension
	filename := header.Filename
	if !strings.HasSuffix(strings.ToLower(filename), ".svg") {
		respondError(w, http.StatusBadRequest, "INVALID_FILE_TYPE", "File must be an SVG")
		return
	}

	// Read file content
	svgContent, err := io.ReadAll(io.LimitReader(file, maxSVGSize))
	if err != nil {
		respondError(w, http.StatusBadRequest, "READ_ERROR", "Error reading file")
		return
	}

	// Basic SVG validation
	contentStr := string(svgContent)
	if !strings.Contains(contentStr, "<svg") {
		respondError(w, http.StatusBadRequest, "INVALID_SVG", "File does not appear to be a valid SVG")
		return
	}

	// Check for duplicate (same file hash)
	fileHash := svgengine.CalculateFileHash(contentStr)
	existingAnalysis, _ := h.svgAnalysisRepo.FindByFileHash(userID, fileHash)
	if existingAnalysis != nil {
		// Return existing analysis instead of creating duplicate
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"data":       existingAnalysis.ToSummary(),
			"cached":     true,
			"message":    "Este archivo ya fue analizado previamente",
		})
		return
	}

	// Analyze the SVG
	result, err := h.analyzer.Analyze(contentStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ANALYSIS_ERROR", "Error analyzing SVG: "+err.Error())
		return
	}

	// Convert to model and save
	analysis := h.analyzer.ToModel(result, userID, filename, contentStr)
	if err := h.svgAnalysisRepo.Create(analysis); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error saving analysis")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":    analysis.ToSummary(),
		"cached":  false,
		"message": "SVG analizado correctamente",
	})
}

// CalculateRequest represents the request body for price calculation
type CalculateRequest struct {
	AnalysisID    uint    `json:"analysis_id"`
	TechnologyID  uint    `json:"technology_id"`
	MaterialID    uint    `json:"material_id"`
	EngraveTypeID uint    `json:"engrave_type_id"`
	Quantity      int     `json:"quantity"`
	Thickness     float64 `json:"thickness,omitempty"`
}

// CalculatePrice handles POST /api/v1/quotes/calculate
// Calculates pricing for an analysis with given options
func (h *Handler) CalculatePrice(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uint)

	var req CalculateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	// Validate required fields
	if req.AnalysisID == 0 || req.TechnologyID == 0 || req.MaterialID == 0 || req.EngraveTypeID == 0 {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "analysis_id, technology_id, material_id, and engrave_type_id are required")
		return
	}

	if req.Quantity < 1 {
		req.Quantity = 1
	}

	// Get the analysis
	analysis, err := h.svgAnalysisRepo.FindByID(req.AnalysisID)
	if err != nil {
		respondError(w, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "SVG analysis not found")
		return
	}

	// Verify ownership
	if analysis.UserID != userID {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "No tiene permiso para usar este análisis")
		return
	}

	// Calculate pricing (uses DB config, NO hardcode)
	priceResult, err := h.calculator.Calculate(analysis, req.TechnologyID, req.MaterialID, req.EngraveTypeID, req.Quantity)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "CALC_ERROR", "Error calculating price: "+err.Error())
		return
	}

	// Create and save quote
	quote := h.calculator.ToQuoteModel(
		priceResult,
		userID,
		analysis.ID,
		req.TechnologyID,
		req.MaterialID,
		req.EngraveTypeID,
		req.Quantity,
		req.Thickness,
	)

	if err := h.quoteRepo.Create(quote); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error saving quote")
		return
	}

	// Increment user's quotes used
	h.userRepo.IncrementQuotesUsed(userID)

	// Load relations for response
	quote, _ = h.quoteRepo.FindByIDWithRelations(quote.ID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":    quote.ToDetailedJSON(),
		"message": "Cotización calculada correctamente",
	})
}

// GetQuote handles GET /api/v1/quotes/:id
// Returns a specific quote
func (h *Handler) GetQuote(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uint)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "Invalid quote ID")
		return
	}

	quote, err := h.quoteRepo.FindByIDWithRelations(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Quote not found")
		return
	}

	// Check ownership (unless admin)
	user, _ := h.userRepo.FindByID(userID)
	if quote.UserID != userID && !user.IsAdmin() {
		respondError(w, http.StatusForbidden, "FORBIDDEN", "No tiene permiso para ver esta cotización")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": quote.ToDetailedJSON(),
	})
}

// GetMyQuotes handles GET /api/v1/quotes/my
// Returns current user's quotes
func (h *Handler) GetMyQuotes(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uint)

	// Parse pagination
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	quotes, err := h.quoteRepo.FindByUserIDWithRelations(userID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error fetching quotes")
		return
	}

	// Convert to summary list
	list := make([]map[string]interface{}, 0, len(quotes))
	for _, q := range quotes {
		list = append(list, q.ToSummary())
	}

	// Get total count
	total, _ := h.quoteRepo.CountByUser(userID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":   list,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetMyAnalyses handles GET /api/v1/quotes/analyses
// Returns current user's SVG analyses
func (h *Handler) GetMyAnalyses(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uint)

	// Parse pagination
	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	analyses, err := h.svgAnalysisRepo.FindByUserID(userID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error fetching analyses")
		return
	}

	// Convert to summary list
	list := make([]map[string]interface{}, 0, len(analyses))
	for _, a := range analyses {
		list = append(list, a.ToSummary())
	}

	// Get total count
	total, _ := h.svgAnalysisRepo.CountByUser(userID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":   list,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Helper functions for responses
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
