// Package llm abstrae el proveedor LLM activo detrás de una interface única.
// Permite conmutar entre Vertex AI, OpenAI-compat (DeepSeek/Kimi/Groq/OpenAI) y Anthropic
// desde el admin UI con hot reload sin reiniciar el servicio.
//
// Contrato:
//   - R1: Solo archivos en este paquete pueden importar SDKs de proveedores.
//   - R7: ToolDef.Parameters usa JSON Schema estándar (map[string]any).
//   - R10: Todos los adapters soportan imágenes (para preservar WhatsApp visual).
//   - R11: ToolCallID roundtrip obligatorio para OpenAI/Anthropic.
//   - R12: Response incluye LatencyMS + TokensIn/Out para observabilidad.
//   - R13: Retry es responsabilidad interna del adapter (cada proveedor tiene códigos distintos).
package llm

import "context"

// Role identifica el emisor de un mensaje en la conversación.
type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model" // traducido a "assistant" en OpenAI/Anthropic
	RoleTool  Role = "tool"  // respuesta de una tool invocada por el modelo
)

// ImageBlob es una imagen enviada por el cliente para análisis visual (R10).
// WhatsApp la usa cuando el cliente manda una foto — el agente la describe y pide medidas.
type ImageBlob struct {
	MIMEType string // "image/jpeg", "image/png", "image/webp"
	Data     []byte // bytes crudos (el adapter se encarga de base64/blob según proveedor)
}

// Message es un turno de conversación. El campo aplicable depende de Role:
//   - Role=User:  Content (texto), opcionalmente Images
//   - Role=Model: Content (respuesta textual) y/o ToolCalls (invocaciones de tools)
//   - Role=Tool:  Content (resultado de la tool), ToolCallID (match con ToolCall.ID)
type Message struct {
	Role       Role
	Content    string
	Images     []ImageBlob // solo para Role=User; nil o vacío = sin imágenes
	ToolCalls  []ToolCall  // solo para Role=Model; tools invocadas por el modelo
	ToolCallID string      // solo para Role=Tool; debe matchear ToolCall.ID (R11)
}

// ToolCall es una invocación de tool que el modelo decidió hacer.
// El ID es crítico: OpenAI y Anthropic requieren que la respuesta (Role=Tool) incluya
// ToolCallID = este ID. Vertex no usa IDs — los tool results son posicionales,
// el adapter de Vertex ignora el campo.
type ToolCall struct {
	ID   string         // "call_abc123" (OpenAI) o "toolu_abc123" (Anthropic); sintético para Vertex
	Name string         // nombre de la tool (debe coincidir con ToolDef.Name)
	Args map[string]any // argumentos parseados desde JSON
}

// ToolDef describe una tool disponible para el modelo.
// Parameters usa JSON Schema estándar — formato nativo de OpenAI/Anthropic.
// El adapter de Vertex traduce a genai.Schema en tiempo de llamada.
//
// Ejemplo Parameters:
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "alto_cm": {"type": "number", "description": "Alto en centímetros"},
//	    "ancho_cm": {"type": "number"}
//	  },
//	  "required": ["alto_cm", "ancho_cm"]
//	}
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema (R7)
}

// Response es el resultado de una llamada Chat.
// Siempre retorna Content o ToolCalls (o ambos). Nunca ambos vacíos si err==nil.
type Response struct {
	Content      string
	ToolCalls    []ToolCall
	Provider     string // "vertex", "openai", "deepseek", "kimi", "anthropic" — para logging
	Model        string // modelo específico usado (útil cuando llm_model="" y se usa default)
	LatencyMS    int64  // R12: medido por el adapter desde el envío hasta la primera respuesta completa
	TokensIn     int    // R12: tokens de entrada (0 si el proveedor no lo retorna en la response)
	TokensOut    int    // R12: tokens de salida
	TokensCached int    // Tokens leídos del prompt cache del proveedor (0 si no hubo hit).
	// Subconjunto de TokensIn — no se suma aparte al total, se cobra a tarifa reducida.
	// OpenAI: usage.prompt_tokens_details.cached_tokens (50% descuento automático)
	// Anthropic: usage.cache_read_input_tokens (90% descuento, requiere cache_control)
	// Vertex: usage_metadata.cached_content_token_count (implicit gratis; explicit 75% descuento)
}

// Config es la configuración de un cliente LLM — pasa al constructor del adapter.
// APIKey llega ya DESENCRIPTADA desde el factory. El factory es el único que toca
// el ciphertext almacenado en system_config.llm_api_key.
type Config struct {
	Provider string // "vertex"|"openai"|"deepseek"|"kimi"|"anthropic"
	APIKey   string // vacío para vertex (usa ADC); requerido para los demás
	Model    string // vacío = DefaultModels[Provider]
	Endpoint string // base URL — solo relevante para openai/deepseek/kimi; vacío = default
}

// chatOpts contiene los parámetros opcionales por-call configurados vía ChatOption.
// Los adapters aplican solo los campos no-nil al request del proveedor.
type chatOpts struct {
	temperature *float64
	maxTokens   *int
	topP        *float64
}

// ChatOption configura un parámetro opcional de una llamada Chat.
// Se usa para casos como HandleSummary (chat_handler.go) que necesita temperature=0.2
// y maxTokens=700 sin cambiar el comportamiento default de los agentes conversacionales.
type ChatOption func(*chatOpts)

// WithTemperature override la temperatura por-call (default según proveedor/adapter).
func WithTemperature(t float64) ChatOption {
	return func(o *chatOpts) { o.temperature = &t }
}

// WithMaxTokens limita los tokens de salida por-call.
func WithMaxTokens(n int) ChatOption {
	return func(o *chatOpts) { o.maxTokens = &n }
}

// WithTopP override el top_p por-call.
func WithTopP(p float64) ChatOption {
	return func(o *chatOpts) { o.topP = &p }
}

// resolveOpts materializa las opciones funcionales en un chatOpts inicializado.
// Usado internamente por los adapters.
func resolveOpts(opts []ChatOption) *chatOpts {
	o := &chatOpts{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Client es la interface que todos los adapters implementan. Es el único tipo que
// los handlers de FabricaLaser (chat web, whatsapp, admin chat) deben referenciar.
//
// Chat envía una conversación al modelo y recibe su respuesta. Implementa tool calling:
// si el modelo decide invocar tools, la respuesta contiene ToolCalls no vacío y el
// caller debe ejecutar las tools, agregar Role=Tool messages al history y volver a
// llamar Chat hasta que ToolCalls esté vacío (tool loop). Max iterations por convención: 5.
//
// TestConnection verifica que las credenciales funcionan. Se llama en el constructor
// de cada adapter (en buildClient del factory) con timeout interno de 10 segundos —
// independiente del timeout HTTP del request del gestor.
type Client interface {
	// Chat envía system + history + tools al modelo y retorna su respuesta.
	// opts aplica parámetros por-call (temperatura, maxTokens, topP).
	Chat(ctx context.Context, system string, history []Message, tools []ToolDef, opts ...ChatOption) (*Response, error)

	// TestConnection hace un roundtrip mínimo ("respondé solo: ok") para verificar
	// credenciales y conectividad. Usado por Factory en construcción y por
	// POST /api/v1/admin/llm/test (endpoint efímero).
	TestConnection(ctx context.Context) error

	// SupportsImages indica si este adapter puede procesar ImageBlob en Message.Images.
	// Hoy todos los adapters retornan true. Se reserva para proveedores futuros sin
	// capacidad visual (ej. Groq) donde la UI pueda advertir al gestor antes de conmutar.
	SupportsImages() bool

	// Provider retorna el identificador corto del proveedor ("vertex", "openai", etc.).
	// Usado para logging estructurado (R12).
	Provider() string
}

// DefaultModels define el modelo a usar cuando Config.Model está vacío (R8).
// Valores verificados contra listas de proveedores al 2026-04.
// Actualizar con cuidado — un modelo inexistente rompe TestConnection en construcción.
var DefaultModels = map[string]string{
	"vertex":    "gemini-3.5-flash-lite", // 2.5-flash se retira 16-20/10/2026 (migración 2026-09-30)
	"deepseek":  "deepseek-chat",
	"kimi":      "moonshot-v1-8k",
	"openai":    "gpt-4.1-mini", // más capable y económico que gpt-4o-mini
	"anthropic": "claude-haiku-4-5",
}

// ValidProviders lista los proveedores aceptados. Usado por el handler admin para
// validar el dropdown antes de persistir (R13 del plan).
var ValidProviders = []string{"vertex", "openai", "deepseek", "kimi", "anthropic"}

// IsValidProvider retorna true si p está en la whitelist.
func IsValidProvider(p string) bool {
	for _, v := range ValidProviders {
		if v == p {
			return true
		}
	}
	return false
}

// ResolveModel retorna el modelo configurado o el default si está vacío.
// Uso: cada adapter llama llm.ResolveModel(cfg.Provider, cfg.Model) al construir request.
func ResolveModel(provider, configured string) string {
	if configured != "" {
		return configured
	}
	return DefaultModels[provider]
}
