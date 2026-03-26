package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// ─── Tipos del Payload entrante de Meta ─────────────────────────────────────

// WebhookPayload es la estructura raíz que Meta envía en cada notificación.
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Value ChangeValue `json:"value"`
	Field string      `json:"field"`
}

type ChangeValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Messages         []Message `json:"messages"`
	Statuses         []Status  `json:"statuses"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type Message struct {
	From      string   `json:"from"`
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	Type      string   `json:"type"`
	Text      *TextMsg `json:"text,omitempty"`
}

type TextMsg struct {
	Body string `json:"body"`
}

type Status struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	RecipientID string `json:"recipient_id"`
}

// ─── Cliente para enviar mensajes vía Meta Cloud API ────────────────────────

// Sender encapsula el cliente HTTP y las credenciales para enviar mensajes.
type Sender struct {
	phoneNumberID string
	accessToken   string
	apiVersion    string
	httpClient    *http.Client
}

// NewSender construye el Sender leyendo credenciales de variables de entorno.
func NewSender() *Sender {
	version := os.Getenv("WHATSAPP_API_VERSION")
	if version == "" {
		version = "v22.0"
	}

	return &Sender{
		phoneNumberID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"),
		accessToken:   os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		apiVersion:    version,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SendText envía un mensaje de texto plano al número destinatario en formato E.164.
func (s *Sender) SendText(ctx context.Context, to, text string) error {
	url := fmt.Sprintf(
		"https://graph.facebook.com/%s/%s/messages",
		s.apiVersion,
		s.phoneNumberID,
	)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text": map[string]string{
			"preview_url": "false",
			"body":        text,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sender: error serializando payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sender: error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sender: error enviando mensaje: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sender: Meta respondió con status %d", resp.StatusCode)
	}

	slog.Info("whatsapp: mensaje enviado exitosamente", "to", to)
	return nil
}
