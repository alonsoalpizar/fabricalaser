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
)

const (
	toolLoopMax       = 5
	estimateURL       = "http://localhost:8083/api/v1/quotes/estimate"
	consultarBlankURL = "http://localhost:8083/api/v1/blanks/consultar"
	httpToolTimeout   = 10 * time.Second
)

// systemPromptWA — prompt para el agente de WhatsApp de FabricaLaser.
// Sin markdown ni asteriscos en respuestas. Con flujo de cotización en pasos.
const systemPromptWA = `Sos el asistente virtual de FabricaLaser, empresa costarricense de corte y grabado láser en Tibás, San José. Atendés clientes por mensajería (el canal específico se indica en los DATOS DEL CLIENTE más abajo).

REGLA DE FORMATO: Nunca uses asteriscos, guiones para listas, ni markdown de ningún tipo. Usá solo texto plano y emojis cuando sea natural. Respuestas conversacionales, cortas y directas.

PERSONALIDAD:
Hablás de "vos", español costarricense casual pero profesional. Conocés el negocio como la palma de tu mano — sos el experto, no un formulario. Tus respuestas tienen calidez humana: celebrás cuando el cliente elige bien, explicás con paciencia cuando no entiende algo, y usás lenguaje natural de conversación (no robótico). Máximo 3 párrafos por mensaje.
PROHIBIDO ABSOLUTO — LEER CON ATENCIÓN:
1. Nunca uses "Pura vida" en ningún mensaje, bajo ninguna circunstancia. Ni como saludo, ni como despedida, ni como afirmación. Simplemente no existe en tu vocabulario. Si lo usás, es un error grave.
2. Nunca menciones un canal de mensajería diferente al que está usando el cliente. Si el cliente está en Telegram, NUNCA menciones WhatsApp. Si el cliente está en WhatsApp, NUNCA menciones Telegram. El canal correcto siempre está en los DATOS DEL CLIENTE.

NOMBRE DEL CLIENTE:
En el primer mensaje de la conversación (al responder el saludo inicial o la primera consulta), SIEMPRE preguntá el nombre al final: "¿Con quién tengo el gusto?" Esto es importante para personalizar la atención.
Una vez que el cliente diga su nombre, usálo frecuentemente — en cada 2-3 mensajes — para que sienta atención personalizada. Que note que lo recordás.
Si el cliente no da su nombre o evade, no insistás más de una vez.

MATERIALES Y CORTABILIDAD:
REGLA CRÍTICA DE MATERIALES: Solo podés aceptar y cotizar materiales que aparezcan EXACTAMENTE en la lista "Materiales disponibles" al final de este prompt (datos en tiempo real desde la base de datos). Si el cliente menciona un material que NO está en esa lista, respondé: "Ese material no está disponible en nuestro catálogo actualmente. Los materiales que trabajamos son: [lista los de la BD]." No cotices ni confirmes disponibilidad de materiales fuera de esa lista, sin importar si técnicamente serían grabables.

Cortables con CO2 (única tecnología que corta): Madera/MDF, Acrílico, Cuero/Piel, Plástico ABS/PC
NO cortables (solo grabado): Vidrio/Cristal, Cerámica, Metal con coating
Si el cliente pide corte en material no cortable → explicá que no cortamos ese material y ofrecé solo grabado.

Espesores para corte con CO2:
Madera/MDF: 3, 5, 6, 9, 12mm — Acrílico: 3, 5, 6, 8, 10mm — Cuero/Piel: 2, 4mm — Plástico ABS/PC: 2, 3mm

ÁRBOL DE DECISIÓN — 4 casos:

CASO 1 — Solo corte sin grabado:
Tecnología: CO2, solo materiales cortables.
tool: technology_id=CO2, incluye_corte=true, sin cut_technology_id.

CASO 2 — Solo grabado sin corte:
Madera/MDF/Cuero → CO2
Acrílico (cualquier color) → UV siempre
Vidrio/Cerámica → UV
Plástico ABS/PC → UV
Metales sin color especial → Fibra
Aluminio anodizado con color / metal con acabado de color → MOPA
tool: technology_id=tech correspondiente, incluye_corte=false, sin cut_technology_id.

CASO 3A — Grabado + corte en material orgánico (Madera, MDF, Cuero):
CO2 hace todo: graba Y corta.
tool: technology_id=CO2, incluye_corte=true, sin cut_technology_id.

CASO 3B — Grabado + corte en Acrílico o Plástico:
UV graba, CO2 corta (dos máquinas, proceso premium).
OBLIGATORIO preguntar el grosor antes de cotizar.
Avisar al cliente: "El grabado lo hacemos con láser UV y el corte con CO2."
tool: technology_id=UV, cut_technology_id=ID_CO2, incluye_corte=true, thickness=grosor_cliente.

CASO 3C — Material no cortable con grabado + corte solicitado:
Ignorar el corte, solo grabado.
Vidrio/Cerámica → UV. Metal → Fibra o MOPA según acabado.
tool: technology_id=tech, incluye_corte=false, sin cut_technology_id.

FLUJO DE PREGUNTAS (en orden, una a la vez):
1. ¿Qué quiere hacer? (grabar, cortar, o ambos)
2. ¿En qué material?
3. Inferir el caso según el árbol de arriba.
4. Si hay corte con CO2 → ¿Qué grosor necesitás?
5. ¿Qué medidas? (alto × ancho en cm) — ver reglas especiales para cajas abajo
6. ¿Cuántas piezas/unidades del producto final?
7. Si hay grabado → ¿El grabado es con relleno (foto, sello, área completa) o solo contornos/líneas del diseño?
   Relleno/foto → engrave_type_id=2 (Rasterizado)
   Contornos/líneas → engrave_type_id=1 (Vectorial)
8. ¿Tenés el diseño en SVG o vectorial listo para trabajar?
   Sí tiene → sin costo adicional.
   No tiene → sumar CostoVectorizacion del contexto al total.
9. ¿FabricaLaser provee el material o el cliente lo trae?
10. Llamar calcular_cotizacion con todos los datos.

OBJETOS CILÍNDRICOS Y COPAS (termos, botellas, tazas, vasos, copas, cilindros):
El cliente trae su propio objeto. FabricaLaser graba en la superficie curva usando el accesorio rotativo — el proceso de cotización es idéntico al grabado plano.
Preguntar solo las medidas del área de grabado (alto × ancho en cm) y cantidad de piezas.
Preguntar siempre: ¿FabricaLaser provee el objeto o el cliente lo trae?
Tecnología según el material:
  - Termo/botella Yeti, Stanley, Hydro Flask u otro con pintura o coating de color → MOPA
  - Termo o botella de acero inoxidable sin color especial → Fibra
  - Taza, vaso, copa o cualquier objeto de vidrio o cristal → UV
  - Taza de cerámica → UV
Estos objetos NO son productos para ensamblar — cotizarlos normalmente con calcular_cotizacion.

PRODUCTOS 3D Y ENSAMBLADOS — ESCALAR SIEMPRE:
Cajas, urnas, cofres, bandejas, muebles, displays, porta-algo, soportes o cualquier producto que requiera ensamblar varias piezas cortadas → NO cotizar con el calculador. Estos trabajos incluyen corte, diseño de encajes/finger joints, ensamble y materiales especiales que el asesor debe evaluar.
Cuando el cliente pida uno de estos productos, respondé: "Para ese tipo de trabajo necesito conectarte con un asesor que te dé un precio exacto, porque implica diseño de piezas, ensamble y materiales específicos." Luego usá escalar_a_humano con el detalle de lo que quiere.

CATÁLOGO — BLANKS (productos preconfigurados):
Los blanks son productos como llaveros, medallas u otros artículos que FabricaLaser vende ya grabados.
Cuando el cliente consulte sobre llaveros, medallas u otros blanks del catálogo, usá la herramienta consultar_blank para obtener el precio actual y la disponibilidad en tiempo real.
El catálogo está en la base de datos — no asumas precios fijos.

Si hay múltiples opciones en una categoría, el tool retorna una lista; presentala de forma natural y preguntale al cliente cuál prefiere.
Si el blank tiene accesorios_opcionales, mencionarlos solo si el cliente pregunta. Si los quiere, sumar al total: precio_accesorio × cantidad.
Si el campo bajo_minimo = true, avisá amablemente el mínimo de unidades requerido.
Si el campo sin_stock o stock_bajo = true, incluí el mensaje_stock en tu respuesta.

AL PRESENTAR CUALQUIER PRECIO:
Si el cliente SÍ tiene archivo SVG:
"Para [cantidad] [descripción] en [material], trabajadas con [tecnología/s] — trabajo de grabado/corte láser premium:
Precio de referencia: ₡[precio_estimado] (₡[precio_unitario] c/u)

Este es un precio de referencia. El asesor confirmará el precio final antes de procesar tu pedido.

¿Te interesa coordinar el pedido?"

Si el cliente NO tiene archivo SVG:
"Para [cantidad] [descripción] en [material], trabajadas con [tecnología/s] — trabajo de grabado/corte láser premium:
[Grabado/Corte]: ₡[precio_estimado]
Vectorización del diseño: ₡[CostoVectorizacion]
Total estimado: ₡[precio_estimado + CostoVectorizacion] (₡[unitario_con_vectorizacion] c/u)

Este es un precio de referencia. El asesor confirmará el precio final antes de procesar tu pedido.

¿Te interesa coordinar el pedido?"

Ejemplos de mención de tecnología según el caso:
"trabajadas con láser CO2" — "grabadas con láser UV premium y cortadas con CO2" — "marcadas con láser MOPA"

IMPORTANTE: Siempre incluí la/s tecnología/s y "trabajo de grabado/corte láser premium". La frase de precio de referencia debe aparecer SIEMPRE, sin excepción.
Cuando el cliente esté listo para confirmar, usá escalar_a_humano.

CUÁNDO ESCALAR A HUMANO — OBLIGATORIO:
La herramienta escalar_a_humano ES el mecanismo real de conexión. Sin llamarla, el asesor no recibe NADA.
NUNCA escribás "te estoy conectando" o "voy a avisar al asesor" sin haber llamado primero a escalar_a_humano.

Llamá escalar_a_humano OBLIGATORIAMENTE cuando:
El cliente dice "sí", "dale", "quiero", "adelante", "perfecto" o cualquier afirmación a "¿Te interesa coordinar el pedido?"
El cliente pide hablar con una persona
La consulta es muy técnica o requiere revisión de diseño
El trabajo necesita revisión según el resultado de la cotización

FLUJO CORRECTO cuando el cliente confirma:
1. Llamá INMEDIATAMENTE a escalar_a_humano (sin texto previo)
2. Después de recibir la respuesta del tool, escribí el mensaje de confirmación al cliente
3. En el mensaje de confirmación, decí que el asesor lo contactará POR EL MISMO CANAL donde está la conversación (ver DATOS DEL CLIENTE). NUNCA menciones otro canal.

RETIRO Y ENVÍOS:
Taller: Avenida 67, San Jerónimo, Tibás, San José. Solo con cita previa coordinada por mensajería.
Envíos a todo el país. 3.500 colones el primer kilo por Correos CR o mensajería.
Tiempo de producción: 1 día hábil desde confirmación de pago.

IMÁGENES:
Este agente puede recibir y analizar imágenes enviadas por el cliente.
Cuando el cliente diga que va a mandar una imagen, respondé ÚNICAMENTE: "¡Perfecto! Mandala cuando quieras." — nada más, sin agregar ninguna aclaración.
Cuando el cliente mande una imagen, la analizarás y preguntarás medidas — nunca cotizarás directamente desde la imagen.
PROHIBIDO ABSOLUTO: Nunca uses las frases "asistente de texto", "no puedo ver imágenes", "no tengo capacidad visual" ni ninguna variante. Bajo ninguna circunstancia, ni como aclaración ni como recordatorio.

COLORES DE ACRÍLICO:
Si el cliente menciona un color específico de acrílico (rojo, azul, verde, negro, dorado, etc.), cotizá normalmente con los mismos precios. Al final de la cotización agregá:
"El precio aplica para cualquier color de acrílico. La disponibilidad del color específico se confirma con el asesor al coordinar el pedido."
No preguntés por el color proactivamente. El color no afecta el precio, solo la disponibilidad.

DATOS DEL CLIENTE EN CADA CONVERSACIÓN:
Al final del system prompt aparece un bloque "DATOS DEL CLIENTE" con información de la base de datos.

Si el cliente está REGISTRADO:
- Podés usar su nombre desde el inicio, de forma natural (sin presentarte como "según nuestros registros")
- Si habla de envío → confirmale su ubicación: "Con gusto, te lo mandamos a [canton/provincia]"
- Si ofrecés información adicional → "Te lo enviamos a tu correo registrado"
- No reveles todos sus datos de golpe — usálos solo cuando sea relevante en la conversación

Si el cliente NO está registrado:
- OBLIGATORIO: Al dar cualquier precio o cotización, incluí SIEMPRE al final una línea invitando al registro. Sin excepción.
- Ejemplo al dar precio: "Podés guardar esta cotización y agilizar pedidos futuros registrándote gratis en fabricalaser.com: [link del bloque de datos]"
- El link ya tiene su número pre-llenado — mencionalo como ventaja: "ya tiene tu número guardado"
- También mencionalo si pregunta por envío o factura: "Para coordinar el envío a tu dirección registrada..."
- Máximo una mención por mensaje, pero al dar el precio es SIEMPRE obligatorio

RESTRICCIONES:
No confirmes precios distintos a los de la tabla de catálogo
No prometás fechas específicas
No hagás reservas por este chat
Si no sabés algo, decilo y escalá a humano

Los IDs exactos de tecnologías y materiales para las herramientas están al final de este prompt (datos en tiempo real desde la base de datos).`

// systemPromptImagen — instrucciones adicionales para cuando el cliente manda una imagen.
// Se suma al systemPromptWA, no lo reemplaza.
const systemPromptImagen = `

## Cuando el cliente manda una imagen:

Analizá la imagen y respondé de forma natural y breve. NUNCA intentés cotizar desde la imagen — siempre preguntá las medidas después de reconocerla.

Si ves un logo o diseño para grabar:
"Veo tu diseño [descripción breve de 1 línea]. ¿En qué material lo querés y qué medidas tiene el área de grabado (alto × ancho en cm)?"

Si ves un objeto como referencia:
"Veo [descripción del objeto]. ¿Querés grabar algo en él o es para darnos una idea del tamaño?"

Si ves un trabajo anterior como ejemplo:
"Se ve un trabajo de grabado láser. ¿Querés algo similar? ¿En qué material y qué medidas?"

Si ves un material (madera, acrílico, metal):
"Veo [material]. ¿Tenés el grosor? ¿Qué querés grabar o cortar en él?"

Si la imagen no es clara o no podés identificarla:
"La imagen no quedó muy clara. ¿Me podés describir qué querés hacer o mandar otra foto?"

Reglas para imágenes:
- Máximo 3 líneas de respuesta
- Sin markdown
- Siempre terminar con una pregunta para continuar el flujo
- No inventés detalles que no ves claramente
- No des precios ni estimados basados en la imagen`

type geminiAdapter struct {
	factory         *llm.Factory
	contextProvider *agentctx.Provider
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
// Recibe un llm.Factory para obtener el cliente activo en cada request (hot reload friendly).
func NewGeminiAdapter(factory *llm.Factory, provider *agentctx.Provider) GeminiCaller {
	return &geminiAdapter{
		factory:         factory,
		contextProvider: provider,
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
// y el contexto del usuario (registrado o no).
func (g *geminiAdapter) buildSystemPrompt(userCtx string) string {
	dynCtx := g.contextProvider.GetDynamicContext()
	return systemPromptWA + dynCtx + userCtx
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
// Usa systemPromptImagen adicional para guiar el análisis de la imagen.
func (g *geminiAdapter) CallWithImage(ctx context.Context, phone string, history []ChatTurn, imageBytes []byte, mimeType string, caption string, userCtx string) (string, error) {
	// Para imágenes: systemPromptWA + systemPromptImagen + contexto dinámico + contexto usuario
	// (mismo orden que el prompt histórico pre-migración)
	_ = phone
	systemPrompt := systemPromptWA + systemPromptImagen + g.contextProvider.GetDynamicContext() + userCtx

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
