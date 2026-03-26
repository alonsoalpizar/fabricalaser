package admin

import (
	"net/http"
	"net/url"

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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    contacts,
	})
}

// POST /api/v1/admin/whatsapp/digest/send — dispara el resumen por email de inmediato
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

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    messages,
	})
}
