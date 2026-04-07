package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	tgMaxMessageLen = 4096
	tgHTTPTimeout   = 15 * time.Second
)

// Sender encapsula el token y el cliente HTTP para enviar mensajes via Telegram Bot API.
type Sender struct {
	botToken   string
	httpClient *http.Client
}

// NewSender construye el Sender leyendo TELEGRAM_BOT_TOKEN del entorno.
func NewSender() *Sender {
	return &Sender{
		botToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		httpClient: &http.Client{
			Timeout: tgHTTPTimeout,
		},
	}
}

// SendText envía un mensaje de texto a un chat de Telegram.
// Si el texto excede 4096 caracteres, lo parte en chunks por salto de línea.
func (s *Sender) SendText(ctx context.Context, chatID int64, text string) error {
	chunks := splitMessage(text, tgMaxMessageLen)
	for _, chunk := range chunks {
		if err := s.sendOneMessage(ctx, chatID, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sender) sendOneMessage(ctx context.Context, chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram sender: error serializando payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram sender: error creando request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram sender: error enviando mensaje: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sender: Telegram respondió con status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("telegram: mensaje enviado exitosamente", "chat_id", chatID)
	return nil
}

// GetFileBytes descarga un archivo de Telegram dado su file_id.
// Primero llama a getFile para obtener el file_path, luego descarga el binario.
// Retorna los bytes, el mimeType inferido, y error.
func (s *Sender) GetFileBytes(ctx context.Context, fileID string) ([]byte, string, error) {
	// 1. Obtener file_path via getFile
	getFileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", s.botToken, fileID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getFileURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("telegram getFile: error creando request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("telegram getFile: error en request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("telegram getFile: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("telegram getFile: error parseando respuesta: %w", err)
	}
	if !result.OK || result.Result.FilePath == "" {
		return nil, "", fmt.Errorf("telegram getFile: respuesta inválida para file_id=%s", fileID)
	}

	// 2. Descargar el archivo
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", s.botToken, result.Result.FilePath)

	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("telegram download: error creando request: %w", err)
	}

	dlResp, err := s.httpClient.Do(dlReq)
	if err != nil {
		return nil, "", fmt.Errorf("telegram download: error descargando archivo: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("telegram download: status %d", dlResp.StatusCode)
	}

	data, err := io.ReadAll(dlResp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("telegram download: error leyendo body: %w", err)
	}

	// Inferir mimeType del file_path
	mimeType := inferMimeType(result.Result.FilePath)

	slog.Info("telegram: archivo descargado", "file_id", fileID, "size", len(data), "mime", mimeType)
	return data, mimeType, nil
}

// splitMessage parte un texto largo en chunks que no excedan maxLen,
// cortando por salto de línea cuando es posible.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		// Buscar el último salto de línea dentro del límite
		cut := strings.LastIndex(text[:maxLen], "\n")
		if cut <= 0 {
			// No hay salto de línea — cortar en el límite duro
			cut = maxLen
		} else {
			cut++ // incluir el \n en el chunk actual
		}

		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}

	return chunks
}

// inferMimeType deduce el MIME type a partir de la extensión del file_path.
func inferMimeType(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	default:
		return "image/jpeg" // Telegram photos son casi siempre JPEG
	}
}
