package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	adminGCPProjectID  = "div-aloalpizar"
	adminGCPLocation   = "us-central1"
	defaultAdminModel  = "gemini-2.5-flash"
	adminToolLoopMax   = 8
	adminTemperature   = 0.4
	adminTopP          = 0.95
	adminMaxOutputTok  = 2048
)

// ToolCallTrace registra una invocación de tool para auditoría.
// Se serializa a JSONB en admin_chat_messages.tool_calls.
type ToolCallTrace struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Result map[string]any `json:"result"`
}

// CallResult es lo que devuelve el adapter al handler: texto + traza de tools.
type CallResult struct {
	Reply     string          `json:"reply"`
	ToolCalls []ToolCallTrace `json:"tool_calls,omitempty"`
}

// geminiAdapter encapsula el cliente Vertex AI y el ContextProvider.
type geminiAdapter struct {
	client          *genai.Client
	contextProvider *ContextProvider
	executor        *toolExecutor
	modelName       string
}

// newGeminiAdapter construye el adapter. R7: el modelo se lee de
// ADMIN_GEMINI_MODEL (default: gemini-2.5-flash, ya validado en producción
// por el bot de WhatsApp).
func newGeminiAdapter(provider *ContextProvider, executor *toolExecutor) *geminiAdapter {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, adminGCPProjectID, adminGCPLocation)
	if err != nil {
		log.Fatalf("admin_chat: failed to create Vertex AI client: %v", err)
	}

	model := os.Getenv("ADMIN_GEMINI_MODEL")
	if model == "" {
		model = defaultAdminModel
	}

	return &geminiAdapter{
		client:          client,
		contextProvider: provider,
		executor:        executor,
		modelName:       model,
	}
}

// Call es la llamada principal. Recibe historial de turnos previos + mensaje
// nuevo + datos del gestor, ejecuta el tool loop hasta 8 iteraciones y
// devuelve respuesta + traza de tools usadas.
func (g *geminiAdapter) Call(
	ctx context.Context,
	adminID uint,
	adminName string,
	history []ChatTurn,
	newMessage string,
) (*CallResult, error) {
	model := g.client.GenerativeModel(g.modelName)

	dynCtx := g.contextProvider.Get()
	adminCtx := buildAdminContextBlock(adminID, adminName)

	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPromptAdmin + adminCtx + dynCtx)},
	}
	model.Tools = []*genai.Tool{{FunctionDeclarations: toolDeclarations()}}

	temp := float32(adminTemperature)
	topP := float32(adminTopP)
	maxOut := int32(adminMaxOutputTok)
	model.Temperature = &temp
	model.TopP = &topP
	model.MaxOutputTokens = &maxOut

	chat := model.StartChat()
	for _, turn := range history {
		chat.History = append(chat.History, &genai.Content{
			Role:  turn.Role,
			Parts: []genai.Part{genai.Text(turn.Content)},
		})
	}

	resp, err := sendWithRetryAdmin(ctx, chat, genai.Text(newMessage))
	if err != nil {
		return nil, fmt.Errorf("admin_chat Call: %w", err)
	}

	var traces []ToolCallTrace

	for i := 0; i < adminToolLoopMax; i++ {
		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			break
		}

		var fc *genai.FunctionCall
		for _, part := range resp.Candidates[0].Content.Parts {
			if call, ok := part.(genai.FunctionCall); ok {
				fc = &call
				break
			}
		}

		if fc == nil {
			// Sin tool call → respuesta final, extraer texto
			text := extractText(resp)
			if text != "" {
				return &CallResult{Reply: text, ToolCalls: traces}, nil
			}
			break
		}

		result, err := g.executor.executeFunction(ctx, adminID, fc)
		if err != nil {
			slog.Error("admin_chat: error ejecutando tool",
				"tool", fc.Name, "admin_id", adminID, "error", err)
			result = map[string]any{"error": err.Error()}
		}

		traces = append(traces, ToolCallTrace{
			Name:   fc.Name,
			Args:   fc.Args,
			Result: result,
		})

		resp, err = sendWithRetryAdmin(ctx, chat, genai.FunctionResponse{
			Name:     fc.Name,
			Response: result,
		})
		if err != nil {
			return nil, fmt.Errorf("admin_chat FunctionResponse: %w", err)
		}
	}

	// Fallback: tool loop terminado, intentar extraer texto
	text := extractText(resp)
	if text == "" {
		text = "No pude generar una respuesta. Intentá reformular la consulta."
	}
	return &CallResult{Reply: text, ToolCalls: traces}, nil
}

func extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return ""
	}
	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			sb.WriteString(string(t))
		}
	}
	return sb.String()
}

// SerializeToolCalls convierte traces a JSON string (nil si vacío) para guardar
// en admin_chat_messages.tool_calls.
func SerializeToolCalls(traces []ToolCallTrace) *string {
	if len(traces) == 0 {
		return nil
	}
	b, err := json.Marshal(traces)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

// sendWithRetryAdmin reintenta hasta 3 veces ante 429 ResourceExhausted con
// backoff 2s/4s/8s. Patrón copiado del WA adapter (R: duplicado consciente
// para no acoplar paquetes).
func sendWithRetryAdmin(ctx context.Context, chat *genai.ChatSession, part genai.Part) (*genai.GenerateContentResponse, error) {
	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		resp, err := chat.SendMessage(ctx, part)
		if err == nil {
			return resp, nil
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.ResourceExhausted {
			return nil, err
		}

		if attempt == len(delays) {
			lastErr = err
			break
		}

		slog.Warn("admin_chat: Vertex AI 429 — reintentando",
			"attempt", attempt+1, "wait", delays[attempt])

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delays[attempt]):
		}
		lastErr = err
	}

	return nil, fmt.Errorf("sendWithRetryAdmin: agotados reintentos tras 429: %w", lastErr)
}
