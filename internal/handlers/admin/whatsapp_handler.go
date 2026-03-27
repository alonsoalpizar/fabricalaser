package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/alonsoalpizar/fabricalaser/internal/whatsapp"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type WhatsappHandler struct {
	waRepo *repository.WhatsappRepository
	rc     *redis.Client
}

func NewWhatsappHandler(rc *redis.Client) *WhatsappHandler {
	return &WhatsappHandler{
		waRepo: repository.NewWhatsappRepository(),
		rc:     rc,
	}
}

// ─────────────────────────────────────────────
// Sessions (paginated, filterable)
// ─────────────────────────────────────────────

// GET /api/v1/admin/whatsapp/sessions?q=&from=&to=&page=1&limit=20
func (h *WhatsappHandler) GetSessions(w http.ResponseWriter, r *http.Request) {
	filter := repository.SessionFilter{
		Query: r.URL.Query().Get("q"),
		From:  r.URL.Query().Get("from"),
		To:    r.URL.Query().Get("to"),
		Page:  queryInt(r, "page", 1),
		Limit: queryInt(r, "limit", 20),
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	page, err := h.waRepo.GetSessions(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error al obtener sesiones")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": page})
}

// GET /api/v1/admin/whatsapp/sessions/{phone}/{date}
func (h *WhatsappHandler) GetSessionMessages(w http.ResponseWriter, r *http.Request) {
	rawPhone := chi.URLParam(r, "phone")
	phone, err := url.QueryUnescape(rawPhone)
	if err != nil {
		phone = rawPhone
	}
	date := chi.URLParam(r, "date") // YYYY-MM-DD

	if phone == "" || date == "" {
		respondError(w, http.StatusBadRequest, "MISSING_PARAMS", "Teléfono y fecha requeridos")
		return
	}

	messages, err := h.waRepo.GetSessionMessages(phone, date)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error al obtener mensajes")
		return
	}
	if messages == nil {
		messages = []repository.ConversationMessage{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": messages})
}

// ─────────────────────────────────────────────
// Purge
// ─────────────────────────────────────────────

// POST /api/v1/admin/whatsapp/purge
// Body: { "older_than_days": 30, "dry_run": true }
func (h *WhatsappHandler) PurgeConversations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OlderThanDays int  `json:"older_than_days"`
		DryRun        bool `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OlderThanDays < 1 {
		respondError(w, http.StatusBadRequest, "INVALID_INPUT", "older_than_days debe ser mayor a 0")
		return
	}

	if req.DryRun {
		count, err := h.waRepo.CountOlderThan(req.OlderThanDays)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"count":   count,
				"dry_run": true,
				"message": fmt.Sprintf("Se eliminarían %d mensajes con más de %d días", count, req.OlderThanDays),
			},
		})
		return
	}

	deleted, err := h.waRepo.PurgeOlderThan(req.OlderThanDays)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"deleted": deleted,
			"dry_run": false,
			"message": fmt.Sprintf("Se eliminaron %d mensajes", deleted),
		},
	})
}

// ─────────────────────────────────────────────
// Legacy endpoints (kept for backward compat)
// ─────────────────────────────────────────────

// GET /api/v1/admin/whatsapp/conversations
func (h *WhatsappHandler) GetConversations(w http.ResponseWriter, r *http.Request) {
	contacts, err := h.waRepo.GetContactSummaries()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error al obtener conversaciones")
		return
	}
	if contacts == nil {
		contacts = []repository.ContactSummary{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": contacts})
}

// GET /api/v1/admin/whatsapp/conversations/{phone}
func (h *WhatsappHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	rawPhone := chi.URLParam(r, "phone")
	phone, err := url.QueryUnescape(rawPhone)
	if err != nil {
		phone = rawPhone
	}
	if phone == "" {
		respondError(w, http.StatusBadRequest, "MISSING_PHONE", "Número de teléfono requerido")
		return
	}
	messages, err := h.waRepo.GetConversation(phone)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", "Error al obtener conversación")
		return
	}
	if messages == nil {
		messages = []repository.ConversationMessage{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": messages})
}

// POST /api/v1/admin/whatsapp/digest/send
func (h *WhatsappHandler) SendDigest(w http.ResponseWriter, r *http.Request) {
	if err := whatsapp.SendDigest(h.rc); err != nil {
		respondError(w, http.StatusInternalServerError, "MAIL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Resumen enviado a info@fabricalaser.com",
	})
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}
