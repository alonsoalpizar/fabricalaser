package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// ─── Interfaces — permiten tests sin dependencias reales ─────────────────────

// RedisClient define las operaciones Redis que necesita el procesador.
type RedisClient interface {
	// SetNX retorna true si la clave fue creada (no existía), false si ya existía.
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// PGClient define las operaciones PostgreSQL para archivo de conversaciones.
type PGClient interface {
	SaveTurn(ctx context.Context, turn ConversationTurn) error
}

// GeminiCaller define el contrato para llamar al agente de Vertex AI.
// Usa la misma firma que tu callGemini() actual — sin modificar nada.
type GeminiCaller interface {
	CallWithHistory(ctx context.Context, history []ChatTurn, newMessage string) (string, error)
}

// ─── Modelos de datos ────────────────────────────────────────────────────────

// ChatTurn representa un turno en el historial de conversación (formato Vertex AI).
type ChatTurn struct {
	Role    string `json:"role"`    // "user" o "model"
	Content string `json:"content"`
}

// ConversationTurn es lo que se persiste en PostgreSQL como archivo.
type ConversationTurn struct {
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── TTLs y límites ──────────────────────────────────────────────────────────

const (
	// TTL de sesión activa en Redis — 4 horas de inactividad resetea el historial
	sessionTTL = 4 * time.Hour

	// TTL de deduplicación — 60 segundos es suficiente para evitar doble procesamiento
	deduplicationTTL = 60 * time.Second

	// Máximo de turnos que se envían a Gemini — evita superar la ventana de contexto
	maxHistoryTurns = 20
)

// ─── MessageProcessor ────────────────────────────────────────────────────────

// MessageProcessor orquesta el flujo completo:
// deduplicación → cargar historial → llamar Gemini → responder → archivar
type MessageProcessor struct {
	redis  RedisClient
	pg     PGClient
	gemini GeminiCaller
	sender *Sender
}

// NewMessageProcessor construye el procesador con sus dependencias.
func NewMessageProcessor(redis RedisClient, pg PGClient, gemini GeminiCaller) *MessageProcessor {
	return &MessageProcessor{
		redis:  redis,
		pg:     pg,
		gemini: gemini,
		sender: NewSender(),
	}
}

// Process ejecuta el flujo completo para un payload de WhatsApp.
// Se llama desde una goroutine — no retorna error al caller, solo loguea.
func (p *MessageProcessor) Process(ctx context.Context, payload *WebhookPayload) {
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				// Solo procesamos mensajes de texto por ahora
				if msg.Type != "text" || msg.Text == nil {
					slog.Info("whatsapp: tipo de mensaje ignorado",
						"type", msg.Type,
						"from", msg.From,
					)
					continue
				}

				if err := p.processTextMessage(ctx, msg); err != nil {
					slog.Error("whatsapp: error procesando mensaje",
						"error", err,
						"message_id", msg.ID,
						"from", msg.From,
					)
				}
			}
		}
	}
}

// processTextMessage contiene la lógica completa para un mensaje de texto.
func (p *MessageProcessor) processTextMessage(ctx context.Context, msg Message) error {
	// 1. Deduplicación — Meta puede enviar el mismo webhook más de una vez
	dedupKey := fmt.Sprintf("wa:dedup:%s", msg.ID)
	isNew, err := p.redis.SetNX(ctx, dedupKey, "1", deduplicationTTL)
	if err != nil {
		return fmt.Errorf("processTextMessage: error verificando deduplicación: %w", err)
	}
	if !isNew {
		slog.Info("whatsapp: mensaje duplicado ignorado", "message_id", msg.ID)
		return nil
	}

	slog.Info("whatsapp: procesando mensaje",
		"from", msg.From,
		"message_id", msg.ID,
		"length", len(msg.Text.Body),
	)

	// 2. Cargar historial de la sesión activa desde Redis
	history, err := p.loadHistory(ctx, msg.From)
	if err != nil {
		// Error no fatal — continuamos sin historial antes que bloquear al usuario
		slog.Warn("whatsapp: no se pudo cargar historial, continuando sin él",
			"error", err,
			"phone", msg.From,
		)
		history = []ChatTurn{}
	}

	// 3. Llamar a Gemini con historial + mensaje nuevo
	response, err := p.gemini.CallWithHistory(ctx, history, msg.Text.Body)
	if err != nil {
		return fmt.Errorf("processTextMessage: error llamando a Gemini: %w", err)
	}

	// 4. Enviar respuesta al usuario vía Meta
	if err := p.sender.SendText(ctx, msg.From, response); err != nil {
		return fmt.Errorf("processTextMessage: error enviando respuesta: %w", err)
	}

	// 5. Actualizar historial en Redis (async — no bloqueamos el flujo principal)
	go p.saveHistoryAsync(context.Background(), msg.From, msg.Text.Body, response)

	return nil
}

// loadHistory recupera el historial de la sesión activa desde Redis.
// Retorna slice vacío si no hay sesión previa — comportamiento normal para usuarios nuevos.
func (p *MessageProcessor) loadHistory(ctx context.Context, phone string) ([]ChatTurn, error) {
	key := fmt.Sprintf("wa:hist:%s", phone)
	raw, err := p.redis.Get(ctx, key)
	if err != nil {
		// Clave no encontrada es un estado válido (sesión nueva o expirada)
		return []ChatTurn{}, nil
	}

	var history []ChatTurn
	if err := json.Unmarshal([]byte(raw), &history); err != nil {
		return nil, fmt.Errorf("loadHistory: error deserializando historial: %w", err)
	}

	// Limitar a los últimos N turnos para no superar la ventana de contexto de Gemini
	if len(history) > maxHistoryTurns {
		history = history[len(history)-maxHistoryTurns:]
	}

	return history, nil
}

// saveHistoryAsync persiste el nuevo turno en Redis y archiva en PostgreSQL.
// Se ejecuta en goroutine separada — los errores se loguean pero no afectan al usuario.
func (p *MessageProcessor) saveHistoryAsync(ctx context.Context, phone, userMsg, botResponse string) {
	// Actualizar historial en Redis
	if err := p.updateRedisHistory(ctx, phone, userMsg, botResponse); err != nil {
		slog.Error("whatsapp: error actualizando historial en Redis",
			"error", err,
			"phone", phone,
		)
	}

	// Archivar ambos turnos en PostgreSQL
	now := time.Now()
	turns := []ConversationTurn{
		{Phone: phone, Role: "user", Content: userMsg, CreatedAt: now},
		{Phone: phone, Role: "model", Content: botResponse, CreatedAt: now},
	}

	for _, turn := range turns {
		if err := p.pg.SaveTurn(ctx, turn); err != nil {
			slog.Error("whatsapp: error archivando turno en PostgreSQL",
				"error", err,
				"phone", phone,
				"role", turn.Role,
			)
		}
	}
}

// updateRedisHistory agrega los nuevos turnos al historial existente en Redis.
func (p *MessageProcessor) updateRedisHistory(ctx context.Context, phone, userMsg, botResponse string) error {
	key := fmt.Sprintf("wa:hist:%s", phone)

	// Cargar historial actual
	history, err := p.loadHistory(ctx, phone)
	if err != nil {
		history = []ChatTurn{}
	}

	// Agregar nuevos turnos
	history = append(history,
		ChatTurn{Role: "user", Content: userMsg},
		ChatTurn{Role: "model", Content: botResponse},
	)

	// Serializar y guardar con TTL renovado
	raw, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("updateRedisHistory: error serializando: %w", err)
	}

	if err := p.redis.Set(ctx, key, string(raw), sessionTTL); err != nil {
		return fmt.Errorf("updateRedisHistory: error guardando en Redis: %w", err)
	}

	return nil
}