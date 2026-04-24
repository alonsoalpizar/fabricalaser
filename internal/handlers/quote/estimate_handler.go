package quote

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/services/pricing"
)

// EstimateRequest — lo que recibe el endpoint desde el tool de Gemini.
// Usa nombres en español para que el tool de Gemini sea legible y natural.
type EstimateRequest struct {
	AltoCM           float64 `json:"alto_cm"`             // Alto del área a grabar/cortar
	AnchoCM          float64 `json:"ancho_cm"`            // Ancho del área a grabar/cortar
	Cantidad         int     `json:"cantidad"`            // Unidades a producir
	TechnologyID     uint    `json:"technology_id"`       // ID de tecnología (de la BD)
	MaterialID       uint    `json:"material_id"`         // ID de material (de la BD)
	EngraveTypeID    uint    `json:"engrave_type_id"`     // ID de tipo de grabado (de la BD)
	Thickness        float64 `json:"thickness,omitempty"`         // Grosor en mm (opcional, default 3.0)
	MaterialIncluded bool    `json:"material_included"`           // true = FabricaLaser provee el material
	IncluyeCorte     bool    `json:"incluye_corte"`               // true = incluir perímetro de corte
	CutTechnologyID  *uint   `json:"cut_technology_id,omitempty"` // nil = misma tech para corte
	IgnoreCutLines   bool    `json:"ignore_cut_lines,omitempty"`  // true = material no cortable
}

// EstimateResponse — lo que retorna el endpoint al tool de Gemini.
type EstimateResponse struct {
	PrecioEstimado   float64 `json:"precio_estimado"`           // Precio final en CRC
	PrecioUnitario   float64 `json:"precio_unitario"`           // Precio por unidad
	AreaCM2          float64 `json:"area_cm2"`                  // Área calculada
	DescuentoVolumen float64 `json:"descuento_volumen"`         // % de descuento aplicado
	Tecnologia       string  `json:"tecnologia"`                // Nombre de la tecnología
	Material         string  `json:"material"`                  // Nombre del material
	Advertencia      string  `json:"advertencia,omitempty"`     // Mensaje si algo requiere revisión
	Error            string  `json:"error,omitempty"`
}

// HandleEstimate es el handler del endpoint POST /api/v1/quotes/estimate.
// No requiere JWT — es llamado internamente por el tool de Gemini via INTERNAL_API_TOKEN.
// El resultado no se persiste en DB; es solo un precio de referencia para WhatsApp.
func (h *Handler) HandleEstimate(w http.ResponseWriter, r *http.Request) {
	// Verificar token interno — este endpoint no usa JWT
	internalToken := os.Getenv("INTERNAL_API_TOKEN")
	if internalToken != "" {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || authHeader[7:] != internalToken {
			sendEstimateError(w, "No autorizado", http.StatusUnauthorized)
			return
		}
	}

	var req EstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendEstimateError(w, "Solicitud inválida", http.StatusBadRequest)
		return
	}

	// Validaciones
	if req.AltoCM <= 0 || req.AnchoCM <= 0 {
		sendEstimateError(w, "Las medidas deben ser mayores a 0", http.StatusBadRequest)
		return
	}
	if req.AltoCM > 100 || req.AnchoCM > 100 {
		sendEstimateError(w, "Medidas fuera del rango de trabajo (máx 100cm)", http.StatusBadRequest)
		return
	}
	if req.TechnologyID == 0 || req.MaterialID == 0 {
		sendEstimateError(w, "technology_id y material_id son requeridos", http.StatusBadRequest)
		return
	}
	if req.Cantidad < 1 {
		req.Cantidad = 1
	}
	if req.Thickness <= 0 {
		req.Thickness = 3.0 // grosor más común
	}
	// Solo aplicar default cuando NO es un trabajo de solo corte.
	// Si incluye_corte=true y engraveTypeID=0 → solo corte, sin grabado.
	// buildSyntheticAnalysis trata engraveTypeID=0 como "sin grabado".
	if req.EngraveTypeID == 0 && !req.IncluyeCorte {
		req.EngraveTypeID = 1 // default: grabado vectorial para trabajos sin corte
	}

	// Convertir cm → mm (el Calculator trabaja en mm)
	altoMM := req.AltoCM * 10
	anchoMM := req.AnchoCM * 10

	// Construir SVGAnalysis sintético desde las medidas
	analysis := pricing.BuildSyntheticAnalysis(altoMM, anchoMM, req.IncluyeCorte, req.EngraveTypeID)

	// Llamar al Calculator sin modificar su lógica
	priceResult, err := h.calculator.Calculate(
		analysis,
		req.TechnologyID,
		req.MaterialID,
		req.EngraveTypeID,
		req.Thickness,
		req.Cantidad,
		req.MaterialIncluded,
		req.CutTechnologyID,
		req.IgnoreCutLines,
	)
	if err != nil {
		sendEstimateError(w, "Error calculando precio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// PriceFinal = MAX(Hybrid, Value) — igual que ToQuoteModel
	priceFinal := math.Max(priceResult.PriceHybridTotal, priceResult.PriceValueTotal)
	precioUnitario := math.Round(priceFinal / float64(req.Cantidad))

	resp := EstimateResponse{
		PrecioEstimado:   math.Round(priceFinal),
		PrecioUnitario:   precioUnitario,
		AreaCM2:          req.AltoCM * req.AnchoCM,
		DescuentoVolumen: priceResult.DiscountVolumePct,
	}

	// Advertencia si el trabajo necesita revisión humana
	if priceResult.Status == models.QuoteStatusNeedsReview {
		resp.Advertencia = "Este trabajo requiere revisión de un asesor antes de confirmar precio final"
	} else if priceResult.Status == models.QuoteStatusRejected {
		resp.Advertencia = "Diseño complejo — precio de referencia solamente, requiere revisión"
	}

	// Obtener nombres desde config cacheada (sin hit extra a BD)
	if config, err := h.configLoader.Load(); err == nil {
		if tech := config.GetTechnology(req.TechnologyID); tech != nil {
			resp.Tecnologia = tech.Name
		}
		if mat := config.GetMaterial(req.MaterialID); mat != nil {
			resp.Material = mat.Name
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func sendEstimateError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(EstimateResponse{Error: msg})
}
