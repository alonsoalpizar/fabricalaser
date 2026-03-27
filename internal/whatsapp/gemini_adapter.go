package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	waProjectID = "div-aloalpizar"
	waLocation  = "us-central1"
	waModelName = "gemini-2.5-flash"

	toolLoopMax    = 5
	estimateURL    = "http://localhost:8083/api/v1/quotes/estimate"
	httpToolTimeout = 10 * time.Second
)

// systemPromptWA — prompt para el agente de WhatsApp de FabricaLaser.
// Sin markdown ni asteriscos en respuestas. Con flujo de cotización en pasos.
const systemPromptWA = `Sos el asistente virtual de FabricaLaser por WhatsApp, empresa costarricense de corte y grabado láser en Tibás, San José.

REGLA DE FORMATO: Nunca uses asteriscos, guiones para listas, ni markdown de ningún tipo. Usá solo texto plano y emojis cuando sea natural. Respuestas conversacionales, cortas y directas como en WhatsApp.

PERSONALIDAD:
Hablás de "vos", español costarricense casual pero profesional. Directo, conocés el negocio. Si la respuesta es corta, la das corta. Máximo 3 párrafos por mensaje.
PROHIBIDO usar "Pura vida" como muletilla o saludo automático. No lo uses al inicio de cada mensaje. Si lo usás, que sea una vez en toda la conversación y solo si el momento lo amerita de verdad. Respondé de forma natural y variada.

TECNOLOGÍAS LÁSER (la matriz te indica qué tecnología es compatible con cada material):
CO2: madera, MDF, acrílico, cuero, vidrio, cerámica
UV: acrílico premium, plásticos delicados (ABS/PC), vidrio fino, cuero de alta gama
Fibra: acero inoxidable, aluminio, cobre, titanio, herramientas
MOPA: aluminio anodizado con color, acero con coloración, joyería metálica

MATERIALES Y ESPESORES:
Madera/MDF: 3mm, 6mm, 9mm, 12mm
Acrílico: 2mm a 10mm
Cuero/Piel: varios grosores
Vidrio/Cristal: superficies planas
Cerámica: azulejos y piezas con coating
Plástico ABS/PC: señalética y piezas técnicas
Metal con coating: aluminio anodizado, acero pintado

CATÁLOGO — PIEZAS ESTÁNDAR (sin cotizador):
Llaveros de acrílico 5cm (blanco o transparente):
25 unidades: 6.000 colones (240 c/u)
50 unidades: 11.000 colones (220 c/u)
100 unidades: 18.000 colones (180 c/u)
Formas: Redondo, Cuadrado, Hexágono, Corazón, Rectángulo, Escudo
Mínimo 25 por forma. No se mezclan formas en un mismo paquete.

Medallas de acrílico 7cm (blanco o transparente):
Mínimo 50 unidades
50-100 unidades: 375 colones c/u

FLUJO DE COTIZACIÓN PERSONALIZADA (10 pasos):
Cuando el cliente pide cotización para un proyecto con diseño propio:
1. Confirmá qué quiere hacer (grabar, cortar, ambos)
2. Preguntá el material
3. Preguntá las medidas del área a trabajar (alto y ancho en cm)
4. Preguntá la cantidad de unidades
4b. Si el trabajo incluye grabado, preguntá:
    "¿El grabado es con relleno (como un sello, foto o área completa) o solo los contornos y líneas del diseño?"
    Relleno o foto → usá engrave_type_id=2 (Rasterizado)
    Solo contornos o líneas → usá engrave_type_id=1 (Vectorial)
4c. Preguntá por el archivo:
    "¿Tenés el diseño en formato SVG o imagen vectorial lista para trabajar?"
    Sí tiene → sin costo adicional de vectorización
    No tiene → debés sumar el CostoVectorizacion al precio final (está en la Configuración operativa del contexto)
5. Confirmá si FabricaLaser provee el material o el cliente lo trae
6. Con toda esa info, usá la herramienta calcular_cotizacion para obtener el precio
7. Presentá el resultado así:
   Si el cliente SÍ tiene archivo SVG:
   "Para [cantidad] [descripción] en [material], el precio de referencia es ₡[precio_estimado] (₡[precio_unitario] c/u). ¿Te interesa coordinar el pedido?"
   Si el cliente NO tiene archivo SVG:
   "Para [cantidad] [descripción] en [material]:
    Grabado: ₡[precio_estimado]
    Vectorización del diseño: ₡[CostoVectorizacion]
    Total estimado: ₡[precio_estimado + CostoVectorizacion] (₡[unitario_con_vectorizacion] c/u)
    ¿Te interesa coordinar el pedido?"
8. Cuando el cliente esté listo para confirmar, usá escalar_a_humano

CUÁNDO ESCALAR A HUMANO:
Usá la herramienta escalar_a_humano cuando:
El cliente dice que quiere hacer el pedido o está listo para confirmar
El cliente pide hablar con una persona
La consulta es muy técnica o requiere revisión de diseño
El trabajo necesita revisión según el resultado de la cotización

RETIRO Y ENVÍOS:
Taller: Avenida 67, San Jerónimo, Tibás, San José. Solo con cita previa coordinada por WhatsApp.
Envíos a todo el país. 3.500 colones el primer kilo por Correos CR o mensajería.
Tiempo de producción: 1 día hábil desde confirmación de pago.

RESTRICCIONES:
No confirmes precios distintos a los de la tabla de catálogo
No prometás fechas específicas
No hagás reservas por este chat
Si no sabés algo, decilo y escalá a humano

Los IDs exactos de tecnologías y materiales para las herramientas están al final de este prompt (datos en tiempo real desde la base de datos).`

type geminiAdapter struct {
	client          *genai.Client
	contextProvider *waContextProvider
	sender          *Sender
}

// NewGeminiAdapter crea un GeminiCaller con soporte de tools y contexto dinámico.
func NewGeminiAdapter(provider *waContextProvider) GeminiCaller {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, waProjectID, waLocation)
	if err != nil {
		log.Fatalf("whatsapp: failed to create Vertex AI client: %v", err)
	}
	return &geminiAdapter{
		client:          client,
		contextProvider: provider,
		sender:          NewSender(),
	}
}

// CallWithHistory es el método legacy — delega a CallWithTools sin teléfono de cliente.
func (g *geminiAdapter) CallWithHistory(ctx context.Context, history []ChatTurn, newMessage string) (string, error) {
	return g.CallWithTools(ctx, "", history, newMessage)
}

// CallWithTools llama a Gemini con historial, tools habilitadas y contexto dinámico.
// Ejecuta el loop de tool calling hasta toolLoopMax iteraciones.
func (g *geminiAdapter) CallWithTools(ctx context.Context, phone string, history []ChatTurn, newMessage string) (string, error) {
	model := g.client.GenerativeModel(waModelName)

	dynCtx := g.contextProvider.GetDynamicContext()
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPromptWA + dynCtx)},
	}
	model.Tools = []*genai.Tool{{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			calcularCotizacionTool(),
			escalarAHumanoTool(),
		},
	}}
	model.SetTemperature(0.3)
	model.SetTopP(0.95)
	model.SetMaxOutputTokens(1024)

	chat := model.StartChat()
	for _, turn := range history {
		chat.History = append(chat.History, &genai.Content{
			Role:  turn.Role,
			Parts: []genai.Part{genai.Text(turn.Content)},
		})
	}

	resp, err := chat.SendMessage(ctx, genai.Text(newMessage))
	if err != nil {
		return "", fmt.Errorf("geminiAdapter: error llamando al modelo: %w", err)
	}

	// Loop de tool calling
	for i := 0; i < toolLoopMax; i++ {
		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			break
		}

		// Buscar FunctionCall en los parts de la respuesta
		var fc *genai.FunctionCall
		for _, part := range resp.Candidates[0].Content.Parts {
			if call, ok := part.(genai.FunctionCall); ok {
				fc = &call
				break
			}
		}

		if fc == nil {
			// Sin tool call → extraer texto y retornar
			var sb strings.Builder
			for _, part := range resp.Candidates[0].Content.Parts {
				if text, ok := part.(genai.Text); ok {
					sb.WriteString(string(text))
				}
			}
			return sb.String(), nil
		}

		// Ejecutar la función
		result, err := g.executeFunction(ctx, phone, fc)
		if err != nil {
			slog.Error("geminiAdapter: error ejecutando tool", "tool", fc.Name, "error", err)
			result = map[string]any{"error": err.Error()}
		}

		// Enviar FunctionResponse al modelo (con retry en caso de 429)
		resp, err = sendWithRetry(ctx, chat, genai.FunctionResponse{
			Name:     fc.Name,
			Response: result,
		})
		if err != nil {
			return "", fmt.Errorf("geminiAdapter: error enviando FunctionResponse: %w", err)
		}
	}

	// Extraer texto del último resp
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		var sb strings.Builder
		for _, part := range resp.Candidates[0].Content.Parts {
			if text, ok := part.(genai.Text); ok {
				sb.WriteString(string(text))
			}
		}
		if sb.Len() > 0 {
			return sb.String(), nil
		}
	}

	return "Hubo un problema procesando tu consulta. Por favor escribinos al +506 7018-3073.", nil
}

// ─── Tool Definitions ────────────────────────────────────────────────────────

func calcularCotizacionTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "calcular_cotizacion",
		Description: "Calcula el precio estimado de un trabajo de grabado o corte láser según las medidas del área de trabajo. Usar cuando el cliente ya proporcionó material, medidas y cantidad.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"alto_cm": {
					Type:        genai.TypeNumber,
					Description: "Alto del área a grabar o cortar, en centímetros",
				},
				"ancho_cm": {
					Type:        genai.TypeNumber,
					Description: "Ancho del área a grabar o cortar, en centímetros",
				},
				"cantidad": {
					Type:        genai.TypeInteger,
					Description: "Número de unidades a producir",
				},
				"technology_id": {
					Type:        genai.TypeInteger,
					Description: "ID de la tecnología láser a usar (ver IDs al final del system prompt)",
				},
				"material_id": {
					Type:        genai.TypeInteger,
					Description: "ID del material a trabajar (ver IDs al final del system prompt)",
				},
				"engrave_type_id": {
					Type:        genai.TypeInteger,
					Description: "ID del tipo de grabado: 1=Vectorial, 2=Rasterizado, 3=Fotograbado, 4=3D/Relieve. Default: 1",
				},
				"thickness": {
					Type:        genai.TypeNumber,
					Description: "Grosor del material en milímetros. Default: 3.0",
				},
				"material_included": {
					Type:        genai.TypeBoolean,
					Description: "true si FabricaLaser provee el material, false si el cliente lo trae",
				},
				"incluye_corte": {
					Type:        genai.TypeBoolean,
					Description: "true si el trabajo incluye corte del perímetro además del grabado",
				},
			},
			Required: []string{"alto_cm", "ancho_cm", "cantidad", "technology_id", "material_id", "material_included", "incluye_corte"},
		},
	}
}

func escalarAHumanoTool() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        "escalar_a_humano",
		Description: "Envía al asesor de ventas un resumen de la conversación cuando el cliente está listo para hacer el pedido o necesita atención personalizada.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"resumen": {
					Type:        genai.TypeString,
					Description: "Resumen del contexto de la conversación: qué quiere el cliente, producto, medidas, cantidad, precio estimado si se calculó",
				},
			},
			Required: []string{"resumen"},
		},
	}
}

// ─── Tool Execution ──────────────────────────────────────────────────────────

func (g *geminiAdapter) executeFunction(ctx context.Context, clientPhone string, fc *genai.FunctionCall) (map[string]any, error) {
	switch fc.Name {
	case "calcular_cotizacion":
		return g.execCalcCotizacion(ctx, fc.Args)
	case "escalar_a_humano":
		return g.execEscalarAHumano(ctx, clientPhone, fc.Args)
	default:
		return nil, fmt.Errorf("tool desconocida: %s", fc.Name)
	}
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

func (g *geminiAdapter) execEscalarAHumano(ctx context.Context, clientPhone string, args map[string]any) (map[string]any, error) {
	resumen, _ := args["resumen"].(string)
	asesorPhone := g.contextProvider.GetAsesorPhone()

	var msg strings.Builder
	msg.WriteString("FabricaLaser — Cliente listo para coordinar\n\n")
	msg.WriteString(resumen)
	if clientPhone != "" {
		msg.WriteString(fmt.Sprintf("\n\nNúmero del cliente: %s", clientPhone))
	}

	if err := g.sender.SendText(ctx, asesorPhone, msg.String()); err != nil {
		slog.Error("whatsapp: escalar_a_humano — error enviando al asesor",
			"asesor", asesorPhone,
			"error", err,
		)
		return map[string]any{"enviado": false, "error": err.Error()}, nil
	}

	slog.Info("whatsapp: escalar_a_humano — mensaje enviado al asesor",
		"asesor", asesorPhone,
		"cliente", clientPhone,
	)
	return map[string]any{"enviado": true}, nil
}

// ─── Retry helper ────────────────────────────────────────────────────────────

// sendWithRetry envía un mensaje al chat con reintentos exponenciales ante errores 429.
// Vertex AI puede retornar ResourceExhausted (429) en el segundo turno del tool loop
// cuando se envía el FunctionResponse. Hasta 3 reintentos: 2s, 4s, 8s.
func sendWithRetry(ctx context.Context, chat *genai.ChatSession, part genai.Part) (*genai.GenerateContentResponse, error) {
	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		resp, err := chat.SendMessage(ctx, part)
		if err == nil {
			return resp, nil
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.ResourceExhausted {
			// Error no recuperable — retornar de inmediato
			return nil, err
		}

		if attempt == len(delays) {
			lastErr = err
			break
		}

		slog.Warn("geminiAdapter: Vertex AI 429 — reintentando",
			"attempt", attempt+1,
			"wait", delays[attempt],
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delays[attempt]):
		}

		lastErr = err
	}

	return nil, fmt.Errorf("sendWithRetry: agotados los reintentos tras 429: %w", lastErr)
}
