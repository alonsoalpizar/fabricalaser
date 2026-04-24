package whatsapp

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

	agentctx "github.com/alonsoalpizar/fabricalaser/internal/agent/context"
	"github.com/alonsoalpizar/fabricalaser/internal/agent/llm"
	"github.com/alonsoalpizar/fabricalaser/internal/agent/prompts"
)

const (
	toolLoopMax       = 5
	estimateURL       = "http://localhost:8083/api/v1/quotes/estimate"
	consultarBlankURL = "http://localhost:8083/api/v1/blanks/consultar"
	httpToolTimeout   = 10 * time.Second
)

// Los system prompts (agent_keys "whatsapp_main" y "whatsapp_image") ahora se
// sirven desde prompts.Provider — ver internal/agent/prompts/fallbacks.go para
// los valores originales hardcoded (equivalentes a los const que antes vivían
// en este archivo) y internal/agent/prompts/provider.go para el orden de
// resolución (cache in-memory → DB con timeout 500ms → fallback).

type geminiAdapter struct {
	factory         *llm.Factory
	contextProvider *agentctx.Provider
	promptProvider  *prompts.Provider
	sender          *Sender
	tgSender        *tgSenderAdapter
}

// tgSenderAdapter es un wrapper liviano para enviar mensajes vía Telegram Bot API.
// Se usa solo para notificaciones al asesor cuando el cliente viene de Telegram.
type tgSenderAdapter struct {
	botToken   string
	httpClient *http.Client
}

func (s *tgSenderAdapter) sendText(ctx context.Context, chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	payload := fmt.Sprintf(`{"chat_id":%d,"text":%s}`, chatID, jsonEscapeString(text))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func jsonEscapeString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// NewGeminiAdapter crea un GeminiCaller con soporte de tools y contexto dinámico.
// Recibe un llm.Factory para obtener el cliente activo en cada request (hot reload friendly)
// y un prompts.Provider para resolver los system prompts desde DB con fallback.
func NewGeminiAdapter(factory *llm.Factory, ctxProv *agentctx.Provider, promptProv *prompts.Provider) GeminiCaller {
	return &geminiAdapter{
		factory:         factory,
		contextProvider: ctxProv,
		promptProvider:  promptProv,
		sender:          NewSender(),
		tgSender: &tgSenderAdapter{
			botToken:   os.Getenv("TELEGRAM_BOT_TOKEN"),
			httpClient: &http.Client{Timeout: 15 * time.Second},
		},
	}
}

// CallWithHistory es el método legacy — delega a CallWithTools sin contexto de usuario.
func (g *geminiAdapter) CallWithHistory(ctx context.Context, history []ChatTurn, newMessage string) (string, error) {
	return g.CallWithTools(ctx, "", history, newMessage, "")
}

// SummarizeConversation genera un resumen conciso de la conversación para el asesor.
// Usa temperatura baja y sin tools para obtener un resumen factual.
func (g *geminiAdapter) SummarizeConversation(ctx context.Context, history []ChatTurn) (string, error) {
	if len(history) == 0 {
		return "", nil
	}

	// Armar transcripción plana para resumir
	var sb strings.Builder
	for _, turn := range history {
		role := "Cliente"
		if turn.Role == "model" {
			role = "Agente"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, turn.Content))
	}

	prompt := "Eres un asistente que resume conversaciones de ventas. " +
		"Lee la siguiente conversación entre un cliente de FabricaLaser y el agente virtual. " +
		"Genera un resumen breve (máximo 5 líneas) con: " +
		"qué necesita el cliente, materiales/tecnología mencionados, dimensiones o cantidades indicadas, " +
		"y si mostró intención de compra. Solo datos concretos, sin adornos.\n\n" +
		"Conversación:\n" + sb.String()

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client := g.factory.Client()
	resp, err := client.Chat(ctxTimeout,
		"",
		[]llm.Message{{Role: llm.RoleUser, Content: prompt}},
		nil,
		llm.WithTemperature(0.1),
		llm.WithMaxTokens(300),
	)
	if err != nil {
		return "", fmt.Errorf("SummarizeConversation: %w", err)
	}
	return strings.TrimSpace(resp.Content), nil
}

// buildSystemPrompt compone el system prompt para WhatsApp con el contexto dinámico
// y el contexto del usuario (registrado o no). El cuerpo base se obtiene del
// prompts.Provider bajo el agent_key "whatsapp_main".
func (g *geminiAdapter) buildSystemPrompt(userCtx string) string {
	dynCtx := g.contextProvider.GetDynamicContext()
	return g.promptProvider.Get("whatsapp_main") + dynCtx + userCtx
}

// convertHistoryToLLMMessages transforma []ChatTurn a []llm.Message.
// ChatTurn.Role es "user" o "model" — alineado con llm.RoleUser/RoleModel.
func convertHistoryToLLMMessages(history []ChatTurn) []llm.Message {
	out := make([]llm.Message, 0, len(history))
	for _, t := range history {
		role := llm.RoleUser
		if t.Role == "model" {
			role = llm.RoleModel
		}
		out = append(out, llm.Message{
			Role:    role,
			Content: t.Content,
		})
	}
	return out
}

// CallWithTools llama al LLM con historial, tools habilitadas y contexto dinámico.
// Ejecuta el loop de tool calling hasta toolLoopMax iteraciones.
func (g *geminiAdapter) CallWithTools(ctx context.Context, phone string, history []ChatTurn, newMessage string, userCtx string) (string, error) {
	systemPrompt := g.buildSystemPrompt(userCtx)

	// Construir historial para el LLM
	llmHistory := convertHistoryToLLMMessages(history)
	llmHistory = append(llmHistory, llm.Message{
		Role:    llm.RoleUser,
		Content: newMessage,
	})

	tools := llmTools()

	for iter := 0; iter < toolLoopMax; iter++ {
		client := g.factory.Client() // hot reload friendly
		resp, err := client.Chat(ctx, systemPrompt, llmHistory, tools,
			llm.WithTemperature(0.3),
			llm.WithTopP(0.95),
			llm.WithMaxTokens(1024),
		)
		if err != nil {
			return "", fmt.Errorf("geminiAdapter: error llamando al modelo: %w", err)
		}

		// Sin tool calls → respuesta final
		if len(resp.ToolCalls) == 0 {
			if strings.TrimSpace(resp.Content) != "" {
				return resp.Content, nil
			}
			break
		}

		// Agregar la respuesta del modelo al history (con sus tool_calls)
		llmHistory = append(llmHistory, llm.Message{
			Role:      llm.RoleModel,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Ejecutar cada tool y agregar resultados al history
		for _, tc := range resp.ToolCalls {
			result, err := g.executeToolCall(ctx, phone, tc)
			if err != nil {
				slog.Error("geminiAdapter: error ejecutando tool", "tool", tc.Name, "error", err)
				result = fmt.Sprintf(`{"error":%s}`, jsonEscapeString(err.Error()))
			}
			llmHistory = append(llmHistory, llm.Message{
				Role:       llm.RoleTool,
				Content:    result,
				ToolCallID: tc.ID, // crítico para OpenAI/Anthropic (R11); Vertex ignora
			})
		}
	}

	return "Hubo un problema procesando tu consulta. Por favor escribinos al +506 7018-3073.", nil
}

// CallWithImage llama al LLM con historial y una imagen inline (sin tools).
// Concatena los prompts "whatsapp_main" + "whatsapp_image" del Provider para
// guiar el análisis de la imagen (mismo orden que el prompt histórico pre-migración).
func (g *geminiAdapter) CallWithImage(ctx context.Context, phone string, history []ChatTurn, imageBytes []byte, mimeType string, caption string, userCtx string) (string, error) {
	_ = phone
	mainPrompt := g.promptProvider.Get("whatsapp_main")
	imagePrompt := g.promptProvider.Get("whatsapp_image")
	systemPrompt := mainPrompt + imagePrompt + g.contextProvider.GetDynamicContext() + userCtx

	llmHistory := convertHistoryToLLMMessages(history)

	// Mensaje del usuario con imagen + caption (puede ser vacío)
	userMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: caption,
		Images:  []llm.ImageBlob{{MIMEType: mimeType, Data: imageBytes}},
	}
	if caption == "" {
		userMsg.Content = "El cliente mandó esta imagen."
	}
	llmHistory = append(llmHistory, userMsg)

	client := g.factory.Client()
	resp, err := client.Chat(ctx, systemPrompt, llmHistory, nil,
		llm.WithTemperature(0.7),
		llm.WithTopP(0.95),
		llm.WithMaxTokens(512),
	)
	if err != nil {
		return "", fmt.Errorf("geminiAdapter: error llamando al modelo con imagen: %w", err)
	}

	if strings.TrimSpace(resp.Content) != "" {
		return resp.Content, nil
	}
	return "No pude analizar la imagen. ¿Me podés describir qué querés hacer?", nil
}

// ─── Tool Definitions ────────────────────────────────────────────────────────

// llmTools retorna las 3 tools disponibles para el agente de WhatsApp en formato
// JSON Schema estándar (R7). El adapter Vertex traduce internamente a genai.Schema.
func llmTools() []llm.ToolDef {
	return []llm.ToolDef{
		{
			Name:        "calcular_cotizacion",
			Description: "Calcula el precio estimado de un trabajo de grabado o corte láser según las medidas del área de trabajo. Usar cuando el cliente ya proporcionó material, medidas y cantidad.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"alto_cm": map[string]any{
						"type":        "number",
						"description": "Alto del área a grabar o cortar, en centímetros",
					},
					"ancho_cm": map[string]any{
						"type":        "number",
						"description": "Ancho del área a grabar o cortar, en centímetros",
					},
					"cantidad": map[string]any{
						"type":        "integer",
						"description": "Número de unidades a producir",
					},
					"technology_id": map[string]any{
						"type":        "integer",
						"description": "ID de la tecnología láser a usar (ver IDs al final del system prompt)",
					},
					"material_id": map[string]any{
						"type":        "integer",
						"description": "ID del material a trabajar (ver IDs al final del system prompt)",
					},
					"engrave_type_id": map[string]any{
						"type":        "integer",
						"description": "ID del tipo de grabado: 1=Vectorial, 2=Rasterizado, 3=Fotograbado, 4=3D/Relieve. Default: 1",
					},
					"thickness": map[string]any{
						"type":        "number",
						"description": "Grosor del material en milímetros. Default: 3.0",
					},
					"material_included": map[string]any{
						"type":        "boolean",
						"description": "true si FabricaLaser provee el material, false si el cliente lo trae",
					},
					"incluye_corte": map[string]any{
						"type":        "boolean",
						"description": "true si el trabajo incluye corte del perímetro además del grabado",
					},
					"cut_technology_id": map[string]any{
						"type":        "integer",
						"description": "ID de tecnología para el corte cuando es diferente a la tecnología de grabado. Usar SOLO en Caso 3B: cuando el cliente quiere grabar con UV y cortar con CO2 (acrílico o plástico con grabado+corte). En todos los demás casos omitir este campo.",
					},
				},
				"required": []string{"alto_cm", "ancho_cm", "cantidad", "technology_id", "material_id", "material_included", "incluye_corte"},
			},
		},
		{
			Name:        "consultar_blank",
			Description: "Consulta precio y disponibilidad de un blank (producto preconfigurado) del catálogo de FabricaLaser, como llaveros o medallas. Usar cuando el cliente pregunte por estos productos.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"categoria": map[string]any{
						"type":        "string",
						"description": "Categoría del blank: 'llavero', 'medalla', etc.",
					},
					"cantidad": map[string]any{
						"type":        "integer",
						"description": "Cantidad de unidades que el cliente quiere",
					},
					"blank_id": map[string]any{
						"type":        "integer",
						"description": "ID específico del blank. Usar 0 (o no incluir) si no se conoce — el tool retorna todas las opciones de la categoría",
					},
				},
				"required": []string{"categoria", "cantidad"},
			},
		},
		{
			Name:        "escalar_a_humano",
			Description: "Envía al asesor de ventas un resumen de la conversación cuando el cliente está listo para hacer el pedido o necesita atención personalizada.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resumen": map[string]any{
						"type":        "string",
						"description": "Resumen del contexto de la conversación: qué quiere el cliente, producto, medidas, cantidad, precio estimado si se calculó",
					},
				},
				"required": []string{"resumen"},
			},
		},
	}
}

// ─── Tool Execution ──────────────────────────────────────────────────────────

// executeToolCall dispatcha la ejecución según el nombre de la tool y retorna
// el resultado serializado como JSON string (el formato que el LLM recibe en
// el turno Role=Tool).
func (g *geminiAdapter) executeToolCall(ctx context.Context, clientPhone string, tc llm.ToolCall) (string, error) {
	var (
		result map[string]any
		err    error
	)
	switch tc.Name {
	case "calcular_cotizacion":
		result, err = g.execCalcCotizacion(ctx, tc.Args)
	case "consultar_blank":
		result, err = g.execConsultarBlank(ctx, tc.Args)
	case "escalar_a_humano":
		result, err = g.execEscalarAHumano(ctx, clientPhone, tc.Args)
	default:
		return "", fmt.Errorf("tool desconocida: %s", tc.Name)
	}
	if err != nil {
		return "", err
	}
	buf, mErr := json.Marshal(result)
	if mErr != nil {
		return "", fmt.Errorf("executeToolCall: error serializando resultado: %w", mErr)
	}
	return string(buf), nil
}

func (g *geminiAdapter) execCalcCotizacion(ctx context.Context, args map[string]any) (map[string]any, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("execCalcCotizacion: error serializando args: %w", err)
	}

	internalToken := os.Getenv("INTERNAL_API_TOKEN")

	httpCtx, cancel := context.WithTimeout(ctx, httpToolTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, estimateURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("execCalcCotizacion: error creando request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if internalToken != "" {
		req.Header.Set("Authorization", "Bearer "+internalToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execCalcCotizacion: error llamando al endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("execCalcCotizacion: error leyendo respuesta: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("execCalcCotizacion: error deserializando respuesta: %w", err)
	}

	slog.Info("whatsapp: calcular_cotizacion ejecutado",
		"status", resp.StatusCode,
		"precio_estimado", result["precio_estimado"],
	)

	return result, nil
}

func (g *geminiAdapter) execConsultarBlank(ctx context.Context, args map[string]any) (map[string]any, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank: error serializando args: %w", err)
	}

	internalToken := os.Getenv("INTERNAL_API_TOKEN")

	httpCtx, cancel := context.WithTimeout(ctx, httpToolTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodPost, consultarBlankURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank: error creando request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if internalToken != "" {
		req.Header.Set("Authorization", "Bearer "+internalToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank: error llamando al endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("execConsultarBlank: error leyendo respuesta: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("execConsultarBlank: error deserializando respuesta: %w", err)
	}

	slog.Info("whatsapp: consultar_blank ejecutado",
		"categoria", args["categoria"],
		"cantidad", args["cantidad"],
		"encontrado", result["encontrado"],
	)

	return result, nil
}

func (g *geminiAdapter) execEscalarAHumano(ctx context.Context, clientPhone string, args map[string]any) (map[string]any, error) {
	resumen, _ := args["resumen"].(string)

	var msg strings.Builder
	msg.WriteString("FabricaLaser — Cliente listo para coordinar\n\n")
	msg.WriteString(resumen)

	// Detectar canal por prefijo del identificador y enviar por el canal correspondiente
	if strings.HasPrefix(clientPhone, "tg:") {
		msg.WriteString(fmt.Sprintf("\n\nCanal: Telegram\nChat ID: %s", strings.TrimPrefix(clientPhone, "tg:")))

		// Enviar al asesor por Telegram
		asesorChatID := g.contextProvider.GetAsesorTelegramChatID()
		if asesorChatID != 0 {
			if err := g.tgSender.sendText(ctx, asesorChatID, msg.String()); err != nil {
				slog.Error("escalar_a_humano: error enviando al asesor por Telegram",
					"asesor_chat_id", asesorChatID,
					"error", err,
				)
				return map[string]any{"enviado": false, "error": err.Error()}, nil
			}
			slog.Info("escalar_a_humano: mensaje enviado al asesor por Telegram",
				"asesor_chat_id", asesorChatID,
				"cliente", clientPhone,
			)
			return map[string]any{"enviado": true}, nil
		}
		// Fallback a WhatsApp si no hay TelegramAsesorChatID configurado
		slog.Warn("escalar_a_humano: TelegramAsesorChatID no configurado, fallback a WhatsApp")
	} else if clientPhone != "" {
		msg.WriteString(fmt.Sprintf("\n\nNúmero del cliente: %s", clientPhone))
	}

	// Enviar al asesor por WhatsApp (canal por defecto o fallback)
	asesorPhone := g.contextProvider.GetAsesorPhone()
	if err := g.sender.SendText(ctx, asesorPhone, msg.String()); err != nil {
		slog.Error("escalar_a_humano: error enviando al asesor por WhatsApp",
			"asesor", asesorPhone,
			"error", err,
		)
		return map[string]any{"enviado": false, "error": err.Error()}, nil
	}

	slog.Info("escalar_a_humano: mensaje enviado al asesor por WhatsApp",
		"asesor", asesorPhone,
		"cliente", clientPhone,
	)
	return map[string]any{"enviado": true}, nil
}
