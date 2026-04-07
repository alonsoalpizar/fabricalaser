package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
)

// Handler agrupa las dependencias necesarias para el webhook de WhatsApp.
// Se inicializa una vez y se reutiliza en cada request.
type Handler struct {
	appSecret   string // WHATSAPP_APP_SECRET — para verificar firma X-Hub-Signature-256
	verifyToken string // WHATSAPP_VERIFY_TOKEN — para el handshake inicial con Meta
	processor   *MessageProcessor
}

// NewHandler construye el Handler leyendo configuración exclusivamente de variables de entorno.
// rateLimiter es opcional (puede ser nil para deshabilitar el rate limiting).
func NewHandler(redisClient RedisClient, pgClient PGClient, geminiCaller GeminiCaller, rateLimiter *RateLimiter, contextProvider *WAContextProvider) *Handler {
	return &Handler{
		appSecret:   os.Getenv("WHATSAPP_APP_SECRET"),
		verifyToken: os.Getenv("WHATSAPP_VERIFY_TOKEN"),
		processor:   NewMessageProcessor(redisClient, pgClient, geminiCaller, rateLimiter, contextProvider),
	}
}

// VerifyWebhook maneja el GET que Meta envía al configurar el webhook.
// Meta espera recibir el hub.challenge de vuelta si el verify_token coincide.
func (h *Handler) VerifyWebhook(w http.ResponseWriter, r *http.Request) {
	mode      := r.URL.Query().Get("hub.mode")
	token     := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode != "subscribe" || token != h.verifyToken {
		slog.Warn("whatsapp: verificación fallida",
			"mode", mode,
			"token_match", token == h.verifyToken,
		)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	slog.Info("whatsapp: webhook verificado exitosamente")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(challenge))
}

// HandleMessage maneja el POST que Meta envía con cada mensaje entrante.
// Responde 200 inmediatamente a Meta (obligatorio en <5s) y procesa de forma asíncrona.
func (h *Handler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	// 1. Leer body completo para verificar firma
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("whatsapp: error leyendo body", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 2. Verificar firma HMAC-SHA256 — seguridad obligatoria
	if !h.verifySignature(r.Header.Get("X-Hub-Signature-256"), body) {
		slog.Warn("whatsapp: firma inválida — posible request no autorizado")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 3. Parsear payload de Meta
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("whatsapp: error parseando payload", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 4. ACK inmediato a Meta — Meta requiere respuesta en menos de 5 segundos
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))

	// 5. Procesar mensaje de forma asíncrona — usar Background para que el contexto
	// no se cancele cuando el handler HTTP retorna el 200 a Meta
	go h.processor.Process(context.Background(), &payload)
}

// verifySignature valida la firma HMAC-SHA256 que Meta adjunta en cada request.
func (h *Handler) verifySignature(signature string, body []byte) bool {
	if len(signature) < 7 || signature[:7] != "sha256=" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.appSecret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expected))
}
