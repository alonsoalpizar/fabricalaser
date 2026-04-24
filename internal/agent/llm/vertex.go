package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"cloud.google.com/go/vertexai/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Constantes fijas para la cuenta GCP del proyecto — alineadas con el adapter
// actual en internal/whatsapp/gemini_adapter.go. El service account del proceso
// provee las credenciales vía ADC, por lo que cfg.APIKey se ignora.
const (
	vertexProjectID = "div-aloalpizar"
	vertexLocation  = "us-central1"

	vertexTestTimeout = 10 * time.Second
)

// vertexAdapter implementa llm.Client contra Vertex AI (Gemini vía
// cloud.google.com/go/vertexai/genai). Se construye vía NewVertexAdapter
// y se reusa: el *genai.Client es seguro para uso concurrente.
type vertexAdapter struct {
	client *genai.Client
	model  string // modelo resuelto (no vacío tras el constructor)
}

// NewVertexAdapter crea un cliente Vertex AI listo para usar. Usa ADC — IGNORA cfg.APIKey.
// Al final del constructor valida la conectividad llamando a TestConnection con timeout
// interno de 10s. Si esa llamada falla, retorna error sin guardar nada.
func NewVertexAdapter(cfg Config) (Client, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, vertexProjectID, vertexLocation)
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to create client: %w", err)
	}

	adapter := &vertexAdapter{
		client: client,
		model:  ResolveModel("vertex", cfg.Model),
	}

	testCtx, cancel := context.WithTimeout(ctx, vertexTestTimeout)
	defer cancel()
	if err := adapter.TestConnection(testCtx); err != nil {
		// No guardamos el cliente si la validación falla.
		_ = client.Close()
		return nil, err
	}

	return adapter, nil
}

// Provider retorna el identificador corto del proveedor.
func (a *vertexAdapter) Provider() string { return "vertex" }

// SupportsImages siempre true: Gemini 2.5 Flash maneja imágenes inline vía genai.Blob.
func (a *vertexAdapter) SupportsImages() bool { return true }

// TestConnection hace un roundtrip mínimo para verificar credenciales y conectividad.
// Aplica un timeout interno de 10s independiente del ctx del caller.
func (a *vertexAdapter) TestConnection(ctx context.Context) error {
	testCtx, cancel := context.WithTimeout(ctx, vertexTestTimeout)
	defer cancel()

	model := a.client.GenerativeModel(a.model)
	model.SetMaxOutputTokens(64) // headroom para que la respuesta sea completa
	model.SetTemperature(0)

	resp, err := model.GenerateContent(testCtx, genai.Text("Say: ok"))
	if err != nil {
		return fmt.Errorf("vertex: test connection failed (model=%s, project=%s): %w", a.model, vertexProjectID, err)
	}
	if len(resp.Candidates) == 0 {
		return fmt.Errorf("vertex: test connection returned 0 candidates (model=%s) — ¿safety filter?", a.model)
	}
	cand := resp.Candidates[0]
	if cand.Content == nil || len(cand.Content.Parts) == 0 {
		return fmt.Errorf("vertex: test connection empty content (model=%s, finish_reason=%v, safety=%v)",
			a.model, cand.FinishReason, cand.SafetyRatings)
	}
	return nil
}

// Chat envía system + history + tools al modelo y retorna Response con métricas.
// El tool loop (hasta 5 iteraciones) NO se ejecuta aquí — el caller es responsable
// de ejecutar tools y re-llamar Chat con mensajes Role=Tool agregados al history.
func (a *vertexAdapter) Chat(
	ctx context.Context,
	system string,
	history []Message,
	tools []ToolDef,
	opts ...ChatOption,
) (*Response, error) {
	o := resolveOpts(opts)

	model := a.client.GenerativeModel(a.model)

	// System prompt
	if system != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(system)},
		}
	}

	// Tools (traducidos de JSON Schema a genai.Schema)
	if len(tools) > 0 {
		decls := make([]*genai.FunctionDeclaration, 0, len(tools))
		for _, t := range tools {
			decls = append(decls, &genai.FunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  jsonSchemaToGenai(t.Parameters),
			})
		}
		model.Tools = []*genai.Tool{{FunctionDeclarations: decls}}
	}

	// Parámetros por-call con defaults del proveedor
	if o.temperature != nil {
		model.SetTemperature(float32(*o.temperature))
	} else {
		model.SetTemperature(0.7)
	}
	if o.maxTokens != nil {
		model.SetMaxOutputTokens(int32(*o.maxTokens))
	} else {
		model.SetMaxOutputTokens(2048)
	}
	if o.topP != nil {
		model.SetTopP(float32(*o.topP))
	} else {
		model.SetTopP(0.95)
	}

	// Traducimos todo el history a []*genai.Content. Para enviar multi-turn
	// con roles correctos al SDK, usamos ChatSession: asignamos los turnos
	// previos a chat.History y enviamos las parts del último turno vía SendMessage.
	contents := messagesToContents(history)

	chat := model.StartChat()
	var lastParts []genai.Part
	if len(contents) == 0 {
		// Edge case: history vacío. Enviamos un mensaje vacío solo para obligar
		// al modelo a responder. En la práctica, el caller siempre agrega al
		// menos un RoleUser antes de llamar Chat.
		lastParts = []genai.Part{genai.Text("")}
	} else {
		last := contents[len(contents)-1]
		lastParts = last.Parts
		chat.History = contents[:len(contents)-1]
	}

	start := time.Now()
	resp, err := sendWithRetryVertex(ctx, chat, lastParts)
	if err != nil {
		return nil, fmt.Errorf("vertex: chat failed: %w", err)
	}

	out := &Response{
		Provider:  "vertex",
		Model:     a.model,
		LatencyMS: time.Since(start).Milliseconds(),
	}

	// Usage metadata (tokens)
	// NOTA sobre prompt caching en Vertex:
	// Gemini 2.5 Flash tiene IMPLICIT CACHING activo automáticamente desde 2024
	// (Google detecta prompts similares en una ventana de ~5 min y aplica descuento
	// transparentemente — el usuario no ve los tokens cacheados en el billing pero
	// paga menos). El campo cached_content_token_count existe en la proto
	// GenerateContentResponse.UsageMetadata, PERO el SDK cloud.google.com/go/vertexai
	// v0.13.2 (deprecated) no lo expone en su veneer UsageMetadata struct.
	//
	// Resultado: TokensCached=0 siempre para Vertex hoy. El beneficio de caching
	// sigue aplicando en el billing de Google, solo no es visible en nuestra
	// observabilidad.
	//
	// Para capturarlo, deuda técnica: migrar al SDK nuevo google.golang.org/genai
	// (ya recomendado por Google, deprecation del v0.13.2 en junio 2026).
	if resp.UsageMetadata != nil {
		out.TokensIn = int(resp.UsageMetadata.PromptTokenCount)
		out.TokensOut = int(resp.UsageMetadata.CandidatesTokenCount)
		// out.TokensCached queda en 0 — no disponible en este SDK
	}

	// Extraer Content + ToolCalls del primer candidate
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		tcIdx := 0
		for _, part := range resp.Candidates[0].Content.Parts {
			switch p := part.(type) {
			case genai.Text:
				out.Content += string(p)
			case genai.FunctionCall:
				out.ToolCalls = append(out.ToolCalls, ToolCall{
					// Vertex no provee ID para function calls — generamos uno
					// sintético estable por índice. El tool loop de Vertex es
					// posicional (R11), por lo que el caller no usa este ID
					// para re-matchear la respuesta.
					ID:   "vertex_tc_" + strconv.Itoa(tcIdx),
					Name: p.Name,
					Args: p.Args,
				})
				tcIdx++
			}
		}
	}

	slog.Info("llm.chat",
		"provider", "vertex",
		"model", out.Model,
		"latency_ms", out.LatencyMS,
		"tokens_in", out.TokensIn,
		"tokens_out", out.TokensOut,
		"tokens_cached", out.TokensCached, // siempre 0 en este SDK (implicit caching activo pero invisible)
		"tool_calls", len(out.ToolCalls),
	)

	return out, nil
}

// messagesToContents traduce []Message → []*genai.Content.
//
//   - RoleUser  → Role="user", Parts con genai.Text(Content) + blobs por cada imagen.
//   - RoleModel → Role="model". Si hay ToolCalls, Parts con genai.FunctionCall;
//     si además hay Content, se agrega genai.Text al final.
//   - RoleTool  → Role="user" (Vertex representa function responses como user-turn)
//     con genai.FunctionResponse. El nombre se toma del ToolCallID como fallback;
//     el tool loop de Vertex es posicional (R11) por lo que no hay match real por ID.
func messagesToContents(history []Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(history))

	for _, m := range history {
		switch m.Role {
		case RoleUser:
			parts := []genai.Part{}
			if m.Content != "" || len(m.Images) == 0 {
				parts = append(parts, genai.Text(m.Content))
			}
			for _, img := range m.Images {
				parts = append(parts, genai.Blob{
					MIMEType: img.MIMEType,
					Data:     img.Data,
				})
			}
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: parts,
			})

		case RoleModel:
			parts := []genai.Part{}
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					parts = append(parts, genai.FunctionCall{
						Name: tc.Name,
						Args: tc.Args,
					})
				}
			}
			if m.Content != "" {
				parts = append(parts, genai.Text(m.Content))
			}
			if len(parts) == 0 {
				// Evitar content vacío — Vertex rechaza turns sin parts.
				parts = append(parts, genai.Text(""))
			}
			contents = append(contents, &genai.Content{
				Role:  "model",
				Parts: parts,
			})

		case RoleTool:
			// Vertex espera la FunctionResponse dentro de un user-turn.
			// El caller debe poner el nombre de la tool en ToolCallID para que
			// Vertex pueda matchear con la FunctionCall previa (Vertex usa el
			// campo Name para matchear, no un ID). Si el caller dejó vacío,
			// Vertex rechazará el turn — documentado en R11.
			response := map[string]any{"result": m.Content}
			contents = append(contents, &genai.Content{
				Role: "user",
				Parts: []genai.Part{
					genai.FunctionResponse{
						Name:     m.ToolCallID,
						Response: response,
					},
				},
			})
		}
	}

	return contents
}

// jsonSchemaToGenai traduce un JSON Schema (map[string]any) a *genai.Schema.
// Preserva description y required; recursa sobre properties e items.
func jsonSchemaToGenai(schema map[string]any) *genai.Schema {
	if schema == nil {
		return nil
	}

	out := &genai.Schema{}

	if t, ok := schema["type"].(string); ok {
		out.Type = mapJSONTypeToGenai(t)
	}
	if desc, ok := schema["description"].(string); ok {
		out.Description = desc
	}

	if props, ok := schema["properties"].(map[string]any); ok && len(props) > 0 {
		out.Properties = make(map[string]*genai.Schema, len(props))
		for name, raw := range props {
			if sub, ok := raw.(map[string]any); ok {
				out.Properties[name] = jsonSchemaToGenai(sub)
			}
		}
	}

	// "required" puede llegar como []string o []any (depende del origen del map)
	if req, ok := schema["required"].([]string); ok {
		out.Required = req
	} else if reqAny, ok := schema["required"].([]any); ok {
		reqStr := make([]string, 0, len(reqAny))
		for _, r := range reqAny {
			if s, ok := r.(string); ok {
				reqStr = append(reqStr, s)
			}
		}
		out.Required = reqStr
	}

	// Schema para arrays — "items" describe el tipo de cada elemento.
	if items, ok := schema["items"].(map[string]any); ok {
		out.Items = jsonSchemaToGenai(items)
	}

	// Enum (string-only): JSON Schema los trae como []any, API admin puede
	// pasarlos como []string.
	if enum, ok := schema["enum"].([]any); ok {
		enumStr := make([]string, 0, len(enum))
		for _, e := range enum {
			if s, ok := e.(string); ok {
				enumStr = append(enumStr, s)
			}
		}
		out.Enum = enumStr
	} else if enum, ok := schema["enum"].([]string); ok {
		out.Enum = enum
	}

	return out
}

// mapJSONTypeToGenai mapea el tipo JSON Schema al tipo genai equivalente.
func mapJSONTypeToGenai(t string) genai.Type {
	switch t {
	case "object":
		return genai.TypeObject
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	default:
		return genai.TypeUnspecified
	}
}

// sendWithRetryVertex envía el mensaje final al chat con retry interno (R13) para
// códigos gRPC transitorios (ResourceExhausted=429, Unavailable=503).
// Backoff exponencial: 500ms, 1s, 2s (hasta 3 intentos tras el primero).
func sendWithRetryVertex(
	ctx context.Context,
	chat *genai.ChatSession,
	parts []genai.Part,
) (*genai.GenerateContentResponse, error) {
	delays := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
	}

	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		resp, err := chat.SendMessage(ctx, parts...)
		if err == nil {
			return resp, nil
		}

		st, ok := status.FromError(err)
		if !ok || (st.Code() != codes.ResourceExhausted && st.Code() != codes.Unavailable) {
			// Error no recuperable — retornar de inmediato.
			return nil, err
		}

		if attempt == len(delays) {
			lastErr = err
			break
		}

		slog.Warn("vertex: transient error — retrying",
			"code", st.Code().String(),
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

	return nil, fmt.Errorf("vertex: retries exhausted: %w", lastErr)
}
