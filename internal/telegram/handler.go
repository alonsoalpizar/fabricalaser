package telegram

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// Handler atiende el webhook de Telegram Bot API.
type Handler struct {
	processor *Processor
}

// NewHandler construye el Handler con el procesador de mensajes.
func NewHandler(processor *Processor) *Handler {
	return &Handler{processor: processor}
}

// HandleWebhook maneja el POST que Telegram envía con cada Update.
// Responde 200 inmediatamente y procesa el mensaje de forma asíncrona.
// Telegram valida por el token secreto en la URL del webhook — no hay firma HMAC.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("telegram: error leyendo body", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		slog.Error("telegram: error parseando update", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// ACK inmediato a Telegram
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))

	// Procesar de forma asíncrona
	go h.processor.Process(context.Background(), &update)
}
