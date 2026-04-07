package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/whatsapp"
)

const (
	sessionTTL       = 4 * time.Hour
	deduplicationTTL = 60 * time.Second
	maxHistoryTurns  = 50
)

// Processor orquesta el flujo completo de mensajes de Telegram:
// deduplicación → rate limit → límite diario → historial → Gemini → responder → archivar.
type Processor struct {
	redis           whatsapp.RedisClient
	pg              whatsapp.PGClient
	gemini          whatsapp.GeminiCaller
	sender          *Sender
	waSender        *whatsapp.Sender // para notificar al asesor vía WhatsApp
	rateLimiter     *whatsapp.RateLimiter
	contextProvider *whatsapp.WAContextProvider
}

// NewProcessor construye el procesador reutilizando las interfaces del paquete whatsapp.
func NewProcessor(redis whatsapp.RedisClient, pg whatsapp.PGClient, gemini whatsapp.GeminiCaller, rateLimiter *whatsapp.RateLimiter, contextProvider *whatsapp.WAContextProvider) *Processor {
	return &Processor{
		redis:           redis,
		pg:              pg,
		gemini:          gemini,
		sender:          NewSender(),
		waSender:        whatsapp.NewSender(),
		rateLimiter:     rateLimiter,
		contextProvider: contextProvider,
	}
}

// Process ejecuta el flujo completo para un Update de Telegram.
func (p *Processor) Process(ctx context.Context, update *Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	chatID := msg.Chat.ID
	phoneKey := fmt.Sprintf("tg:%d", chatID)

	switch {
	case len(msg.Photo) > 0:
		if err := p.processImage(ctx, msg, chatID, phoneKey); err != nil {
			slog.Error("telegram: error procesando imagen",
				"error", err,
				"chat_id", chatID,
				"message_id", msg.MessageID,
			)
		}
	case msg.Text != "":
		if err := p.processText(ctx, msg, chatID, phoneKey); err != nil {
			slog.Error("telegram: error procesando mensaje de texto",
				"error", err,
				"chat_id", chatID,
				"message_id", msg.MessageID,
			)
		}
	default:
		slog.Info("telegram: tipo de mensaje ignorado", "chat_id", chatID)
	}
}

func (p *Processor) processText(ctx context.Context, msg *TGMessage, chatID int64, phoneKey string) error {
	// 1. Deduplicación
	dedupKey := fmt.Sprintf("tg:dedup:%d", msg.MessageID)
	isNew, err := p.redis.SetNX(ctx, dedupKey, "1", deduplicationTTL)
	if err != nil {
		return fmt.Errorf("processText: error verificando deduplicación: %w", err)
	}
	if !isNew {
		slog.Info("telegram: mensaje duplicado ignorado", "message_id", msg.MessageID)
		return nil
	}

	// 2. Rate limiting
	if p.rateLimiter != nil {
		result := p.rateLimiter.Check(ctx, phoneKey)
		if result == whatsapp.Deny {
			slog.Warn("telegram: mensaje descartado por rate limiter", "chat_id", chatID)
			return nil
		}
	}

	slog.Info("telegram: procesando mensaje",
		"chat_id", chatID,
		"message_id", msg.MessageID,
		"length", len(msg.Text),
	)

	// 2b. Opt-in — disclaimer de privacidad en primer contacto (1 vez cada 30 días)
	optinKey := fmt.Sprintf("tg:optin:%d", chatID)
	isFirstContact, _ := p.redis.SetNX(ctx, optinKey, "1", 30*24*time.Hour)
	if isFirstContact {
		_ = p.sender.SendText(ctx, chatID,
			"Al comunicarte con FabricaLaser por este canal, aceptás que "+
				"procesemos tu número, nombre y mensajes para brindarte cotizaciones "+
				"y atención al cliente. Más info: fabricalaser.com/privacidad")
	}

	// 3. Límite diario de mensajes (el asesor está exento)
	count, limitErr := p.checkDailyLimit(ctx, phoneKey)
	isAsesor := chatID == p.contextProvider.GetAsesorTelegramChatID()
	if limitErr != nil {
		slog.Warn("telegram: error verificando límite diario, continuando", "error", limitErr)
	} else if !isAsesor {
		maxMsgs := p.contextProvider.GetMaxMensajesDia()
		if count > int64(maxMsgs) {
			slog.Info("telegram: límite diario alcanzado", "chat_id", chatID, "count", count, "max", maxMsgs)
			_ = p.sender.SendText(ctx, chatID,
				"Hemos alcanzado el límite de mensajes automáticos por hoy. "+
					"Un asesor de FabricaLaser te va a contactar para ayudarte. ¡Gracias por tu paciencia!")

			// Notificar al asesor por Telegram con resumen
			p.notifyAsesorLimit(ctx, chatID, phoneKey, msg, maxMsgs)
			return nil
		}
	}

	// 4. Cargar historial de la sesión activa desde Redis
	history, err := p.loadHistory(ctx, phoneKey)
	if err != nil {
		slog.Warn("telegram: no se pudo cargar historial, continuando sin él", "error", err)
		history = []whatsapp.ChatTurn{}
	}

	// 5. Construir contexto del usuario de Telegram
	userCtx := p.buildTelegramUserCtx(msg)

	// 6. Llamar a Gemini con historial + mensaje nuevo
	response, err := p.gemini.CallWithTools(ctx, phoneKey, history, msg.Text, userCtx)
	if err != nil {
		_ = p.sender.SendText(ctx, chatID,
			"En este momento tengo un problema técnico. Por favor intentá de nuevo en unos segundos.")
		return fmt.Errorf("processText: error llamando a Gemini: %w", err)
	}

	// Aviso preventivo cuando quedan 2 mensajes automáticos (exento si es el asesor)
	if limitErr == nil && !isAsesor {
		maxMsgs := p.contextProvider.GetMaxMensajesDia()
		warningAt := int64(maxMsgs) - 2
		if count == warningAt {
			response += fmt.Sprintf("\n\n(Nota: te quedan 2 consultas automáticas por hoy. "+
				"Si necesitás más ayuda, un asesor puede atenderte.)")
		}
	}

	// 7. Enviar respuesta
	if err := p.sender.SendText(ctx, chatID, response); err != nil {
		return fmt.Errorf("processText: error enviando respuesta: %w", err)
	}

	// 8. Guardar historial async
	go p.saveHistoryAsync(context.Background(), phoneKey, msg.Text, response)

	return nil
}

func (p *Processor) processImage(ctx context.Context, msg *TGMessage, chatID int64, phoneKey string) error {
	// 1. Deduplicación
	dedupKey := fmt.Sprintf("tg:dedup:%d", msg.MessageID)
	isNew, err := p.redis.SetNX(ctx, dedupKey, "1", deduplicationTTL)
	if err != nil {
		return fmt.Errorf("processImage: error verificando deduplicación: %w", err)
	}
	if !isNew {
		slog.Info("telegram: imagen duplicada ignorada", "message_id", msg.MessageID)
		return nil
	}

	// 2. Rate limiting
	if p.rateLimiter != nil {
		result := p.rateLimiter.Check(ctx, phoneKey)
		if result == whatsapp.Deny {
			slog.Warn("telegram: imagen descartada por rate limiter", "chat_id", chatID)
			return nil
		}
	}

	slog.Info("telegram: procesando imagen", "chat_id", chatID, "message_id", msg.MessageID)

	// 2b. Opt-in — disclaimer en primer contacto (reutiliza la clave de texto)
	optinKey := fmt.Sprintf("tg:optin:%d", chatID)
	isFirstContact, _ := p.redis.SetNX(ctx, optinKey, "1", 30*24*time.Hour)
	if isFirstContact {
		_ = p.sender.SendText(ctx, chatID,
			"Al comunicarte con FabricaLaser por este canal, aceptás que "+
				"procesemos tu número, nombre y mensajes para brindarte cotizaciones "+
				"y atención al cliente. Más info: fabricalaser.com/privacidad")
	}

	// 3. Límite diario (el asesor está exento)
	count, limitErr := p.checkDailyLimit(ctx, phoneKey)
	isAsesor := chatID == p.contextProvider.GetAsesorTelegramChatID()
	if limitErr != nil {
		slog.Warn("telegram: error verificando límite diario en imagen, continuando", "error", limitErr)
	} else if !isAsesor {
		maxMsgs := p.contextProvider.GetMaxMensajesDia()
		if count > int64(maxMsgs) {
			slog.Info("telegram: límite diario alcanzado (imagen)", "chat_id", chatID, "count", count)
			_ = p.sender.SendText(ctx, chatID,
				"Hemos alcanzado el límite de mensajes automáticos por hoy. "+
					"Un asesor de FabricaLaser te va a contactar para ayudarte. ¡Gracias por tu paciencia!")

			// Notificar al asesor vía WhatsApp con resumen
			p.notifyAsesorLimit(ctx, chatID, phoneKey, msg, maxMsgs)
			return nil
		}
	}

	// 4. Descargar imagen — tomar la última del array (mayor resolución)
	bestPhoto := msg.Photo[len(msg.Photo)-1]
	imageBytes, mimeType, err := p.sender.GetFileBytes(ctx, bestPhoto.FileID)
	if err != nil {
		slog.Error("telegram: error descargando imagen", "error", err, "file_id", bestPhoto.FileID)
		_ = p.sender.SendText(ctx, chatID,
			"No pude procesar la imagen. ¿Me podés describir qué querés hacer?")
		return fmt.Errorf("processImage: error descargando imagen: %w", err)
	}

	// 5. Cargar historial
	history, err := p.loadHistory(ctx, phoneKey)
	if err != nil {
		slog.Warn("telegram: no se pudo cargar historial para imagen", "error", err)
		history = []whatsapp.ChatTurn{}
	}

	// 6. Construir contexto del usuario
	userCtx := p.buildTelegramUserCtx(msg)

	// 7. Llamar a Gemini con la imagen
	response, err := p.gemini.CallWithImage(ctx, phoneKey, history, imageBytes, mimeType, msg.Caption, userCtx)
	if err != nil {
		slog.Error("telegram: error llamando Gemini con imagen", "error", err)
		_ = p.sender.SendText(ctx, chatID,
			"No pude analizar la imagen. ¿Me podés describir qué querés hacer?")
		return fmt.Errorf("processImage: error llamando Gemini: %w", err)
	}

	// 8. Enviar respuesta
	if err := p.sender.SendText(ctx, chatID, response); err != nil {
		return fmt.Errorf("processImage: error enviando respuesta: %w", err)
	}

	// 9. Guardar historial async
	go p.saveHistoryAsync(context.Background(), phoneKey, "[El cliente mandó una imagen]", response)

	return nil
}

// ─── Historial Redis ─────────────────────────────────────────────────────────

func (p *Processor) loadHistory(ctx context.Context, phoneKey string) ([]whatsapp.ChatTurn, error) {
	key := fmt.Sprintf("tg:hist:%s", phoneKey)
	raw, err := p.redis.Get(ctx, key)
	if err != nil {
		return []whatsapp.ChatTurn{}, nil
	}

	var history []whatsapp.ChatTurn
	if err := json.Unmarshal([]byte(raw), &history); err != nil {
		return nil, fmt.Errorf("loadHistory: error deserializando historial: %w", err)
	}

	if len(history) > maxHistoryTurns {
		history = history[len(history)-maxHistoryTurns:]
	}

	return history, nil
}

func (p *Processor) saveHistoryAsync(ctx context.Context, phoneKey, userMsg, botResponse string) {
	if err := p.updateRedisHistory(ctx, phoneKey, userMsg, botResponse); err != nil {
		slog.Error("telegram: error actualizando historial en Redis", "error", err, "phoneKey", phoneKey)
	}

	now := time.Now()
	turns := []whatsapp.ConversationTurn{
		{Phone: phoneKey, Role: "user", Content: userMsg, CreatedAt: now},
		{Phone: phoneKey, Role: "model", Content: botResponse, CreatedAt: now},
	}

	for _, turn := range turns {
		if err := p.pg.SaveTurn(ctx, turn); err != nil {
			slog.Error("telegram: error archivando turno en PostgreSQL",
				"error", err,
				"phoneKey", phoneKey,
				"role", turn.Role,
			)
		}
	}
}

func (p *Processor) updateRedisHistory(ctx context.Context, phoneKey, userMsg, botResponse string) error {
	key := fmt.Sprintf("tg:hist:%s", phoneKey)

	history, err := p.loadHistory(ctx, phoneKey)
	if err != nil {
		history = []whatsapp.ChatTurn{}
	}

	history = append(history,
		whatsapp.ChatTurn{Role: "user", Content: userMsg},
		whatsapp.ChatTurn{Role: "model", Content: botResponse},
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

// ─── Límite diario ───────────────────────────────────────────────────────────

func (p *Processor) checkDailyLimit(ctx context.Context, phoneKey string) (int64, error) {
	loc, _ := time.LoadLocation("America/Costa_Rica")
	now := time.Now().In(loc)
	fecha := now.Format("2006-01-02")
	key := fmt.Sprintf("tg:limit:%s:%s", phoneKey, fecha)

	count, err := p.redis.Incr(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("checkDailyLimit: error incrementando contador: %w", err)
	}

	if count == 1 {
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
		_ = p.redis.Expire(ctx, key, midnight.Sub(now))
	}

	return count, nil
}

// ─── Notificación al asesor ───────────────────────────────────────────────────

// notifyAsesorLimit envía un resumen de la conversación al asesor vía Telegram
// cuando un cliente de Telegram alcanza el límite diario de mensajes.
// Si no hay TelegramAsesorChatID configurado, hace fallback a WhatsApp.
func (p *Processor) notifyAsesorLimit(ctx context.Context, chatID int64, phoneKey string, msg *TGMessage, maxMsgs int) {
	// Generar resumen de la conversación
	history, _ := p.loadHistory(ctx, phoneKey)
	resumenConversacion := ""
	if len(history) > 0 {
		if s, err := p.gemini.SummarizeConversation(ctx, history); err == nil {
			resumenConversacion = "\n\nResumen de la conversación:\n" + s
		}
	}

	// Datos del cliente de Telegram
	clienteInfo := fmt.Sprintf("Chat ID: %d", chatID)
	if msg.From != nil {
		nombre := msg.From.FirstName
		if msg.From.LastName != "" {
			nombre += " " + msg.From.LastName
		}
		clienteInfo = fmt.Sprintf("Nombre: %s", nombre)
		if msg.From.Username != "" {
			clienteInfo += fmt.Sprintf("\nTelegram: @%s", msg.From.Username)
		}
	}

	resumen := fmt.Sprintf(
		"FabricaLaser — Límite alcanzado (Telegram)\n\n"+
			"⚠️ Cliente alcanzó el límite de %d mensajes hoy.\n"+
			"Requiere atención humana para completar su consulta.\n\n"+
			"%s%s",
		maxMsgs, clienteInfo, resumenConversacion)

	// Intentar enviar por Telegram primero
	asesorChatID := p.contextProvider.GetAsesorTelegramChatID()
	if asesorChatID != 0 {
		if err := p.sender.SendText(ctx, asesorChatID, resumen); err != nil {
			slog.Error("telegram: error notificando al asesor por Telegram",
				"error", err,
				"asesor_chat_id", asesorChatID,
				"chat_id", chatID,
			)
		} else {
			return
		}
	}

	// Fallback a WhatsApp
	asesorPhone := p.contextProvider.GetAsesorPhone()
	if err := p.waSender.SendText(ctx, asesorPhone, resumen); err != nil {
		slog.Error("telegram: error notificando al asesor vía WhatsApp (fallback)",
			"error", err,
			"asesor", asesorPhone,
			"chat_id", chatID,
		)
	}
}

// ─── Contexto de usuario Telegram ────────────────────────────────────────────

func (p *Processor) buildTelegramUserCtx(msg *TGMessage) string {
	var b strings.Builder
	b.WriteString("\n\nDATOS DEL CLIENTE (Telegram):\n")
	b.WriteString("Canal: Telegram\n")

	if msg.From != nil {
		nombre := msg.From.FirstName
		if msg.From.LastName != "" {
			nombre += " " + msg.From.LastName
		}
		b.WriteString(fmt.Sprintf("Nombre: %s\n", nombre))

		if msg.From.Username != "" {
			b.WriteString(fmt.Sprintf("Username: @%s\n", msg.From.Username))
		}
	}

	b.WriteString("Estado: NO registrado en fabricalaser.com (contacto vía Telegram)\n")
	b.WriteString("\nINSTRUCCIONES ESPECÍFICAS PARA TELEGRAM:")
	b.WriteString("\n- El cliente está en TELEGRAM. Cuando escales a un asesor, decile al cliente que el asesor lo contactará por TELEGRAM. NUNCA menciones WhatsApp como medio de contacto con este cliente.")
	b.WriteString("\n- RECORDATORIO CRÍTICO: NUNCA uses 'Pura vida' bajo ninguna circunstancia. Ni como saludo, ni como despedida, ni como afirmación.")
	return b.String()
}
