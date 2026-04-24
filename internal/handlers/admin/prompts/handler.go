// Package prompts (admin) expone los endpoints para el panel /admin/prompts.html
// donde el gestor edita los system prompts de los agentes LLM.
//
// Arquitectura:
//   - CRUD sobre agent_prompts + agent_prompt_versions vía repository
//   - Hot reload: cada PUT/POST que cambia body publica en canal Redis
//     "prompts:updated <agent_key>" — todos los provider en memoria invalidan
//     su cache. Cambio aplica en segundos a los 3 agentes.
//   - Sandbox: POST /sandbox construye un cliente LLM efímero con el body
//     enviado en el request (sin persistir) para que el gestor itere
//     probando antes de guardar.
//   - R3 (del plan): NUNCA retorna api_key del factory. Sandbox usa el cliente
//     ACTIVO del factory, no uno construido con credenciales nuevas.
package prompts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/agent/llm"
	agentprompts "github.com/alonsoalpizar/fabricalaser/internal/agent/prompts"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"

	"github.com/go-chi/chi/v5"
)

// Handler agrupa las dependencias: repo para CRUD/versionado, provider para
// invalidación de cache, llmFactory para el sandbox.
type Handler struct {
	repo       *repository.AgentPromptRepository
	provider   *agentprompts.Provider
	llmFactory *llm.Factory
}

func NewHandler(repo *repository.AgentPromptRepository, provider *agentprompts.Provider, llmFactory *llm.Factory) *Handler {
	return &Handler{
		repo:       repo,
		provider:   provider,
		llmFactory: llmFactory,
	}
}

// =============================================================================
// GET /api/v1/admin/prompts — lista de metadata (sin body completo)
// R10: no retorna body para evitar cargar ~34KB en el listado.
// =============================================================================

type listItem struct {
	ID          uint      `json:"id"`
	AgentKey    string    `json:"agent_key"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	Version     int       `json:"version"`
	BodyLength  int       `json:"body_length"`
	UpdatedBy   *uint     `json:"updated_by,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	all, err := h.repo.FindAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error cargando prompts: "+err.Error())
		return
	}

	resp := make([]listItem, 0, len(all))
	for _, p := range all {
		resp = append(resp, listItem{
			ID:          p.ID,
			AgentKey:    p.AgentKey,
			Title:       p.Title,
			Description: p.Description,
			Version:     p.Version,
			BodyLength:  len(p.Body),
			UpdatedBy:   p.UpdatedBy,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
// GET /api/v1/admin/prompts/{agent_key} — detalle completo con body
// =============================================================================

func (h *Handler) GetOne(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "agent_key")
	p, err := h.repo.FindByKey(key)
	if err != nil {
		if errors.Is(err, repository.ErrAgentPromptNotFound) {
			writeError(w, http.StatusNotFound, "prompt no existe: "+key)
			return
		}
		writeError(w, http.StatusInternalServerError, "error: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// =============================================================================
// PUT /api/v1/admin/prompts/{agent_key} — guardar nueva versión
// Body: {body: string, note?: string}
// =============================================================================

type saveRequest struct {
	Body string  `json:"body"`
	Note *string `json:"note,omitempty"`
}

type saveResponse struct {
	OK        bool      `json:"ok"`
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Límites de validación del body.
// Min: 50 chars evita que el gestor guarde vacío por error (rompería agente).
// Max: 100000 chars es un margen cómodo — los prompts más largos hoy tienen ~34KB.
const (
	bodyMinLength = 50
	bodyMaxLength = 100000
)

func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "agent_key")

	p, err := h.repo.FindByKey(key)
	if err != nil {
		if errors.Is(err, repository.ErrAgentPromptNotFound) {
			writeError(w, http.StatusNotFound, "prompt no existe: "+key)
			return
		}
		writeError(w, http.StatusInternalServerError, "error: "+err.Error())
		return
	}

	var req saveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido: "+err.Error())
		return
	}

	// Validar longitud
	bodyLen := len(strings.TrimSpace(req.Body))
	if bodyLen < bodyMinLength {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("body demasiado corto (%d chars) — mínimo %d", bodyLen, bodyMinLength))
		return
	}
	if bodyLen > bodyMaxLength {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("body demasiado largo (%d chars) — máximo %d", bodyLen, bodyMaxLength))
		return
	}

	// userID viene del AuthMiddleware (context)
	userID, _ := r.Context().Value("userID").(uint)

	newVer, err := h.repo.SaveNewVersion(p.ID, req.Body, userID, req.Note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error guardando: "+err.Error())
		return
	}

	// Invalidar cache local (el provider del proceso actual)
	h.provider.Invalidate(key)

	// Publicar para invalidar cache en otros procesos / para auditoría
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.provider.PublishInvalidate(ctx, key); err != nil {
		// No fatal — la invalidación local ya se hizo. Logueamos y seguimos.
		fmt.Printf("prompts.admin: publish invalidate failed: %v\n", err)
	}

	writeJSON(w, http.StatusOK, saveResponse{
		OK:        true,
		Version:   newVer,
		UpdatedAt: time.Now().UTC(),
	})
}

// =============================================================================
// GET /api/v1/admin/prompts/{agent_key}/versions — lista de versiones (con preview)
// =============================================================================

type versionListItem struct {
	ID                  uint      `json:"id"`
	Version             int       `json:"version"`
	UpdatedBy           *uint     `json:"updated_by,omitempty"`
	Note                *string   `json:"note,omitempty"`
	RestoredFromVersion *int      `json:"restored_from_version,omitempty"`
	BodyPreview         string    `json:"body_preview"`
	BodyLength          int       `json:"body_length"`
	CreatedAt           time.Time `json:"created_at"`
}

func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "agent_key")

	p, err := h.repo.FindByKey(key)
	if err != nil {
		if errors.Is(err, repository.ErrAgentPromptNotFound) {
			writeError(w, http.StatusNotFound, "prompt no existe: "+key)
			return
		}
		writeError(w, http.StatusInternalServerError, "error: "+err.Error())
		return
	}

	// Default limit 20, con override ?limit=N (max 100)
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	versions, err := h.repo.FindVersions(p.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "error: "+err.Error())
		return
	}

	resp := make([]versionListItem, 0, len(versions))
	for _, v := range versions {
		resp = append(resp, versionListItem{
			ID:                  v.ID,
			Version:             v.Version,
			UpdatedBy:           v.UpdatedBy,
			Note:                v.Note,
			RestoredFromVersion: v.RestoredFromVersion,
			BodyPreview:         previewBody(v.Body, 200),
			BodyLength:          len(v.Body),
			CreatedAt:           v.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
// GET /api/v1/admin/prompts/{agent_key}/versions/{version} — versión específica
// =============================================================================

func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "agent_key")
	verStr := chi.URLParam(r, "version")
	ver, err := strconv.Atoi(verStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "version inválida")
		return
	}

	p, err := h.repo.FindByKey(key)
	if err != nil {
		if errors.Is(err, repository.ErrAgentPromptNotFound) {
			writeError(w, http.StatusNotFound, "prompt no existe: "+key)
			return
		}
		writeError(w, http.StatusInternalServerError, "error: "+err.Error())
		return
	}

	v, err := h.repo.FindVersion(p.ID, ver)
	if err != nil {
		if errors.Is(err, repository.ErrVersionNotFound) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("versión %d no existe para %s", ver, key))
			return
		}
		writeError(w, http.StatusInternalServerError, "error: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, v)
}

// =============================================================================
// POST /api/v1/admin/prompts/{agent_key}/rollback — restaurar versión anterior
// Body: {to_version: int, note?: string}
// Response incluye restored_from_version para que la UI muestre contexto claro
// =============================================================================

type rollbackRequest struct {
	ToVersion int     `json:"to_version"`
	Note      *string `json:"note,omitempty"`
}

type rollbackResponse struct {
	OK                  bool      `json:"ok"`
	NewVersion          int       `json:"new_version"`
	RestoredFromVersion int       `json:"restored_from_version"`
	Note                string    `json:"note"`
	BodyPreview         string    `json:"body_preview"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (h *Handler) Rollback(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "agent_key")

	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido: "+err.Error())
		return
	}

	if req.ToVersion < 1 {
		writeError(w, http.StatusBadRequest, "to_version debe ser >= 1")
		return
	}

	p, err := h.repo.FindByKey(key)
	if err != nil {
		if errors.Is(err, repository.ErrAgentPromptNotFound) {
			writeError(w, http.StatusNotFound, "prompt no existe: "+key)
			return
		}
		writeError(w, http.StatusInternalServerError, "error: "+err.Error())
		return
	}

	userID, _ := r.Context().Value("userID").(uint)

	result, err := h.repo.Rollback(p.ID, req.ToVersion, userID)
	if err != nil {
		if errors.Is(err, repository.ErrVersionNotFound) {
			writeError(w, http.StatusNotFound,
				fmt.Sprintf("versión %d no existe para %s", req.ToVersion, key))
			return
		}
		writeError(w, http.StatusInternalServerError, "error ejecutando rollback: "+err.Error())
		return
	}

	// Invalidar cache local + publicar
	h.provider.Invalidate(key)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	_ = h.provider.PublishInvalidate(ctx, key)

	writeJSON(w, http.StatusOK, rollbackResponse{
		OK:                  true,
		NewVersion:          result.NewVersion,
		RestoredFromVersion: result.RestoredFromVersion,
		Note:                fmt.Sprintf("Restaurado desde v%d", result.RestoredFromVersion),
		BodyPreview:         result.BodyPreview,
		UpdatedAt:           time.Now().UTC(),
	})
}

// =============================================================================
// POST /api/v1/admin/prompts/sandbox — probar un prompt sin guardar
// Body: {agent_key: string, body: string, test_message: string, history?: [...]}
// Response: {response, latency_ms, tokens_in, tokens_out, provider, model}
// Usa el cliente LLM ACTIVO del factory (NO construye uno nuevo — R3 de Paso 2).
// Timeout 15s, sin tools.
// =============================================================================

type sandboxRequest struct {
	AgentKey    string              `json:"agent_key"` // informativo, para logging
	Body        string              `json:"body"`      // el prompt a probar (no persiste)
	TestMessage string              `json:"test_message"`
	History     []sandboxHistoryMsg `json:"history,omitempty"` // para conversaciones multi-turn
}

type sandboxHistoryMsg struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

type sandboxResponse struct {
	Response  string `json:"response"`
	LatencyMS int64  `json:"latency_ms"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Error     string `json:"error,omitempty"`
}

func (h *Handler) Sandbox(w http.ResponseWriter, r *http.Request) {
	var req sandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Body) == "" {
		writeError(w, http.StatusBadRequest, "body (prompt) no puede estar vacío")
		return
	}
	if strings.TrimSpace(req.TestMessage) == "" {
		writeError(w, http.StatusBadRequest, "test_message no puede estar vacío")
		return
	}

	// Traducir history al formato llm.Message
	history := make([]llm.Message, 0, len(req.History)+1)
	for _, m := range req.History {
		role := llm.RoleUser
		if m.Role == "assistant" || m.Role == "model" {
			role = llm.RoleModel
		}
		history = append(history, llm.Message{Role: role, Content: m.Content})
	}
	// El test_message va como último mensaje del user
	history = append(history, llm.Message{Role: llm.RoleUser, Content: req.TestMessage})

	client := h.llmFactory.Client()

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := client.Chat(ctx, req.Body, history, nil)
	if err != nil {
		writeJSON(w, http.StatusOK, sandboxResponse{
			Provider: client.Provider(),
			Error:    err.Error(),
		})
		return
	}

	if resp == nil {
		writeJSON(w, http.StatusOK, sandboxResponse{
			Provider: client.Provider(),
			Error:    "respuesta vacía del modelo",
		})
		return
	}

	writeJSON(w, http.StatusOK, sandboxResponse{
		Response:  resp.Content,
		LatencyMS: resp.LatencyMS,
		TokensIn:  resp.TokensIn,
		TokensOut: resp.TokensOut,
		Provider:  resp.Provider,
		Model:     resp.Model,
	})
}

// =============================================================================
// Helpers
// =============================================================================

// previewBody retorna los primeros `n` caracteres del body, o el body completo
// si es más corto. Agrega "..." si hubo truncado.
func previewBody(body string, n int) string {
	if len(body) <= n {
		return body
	}
	return body[:n] + "..."
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Silencia warning de models no usado cuando compila con optimizaciones aggressive
var _ = models.AgentPrompt{}
