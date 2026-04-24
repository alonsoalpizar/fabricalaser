package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/alonsoalpizar/fabricalaser/internal/agent/llm"
	"github.com/alonsoalpizar/fabricalaser/internal/agent/prompts"
)

const (
	adminToolLoopMax  = 8
	adminTemperature  = 0.4
	adminTopP         = 0.95
	adminMaxOutputTok = 2048
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

// geminiAdapter encapsula el factory LLM, el ContextProvider y el Provider de
// system prompts.
//
// Nombre preservado por compatibilidad con handler.go, pero ya no está atado a
// Gemini/Vertex — consume la interface llm.Client, por lo que opera sobre
// cualquier proveedor activo (Vertex, OpenAI, DeepSeek, Kimi, Anthropic).
// El factory se consulta en cada iteración del tool loop para que el hot-reload
// del admin UI tome efecto aun durante una conversación en vuelo (R6).
//
// El promptProvider sirve el system prompt base del agente admin_chat desde DB
// (con fallback hardcoded e hot-reload vía pub/sub Redis).
type geminiAdapter struct {
	factory         *llm.Factory
	contextProvider *ContextProvider
	executor        *toolExecutor
	promptProvider  *prompts.Provider
}

// newGeminiAdapter construye el adapter. Ya no recibe ni model name ni
// credenciales — todo viene del factory. El promptProvider se inyecta para
// poder servir el system prompt base desde DB con hot reload.
func newGeminiAdapter(
	factory *llm.Factory,
	provider *ContextProvider,
	executor *toolExecutor,
	promptProv *prompts.Provider,
) *geminiAdapter {
	return &geminiAdapter{
		factory:         factory,
		contextProvider: provider,
		executor:        executor,
		promptProvider:  promptProv,
	}
}

// Call es la llamada principal. Recibe historial de turnos previos + mensaje
// nuevo + datos del gestor, ejecuta el tool loop hasta adminToolLoopMax
// iteraciones y devuelve respuesta + traza de tools usadas.
//
// ChatTurn.Role ("user"|"model") se mapea directo a llm.RoleUser/llm.RoleModel.
// El repositorio no persiste turnos de tool (role="tool"): esos viven solo
// dentro del tool loop y no se replay-ean en llamadas futuras.
func (g *geminiAdapter) Call(
	ctx context.Context,
	adminID uint,
	adminName string,
	history []ChatTurn,
	newMessage string,
) (*CallResult, error) {
	dynCtx := g.contextProvider.Get()
	adminCtx := buildAdminContextBlock(adminID, adminName)
	systemPrompt := g.promptProvider.Get("admin_chat") + adminCtx + dynCtx

	// Traducir historial del repo a []llm.Message
	messages := make([]llm.Message, 0, len(history)+1)
	for _, turn := range history {
		role := llm.RoleUser
		if turn.Role == "model" {
			role = llm.RoleModel
		}
		messages = append(messages, llm.Message{
			Role:    role,
			Content: turn.Content,
		})
	}
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: newMessage,
	})

	tools := toolDeclarations()
	opts := []llm.ChatOption{
		llm.WithTemperature(adminTemperature),
		llm.WithTopP(adminTopP),
		llm.WithMaxTokens(adminMaxOutputTok),
	}

	var traces []ToolCallTrace

	for iter := 0; iter < adminToolLoopMax; iter++ {
		client := g.factory.Client() // re-lookup por iteración → hot reload friendly (R6)
		resp, err := client.Chat(ctx, systemPrompt, messages, tools, opts...)
		if err != nil {
			return nil, fmt.Errorf("admin_chat Call: %w", err)
		}

		// Respuesta final: sin tool calls
		if len(resp.ToolCalls) == 0 {
			reply := resp.Content
			if reply == "" {
				reply = "No pude generar una respuesta. Intentá reformular la consulta."
			}
			return &CallResult{Reply: reply, ToolCalls: traces}, nil
		}

		// Agregar la respuesta del modelo (con tool_calls) al historial de la conversación
		messages = append(messages, llm.Message{
			Role:      llm.RoleModel,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Ejecutar cada tool solicitada y agregar su resultado como mensaje role=tool
		for _, tc := range resp.ToolCalls {
			result, execErr := g.executor.executeToolCall(ctx, adminID, tc)
			if execErr != nil {
				slog.Error("admin_chat: error ejecutando tool",
					"tool", tc.Name, "admin_id", adminID, "error", execErr)
				result = map[string]any{"error": execErr.Error()}
			}

			traces = append(traces, ToolCallTrace{
				Name:   tc.Name,
				Args:   tc.Args,
				Result: result,
			})

			// Serializar resultado a JSON para Message.Content (contrato llm.RoleTool)
			resultJSON, mErr := json.Marshal(result)
			if mErr != nil {
				resultJSON = []byte(fmt.Sprintf(`{"error":%q}`, mErr.Error()))
			}

			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    string(resultJSON),
				ToolCallID: tc.ID, // crítico para OpenAI/Anthropic (R11); Vertex lo ignora
			})
		}
	}

	// Tool loop agotado sin respuesta final
	slog.Warn("admin_chat: tool loop alcanzó máximo de iteraciones",
		"admin_id", adminID, "max", adminToolLoopMax, "traces", len(traces))
	return &CallResult{
		Reply:     "No pude concretar la respuesta tras varios intentos. Reformulá la consulta.",
		ToolCalls: traces,
	}, nil
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
