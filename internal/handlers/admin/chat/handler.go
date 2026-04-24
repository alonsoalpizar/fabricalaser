package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

const (
	redisHistKey    = "admin:chat:hist:%d"    // %d = adminID
	redisSessionKey = "admin:chat:session:%d" // %d = adminID
	sessionTTL      = 4 * time.Hour
	requestTimeout  = 60 * time.Second
)

// Handler expone los 5 endpoints del chat administrativo bajo /api/v1/admin/chat/*.
type Handler struct {
	redis    *redis.Client
	repo     *ConversationRepository
	executor *toolExecutor
	gemini   *geminiAdapter
}

// NewHandler construye el handler con todas las dependencias.
// El context provider se inyecta porque también lo usa el ContextProvider
// global del servidor (cache compartido).
func NewHandler(redisClient *redis.Client, ctxProvider *ContextProvider) *Handler {
	executor := newToolExecutor()
	return &Handler{
		redis:    redisClient,
		repo:     NewConversationRepository(),
		executor: executor,
		gemini:   newGeminiAdapter(ctxProvider, executor),
	}
}

// ─── Endpoints ───────────────────────────────────────────────────────────────

type sendMessageRequest struct {
	Content string `json:"content"`
}

type sendMessageResponse struct {
	Reply     string          `json:"reply"`
	SessionID string          `json:"session_id"`
	ToolCalls []ToolCallTrace `json:"tool_calls,omitempty"`
}

// SendMessage atiende POST /api/v1/admin/chat/message.
//
// Flujo (R2, R3, R4, R6 aplicados):
//  1. Extrae adminID/adminName del JWT (puesto por AuthMiddleware).
//  2. loadHistory con fallback Redis→DB (R3).
//  3. Si no hay sesión activa: cierra huérfanas previas (R6) y crea nueva.
//  4. Append turno user a Redis + persist async con logging (R2, R4).
//  5. Llama a Gemini (tool loop hasta 8).
//  6. Append turno model a Redis + persist async con tool_calls (R2, R4).
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	adminID, adminName, ok := extractAdmin(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if req.Content == "" {
		writeJSONError(w, http.StatusBadRequest, "content vacío")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	history, sessionID, err := h.loadHistory(ctx, adminID)
	if err != nil {
		slog.Error("admin_chat: error cargando historial", "admin_id", adminID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "error cargando historial")
		return
	}

	// Si no hay sesión activa: cerrar huérfanas (R6) y crear una nueva
	if sessionID == "" {
		if err := h.repo.EndAllOpenForAdmin(adminID, ""); err != nil {
			slog.Error("admin_chat: error cerrando huérfanas", "admin_id", adminID, "error", err)
		}
		newID, err := h.repo.CreateSession(adminID)
		if err != nil {
			slog.Error("admin_chat: error creando sesión", "admin_id", adminID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, "error creando sesión")
			return
		}
		sessionID = newID
		_ = h.redis.Set(ctx, fmt.Sprintf(redisSessionKey, adminID), sessionID, sessionTTL).Err()
	}

	// 1. Persistir turno user (async + logging)
	go h.persistMessage(sessionID, adminID, "user", req.Content, nil)

	// 2. Llamar a Gemini
	result, err := h.gemini.Call(ctx, adminID, adminName, history, req.Content)
	if err != nil {
		slog.Error("admin_chat: error llamando Gemini",
			"admin_id", adminID, "session_id", sessionID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "error generando respuesta")
		return
	}

	// 3. Persistir turno model (async + logging) con tool_calls JSONB
	toolCallsJSON := SerializeToolCalls(result.ToolCalls)
	go h.persistMessage(sessionID, adminID, "model", result.Reply, toolCallsJSON)

	// 4. Actualizar historial Redis (síncrono — la próxima llamada lo necesita)
	updatedHistory := append(history,
		ChatTurn{Role: "user", Content: req.Content},
		ChatTurn{Role: "model", Content: result.Reply},
	)
	h.saveHistoryRedis(ctx, adminID, updatedHistory)

	writeJSON(w, http.StatusOK, sendMessageResponse{
		Reply:     result.Reply,
		SessionID: sessionID,
		ToolCalls: result.ToolCalls,
	})
}

type resetResponse struct {
	SessionID        string `json:"session_id"`
	PreviousArchived bool   `json:"previous_archived"`
}

// Reset cierra la sesión activa del gestor y limpia Redis.
// NO crea una nueva sesión inmediatamente — se creará en la próxima
// SendMessage según el flujo natural (R6).
func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	adminID, _, ok := extractAdmin(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	ctx := r.Context()
	hadOpen := false

	// Cerrar TODAS las abiertas (R6, exceptID == "")
	if err := h.repo.EndAllOpenForAdmin(adminID, ""); err != nil {
		slog.Error("admin_chat Reset: error cerrando sesiones", "admin_id", adminID, "error", err)
	} else {
		hadOpen = true
	}

	// Limpiar Redis
	_ = h.redis.Del(ctx,
		fmt.Sprintf(redisHistKey, adminID),
		fmt.Sprintf(redisSessionKey, adminID),
	).Err()

	writeJSON(w, http.StatusOK, resetResponse{SessionID: "", PreviousArchived: hadOpen})
}

type historyResponse struct {
	SessionID string     `json:"session_id"`
	Messages  []ChatTurn `json:"messages"`
}

// GetHistory retorna el historial de la sesión activa. Útil para que la UI
// reconstruya el chat al recargar la página.
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	adminID, _, ok := extractAdmin(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "no autenticado")
		return
	}

	history, sessionID, err := h.loadHistory(r.Context(), adminID)
	if err != nil {
		slog.Error("admin_chat GetHistory: error", "admin_id", adminID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "error cargando historial")
		return
	}

	writeJSON(w, http.StatusOK, historyResponse{
		SessionID: sessionID,
		Messages:  history,
	})
}

type listSessionsResponse struct {
	Sessions []SessionRow `json:"sessions"`
	Total    int64        `json:"total"`
}

// ListSessions retorna sesiones paginadas (auditoría, todos los admins).
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	sessions, total, err := h.repo.ListSessions(limit, offset)
	if err != nil {
		slog.Error("admin_chat ListSessions: error", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "error listando sesiones")
		return
	}

	writeJSON(w, http.StatusOK, listSessionsResponse{
		Sessions: sessions,
		Total:    total,
	})
}

type sessionDetailResponse struct {
	Session  *SessionRow  `json:"session,omitempty"`
	Messages []MessageRow `json:"messages"`
}

// GetSessionMessages retorna todos los mensajes de una sesión específica.
func (h *Handler) GetSessionMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "session id requerido")
		return
	}

	msgs, err := h.repo.GetSessionMessages(sessionID)
	if err != nil {
		slog.Error("admin_chat GetSessionMessages: error", "session_id", sessionID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, "error cargando sesión")
		return
	}

	writeJSON(w, http.StatusOK, sessionDetailResponse{Messages: msgs})
}

// ─── Carga de historial con fallback Redis→DB (R3) ──────────────────────────

func (h *Handler) loadHistory(ctx context.Context, adminID uint) ([]ChatTurn, string, error) {
	histKey := fmt.Sprintf(redisHistKey, adminID)
	sessionKey := fmt.Sprintf(redisSessionKey, adminID)

	// 1. Intentar desde Redis
	histRaw, err := h.redis.Get(ctx, histKey).Result()
	if err == nil && histRaw != "" {
		var hist []ChatTurn
		if err := json.Unmarshal([]byte(histRaw), &hist); err == nil {
			sessionID, _ := h.redis.Get(ctx, sessionKey).Result()
			return hist, sessionID, nil
		}
	}

	// 2. Redis miss → fallback a DB
	turns, sessionID, err := h.repo.LoadActiveSession(adminID)
	if err != nil {
		return nil, "", err
	}

	if sessionID == "" {
		// No hay sesión activa — devolver vacío, no es error
		return []ChatTurn{}, "", nil
	}

	// 3. Reconstruir Redis con lo recuperado de DB
	h.saveHistoryRedis(ctx, adminID, turns)
	_ = h.redis.Set(ctx, sessionKey, sessionID, sessionTTL).Err()

	slog.Info("admin_chat: historial reconstruido desde DB",
		"admin_id", adminID, "session_id", sessionID, "turnos", len(turns))

	return turns, sessionID, nil
}

func (h *Handler) saveHistoryRedis(ctx context.Context, adminID uint, history []ChatTurn) {
	if len(history) == 0 {
		return
	}
	raw, err := json.Marshal(history)
	if err != nil {
		slog.Error("admin_chat: error serializando historial", "admin_id", adminID, "error", err)
		return
	}
	if err := h.redis.Set(ctx, fmt.Sprintf(redisHistKey, adminID), string(raw), sessionTTL).Err(); err != nil {
		slog.Error("admin_chat: error guardando historial en Redis", "admin_id", adminID, "error", err)
	}
}

// persistMessage guarda en DB de forma asíncrona, loggeando errores (R2).
// Usa context.Background() porque el request HTTP puede haber terminado
// para cuando esto corra.
func (h *Handler) persistMessage(sessionID string, adminID uint, role, content string, toolCalls *string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.repo.SaveMessage(ctx, sessionID, adminID, role, content, toolCalls); err != nil {
		slog.Error("admin_chat: failed to persist message",
			"session_id", sessionID, "admin_id", adminID, "role", role, "error", err)
	}
}

// ─── Helpers HTTP ────────────────────────────────────────────────────────────

func extractAdmin(r *http.Request) (uint, string, bool) {
	idVal := r.Context().Value("userID")
	if idVal == nil {
		return 0, "", false
	}
	adminID, ok := idVal.(uint)
	if !ok || adminID == 0 {
		return 0, "", false
	}
	name, _ := r.Context().Value("userName").(string)
	return adminID, name, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"message": msg},
	})
}
