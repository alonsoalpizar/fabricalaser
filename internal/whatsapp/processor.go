package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ─── Interfaces — permiten tests sin dependencias reales ─────────────────────

// RedisClient define las operaciones Redis que necesita el procesador.
type RedisClient interface {
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// PGClient define las operaciones PostgreSQL para archivo de conversaciones.
type PGClient interface {
	SaveTurn(ctx context.Context, turn ConversationTurn) error
}

// GeminiCaller define el contrato para llamar al agente de Vertex AI.
type GeminiCaller interface {
	CallWithHistory(ctx context.Context, history []ChatTurn, newMessage string) (string, error)
	CallWithTools(ctx context.Context, phone string, history []ChatTurn, newMessage string) (string, error)
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
	sessionTTL       = 4 * time.Hour
	deduplicationTTL = 60 * time.Second
	maxHistoryTurns  = 20
)

// ─── MessageProcessor ────────────────────────────────────────────────────────

// MessageProcessor orquesta el flujo completo:
// deduplicación → rate limit → cargar historial → llamar Gemini → responder → archivar
type MessageProcessor struct {
	redis           RedisClient
	pg              PGClient
	gemini          GeminiCaller
	sender          *Sender
	rateLimiter     *RateLimiter
	contextProvider *waContextProvider
}

// NewMessageProcessor construye el procesador con sus dependencias.
// rateLimiter puede ser nil — en ese caso el rate limiting queda deshabilitado (fail open).
func NewMessageProcessor(redis RedisClient, pg PGClient, gemini GeminiCaller, rateLimiter *RateLimiter, contextProvider *waContextProvider) *MessageProcessor {
	return &MessageProcessor{
		redis:           redis,
		pg:              pg,
		gemini:          gemini,
		sender:          NewSender(),
		rateLimiter:     rateLimiter,
		contextProvider: contextProvider,
	}
}

// Process ejecuta el flujo completo para un payload de WhatsApp.
func (p *MessageProcessor) Process(ctx context.Context, payload *WebhookPayload) {
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
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

	// 2. Rate limiting — solo para nuevas conversaciones
	if p.rateLimiter != nil {
		result := p.rateLimiter.Check(ctx, msg.From)
		if result == Deny {
			slog.Warn("whatsapp: mensaje descartado por rate limiter",
				"from", msg.From,
				"message_id", msg.ID,
			)
			return nil
		}
		if result == AllowContinuation {
			slog.Debug("whatsapp: conversación continuada — sin contar", "from", msg.From)
		}
	}

	slog.Info("whatsapp: procesando mensaje",
		"from", msg.From,
		"message_id", msg.ID,
		"length", len(msg.Text.Body),
	)

	// 3. Límite diario de mensajes
	count, limitErr := p.checkDailyLimit(ctx, msg.From)
	if limitErr != nil {
		slog.Warn("whatsapp: error verificando límite diario, continuando normalmente", "error", limitErr)
	} else {
		maxMsgs := p.contextProvider.GetMaxMensajesDia()
		if count > int64(maxMsgs) {
			slog.Info("whatsapp: límite diario alcanzado", "from", msg.From, "count", count, "max", maxMsgs)
			mensajeLimite := "Hemos alcanzado el límite de mensajes automáticos por hoy. " +
				"Un asesor de FabricaLaser te va a contactar para ayudarte. ¡Gracias por tu paciencia!"
			_ = p.sender.SendText(ctx, msg.From, mensajeLimite)

			asesorPhone := p.contextProvider.GetAsesorPhone()
			resumen := fmt.Sprintf(
				"FabricaLaser — Límite alcanzado\n\n"+
					"⚠️ Cliente alcanzó el límite de %d mensajes hoy.\n"+
					"Requiere atención humana para completar su consulta.\n"+
					"Número del cliente: %s", maxMsgs, msg.From)
			_ = p.sender.SendText(ctx, asesorPhone, resumen)
			return nil
		}
	}

	// 4. Cargar historial de la sesión activa desde Redis
	history, err := p.loadHistory(ctx, msg.From)
	if err != nil {
		slog.Warn("whatsapp: no se pudo cargar historial, continuando sin él",
			"error", err,
			"phone", msg.From,
		)
		history = []ChatTurn{}
	}

	// 5. Llamar a Gemini con historial + mensaje nuevo (con tools)
	response, err := p.gemini.CallWithTools(ctx, msg.From, history, msg.Text.Body)
	if err != nil {
		return fmt.Errorf("processTextMessage: error llamando a Gemini: %w", err)
	}

	// Aviso preventivo cuando quedan 2 mensajes automáticos
	if limitErr == nil {
		maxMsgs := p.contextProvider.GetMaxMensajesDia()
		warningAt := int64(maxMsgs) - 2
		if count == warningAt {
			response += fmt.Sprintf("\n\n(Nota: te quedan 2 consultas automáticas por hoy. "+
				"Si necesitás más ayuda, un asesor puede atenderte.)")
		}
	}

	// 4. Enviar respuesta al usuario vía Meta
	if err := p.sender.SendText(ctx, msg.From, response); err != nil {
		if errors.Is(err, ErrMetaRateLimit) {
			// Límite real de Meta alcanzado — loguear pero no propagar como error fatal
			slog.Error("whatsapp: límite real de Meta — mensaje no enviado, revisar verificación de negocio",
				"from", msg.From,
			)
			return nil
		}
		return fmt.Errorf("processTextMessage: error enviando respuesta: %w", err)
	}

	// 5. Actualizar historial en Redis (async)
	go p.saveHistoryAsync(context.Background(), msg.From, msg.Text.Body, response)

	return nil
}

func (p *MessageProcessor) loadHistory(ctx context.Context, phone string) ([]ChatTurn, error) {
	key := fmt.Sprintf("wa:hist:%s", phone)
	raw, err := p.redis.Get(ctx, key)
	if err != nil {
		return []ChatTurn{}, nil
	}

	var history []ChatTurn
	if err := json.Unmarshal([]byte(raw), &history); err != nil {
		return nil, fmt.Errorf("loadHistory: error deserializando historial: %w", err)
	}

	if len(history) > maxHistoryTurns {
		history = history[len(history)-maxHistoryTurns:]
	}

	return history, nil
}

func (p *MessageProcessor) saveHistoryAsync(ctx context.Context, phone, userMsg, botResponse string) {
	if err := p.updateRedisHistory(ctx, phone, userMsg, botResponse); err != nil {
		slog.Error("whatsapp: error actualizando historial en Redis",
			"error", err,
			"phone", phone,
		)
	}

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

// checkDailyLimit incrementa el contador diario del teléfono y retorna el valor actual.
// La clave expira a medianoche hora Costa Rica. Devuelve (count, error).
func (p *MessageProcessor) checkDailyLimit(ctx context.Context, phone string) (int64, error) {
	loc, _ := time.LoadLocation("America/Costa_Rica")
	now := time.Now().In(loc)
	fecha := now.Format("2006-01-02")
	key := fmt.Sprintf("wa:limit:%s:%s", phone, fecha)

	count, err := p.redis.Incr(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("checkDailyLimit: error incrementando contador: %w", err)
	}

	// Setear TTL solo en el primer mensaje del día
	if count == 1 {
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
		_ = p.redis.Expire(ctx, key, midnight.Sub(now))
	}

	return count, nil
}

func (p *MessageProcessor) updateRedisHistory(ctx context.Context, phone, userMsg, botResponse string) error {
	key := fmt.Sprintf("wa:hist:%s", phone)

	history, err := p.loadHistory(ctx, phone)
	if err != nil {
		history = []ChatTurn{}
	}

	history = append(history,
		ChatTurn{Role: "user", Content: userMsg},
		ChatTurn{Role: "model", Content: botResponse},
	)

	raw, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("updateRedisHistory: error serializando: %w", err)
	}

	if err := p.redis.Set(ctx, key, string(raw), sessionTTL); err != nil {
		return fmt.Errorf("updateRedisHistory: error guardando en Redis: %w", err)
	}

	return nil
}
