package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	agentctx "github.com/alonsoalpizar/fabricalaser/internal/agent/context"
	"github.com/alonsoalpizar/fabricalaser/internal/agent/llm"
	"github.com/alonsoalpizar/fabricalaser/internal/agent/prompts"
	"github.com/alonsoalpizar/fabricalaser/internal/database"
)

// ChatRequest represents an incoming chat message
type ChatRequest struct {
	Message string         `json:"message"`
	History []HistoryEntry `json:"history,omitempty"`
}

// HistoryEntry represents a previous message in the conversation
type HistoryEntry struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ChatResponse represents the API response
type ChatResponse struct {
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Handler handles chat requests
type Handler struct {
	factory         *llm.Factory
	contextProvider *agentctx.Provider
	promptProvider  *prompts.Provider
}

// NewHandler creates a new chat handler con el factory de LLM compartido,
// un agentctx.Provider (mismo que usan los agentes de WhatsApp/Telegram) y
// un prompts.Provider que sirve los system prompts desde DB con fallback.
// El cliente LLM se obtiene on-demand via factory.Client() en cada request,
// lo que permite hot reload de proveedor sin reiniciar el servicio.
func NewHandler(factory *llm.Factory, ctxProv *agentctx.Provider, promptProv *prompts.Provider) *Handler {
	return &Handler{
		factory:         factory,
		contextProvider: ctxProv,
		promptProvider:  promptProv,
	}
}

// HandleChat processes a chat message via el Client LLM activo
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Solicitud inválida", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Message) == "" {
		sendError(w, "El mensaje no puede estar vacío", http.StatusBadRequest)
		return
	}

	// Get user identity from context (set by AuthMiddleware on /chat/* — auth opcional)
	userName, _ := r.Context().Value("userName").(string)
	userID, _ := r.Context().Value("userID").(uint)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	dynCtx := h.contextProvider.GetDynamicContext()
	response, err := h.callLLM(ctx, req.Message, req.History, userName, dynCtx)
	if err != nil {
		log.Printf("LLM error: %v", err)
		sendError(w, "Error procesando la solicitud", http.StatusInternalServerError)
		return
	}

	// Persistencia para clientes registrados.
	// Anónimos: no se guarda nada (decisión consciente: no consintieron explícitamente).
	// Esquema reutiliza tabla whatsapp_conversations con prefijo "web:<userID>" en
	// la columna phone — mismo patrón que Telegram usa "tg:<chat_id>".
	if userID > 0 {
		go persistWebTurns(userID, req.Message, response)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{Response: response})
}

// persistWebTurns guarda el par (user, model) en whatsapp_conversations.
// Fire-and-forget con logging estructurado de errores (no propaga al cliente).
func persistWebTurns(userID uint, userMsg, modelMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	phone := fmt.Sprintf("web:%d", userID)
	now := time.Now()
	db := database.Get()

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO whatsapp_conversations (phone, role, content, created_at) VALUES (?, ?, ?, ?)",
		phone, "user", userMsg, now,
	).Error; err != nil {
		slog.Error("chat web: failed to persist user turn",
			"user_id", userID, "error", err)
		return
	}

	// El segundo INSERT usa now+1ms para garantizar orden cronológico estable
	// si los timestamps tienen baja resolución.
	if err := db.WithContext(ctx).Exec(
		"INSERT INTO whatsapp_conversations (phone, role, content, created_at) VALUES (?, ?, ?, ?)",
		phone, "model", modelMsg, now.Add(time.Millisecond),
	).Error; err != nil {
		slog.Error("chat web: failed to persist model turn",
			"user_id", userID, "error", err)
	}
}

// callLLM construye el system prompt según auth state, arma el historial en formato
// llm.Message y envía la conversación al cliente LLM activo via h.factory.Client().
// No utiliza tools — el chat web aún no las soporta (ver roadmap paso 3).
func (h *Handler) callLLM(ctx context.Context, message string, history []HistoryEntry, userName string, dynCtx string) (string, error) {
	// Choose instruction based on auth state, then append live DB context.
	// Los system prompts vienen del prompts.Provider (DB con fallback hardcoded
	// y hot reload via pub/sub Redis) — ya no de const hardcoded en este archivo.
	var instruction string
	if userName == "" {
		instruction = h.promptProvider.Get("chat_web_public") + dynCtx
	} else {
		instruction = h.promptProvider.Get("chat_web_logged") +
			fmt.Sprintf("\n\n## Contexto del usuario actual:\n- Nombre: %s\n- Ya está registrado y autenticado en la plataforma", userName) +
			dynCtx
	}

	// Traducir history a []llm.Message: "user" -> RoleUser, "assistant"/"model" -> RoleModel
	llmHistory := make([]llm.Message, 0, len(history)+1)
	for _, entry := range history {
		role := llm.RoleUser
		if entry.Role == "assistant" || entry.Role == "model" {
			role = llm.RoleModel
		}
		llmHistory = append(llmHistory, llm.Message{
			Role:    role,
			Content: entry.Content,
		})
	}

	// El mensaje actual del usuario va como último turno
	llmHistory = append(llmHistory, llm.Message{
		Role:    llm.RoleUser,
		Content: message,
	})

	// Hot reload friendly: cada request obtiene el cliente actual desde el factory
	client := h.factory.Client()
	resp, err := client.Chat(ctx, instruction, llmHistory, nil)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return "", fmt.Errorf("empty response from model")
	}

	return resp.Content, nil
}

// SummaryRequest represents the conversation history to summarize
type SummaryRequest struct {
	History []HistoryEntry `json:"history"`
}

// SummaryResponse is the API response for the summary endpoint
type SummaryResponse struct {
	Summary string `json:"summary,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleSummary generates a concise WhatsApp-ready summary of the conversation
func (h *Handler) HandleSummary(w http.ResponseWriter, r *http.Request) {
	var req SummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.History) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SummaryResponse{Error: "Historial inválido"})
		return
	}

	// Build conversation text for summarization
	var conv strings.Builder
	for _, entry := range req.History {
		label := "Cliente"
		if entry.Role == "assistant" {
			label = "FabricaLaser"
		}
		conv.WriteString(label + ": " + entry.Content + "\n")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	systemSummary := "Sos un asistente especializado en resumir conversaciones de ventas para traspasar contexto a un asesor humano. Respondé solo con el resumen estructurado, sin saludos, sin explicaciones adicionales."

	prompt := `Analizá la conversación completa y generá un resumen estructurado para el asesor de FabricaLaser que va a atender al cliente por WhatsApp.

El resumen debe seguir este formato exacto:

El cliente [nombre si se mencionó, si no ""] consultó sobre:
- [producto/tema 1]: [detalles, cantidad, precio cotizado si aplica]
- [producto/tema 2]: [detalles, cantidad, precio cotizado si aplica]
(listá TODOS los productos o temas que el cliente preguntó durante la conversación, no solo el último)

Estado: [en qué quedó la conversación — qué decidió, qué duda quedó pendiente, qué necesita resolver el asesor]

Reglas:
- Incluí todos los temas consultados, aunque sean de mensajes anteriores
- Si se mencionaron precios o cantidades, incluílos siempre
- El estado debe indicar claramente qué necesita el asesor para continuar sin preguntarle al cliente desde cero
- Usá español de Costa Rica, tono directo
- El resumen puede ser tan largo como sea necesario para cubrir todo el contexto

Conversación:
` + conv.String()

	client := h.factory.Client()
	resp, err := client.Chat(ctx, systemSummary, []llm.Message{
		{Role: llm.RoleUser, Content: prompt},
	}, nil, llm.WithTemperature(0.2), llm.WithMaxTokens(700))
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		log.Printf("summary: error from model: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SummaryResponse{Error: "No se pudo generar el resumen"})
		return
	}

	summary := "Consulta desde el chat de FabricaLaser.com:\n\n" + strings.TrimSpace(resp.Content)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SummaryResponse{Summary: summary})
}

func sendError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ChatResponse{Error: message})
}
