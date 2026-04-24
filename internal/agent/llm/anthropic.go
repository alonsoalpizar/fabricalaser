package llm

// Adapter para Anthropic Messages API (/v1/messages).
// Usa net/http stdlib (sin SDK oficial) porque /v1/messages es un solo POST JSON
// sin streaming ni lógica cliente compleja. Menos dependencias = menos superficie.
//
// Referencia: https://docs.anthropic.com/en/api/messages
//
// Retry interno (R13):
//   - HTTP 429: respeta header retry-after (segundos). Backoff 1s, 2s, 4s hasta 3 intentos.
//   - HTTP 529 (overloaded): mismo retry que 429.
//   - HTTP 5xx: retry una vez con 1s.
//   - Otros 4xx: no retry, error inmediato con body.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
	defaultMaxTokens  = 2048
	defaultTempAnth   = 0.7
	defaultTopPAnth   = 0.95
)

// anthropicAdapter implementa llm.Client contra /v1/messages.
type anthropicAdapter struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewAnthropicAdapter construye un adapter Anthropic.
// Requiere cfg.APIKey no vacío. cfg.Endpoint se ignora (endpoint fijo).
// Llama TestConnection con timeout 10s al final — si falla, retorna error.
func NewAnthropicAdapter(cfg Config) (Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("anthropic: APIKey required")
	}
	a := &anthropicAdapter{
		apiKey: cfg.APIKey,
		model:  ResolveModel("anthropic", cfg.Model),
		http:   &http.Client{Timeout: 60 * time.Second},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("anthropic: test connection failed: %w", err)
	}
	return a, nil
}

// ---------- Wire types ----------

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"` // required
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
}

// anthropicMessage.Content puede ser string o []contentBlock — por eso any.
type anthropicMessage struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// contentBlock cubre todos los tipos: text, image, tool_use, tool_result.
// Campos opcionales se omiten con omitempty.
type contentBlock struct {
	Type string `json:"type"`
	// text
	Text string `json:"text,omitempty"`
	// image
	Source *imageSource `json:"source,omitempty"`
	// tool_use
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
	// tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type imageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/jpeg" etc
	Data      string `json:"data"`       // base64
}

type anthropicResponse struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Role    string                  `json:"role"`
	Model   string                  `json:"model"`
	Content []anthropicResponseBlok `json:"content"`
	Usage   anthropicUsage          `json:"usage"`
}

type anthropicResponseBlok struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicErrorEnvelope struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ---------- Interface methods ----------

func (a *anthropicAdapter) Provider() string    { return "anthropic" }
func (a *anthropicAdapter) SupportsImages() bool { return true }

// Chat envía la conversación al modelo y retorna Response.
func (a *anthropicAdapter) Chat(
	ctx context.Context,
	system string,
	history []Message,
	tools []ToolDef,
	opts ...ChatOption,
) (*Response, error) {
	o := resolveOpts(opts)

	msgs, err := translateHistory(history)
	if err != nil {
		return nil, err
	}

	reqBody := anthropicRequest{
		Model:    a.model,
		System:   system,
		Messages: msgs,
		Tools:    translateTools(tools),
	}

	// MaxTokens required.
	if o.maxTokens != nil {
		reqBody.MaxTokens = *o.maxTokens
	} else {
		reqBody.MaxTokens = defaultMaxTokens
	}

	// Temperature / TopP con defaults.
	if o.temperature != nil {
		reqBody.Temperature = o.temperature
	} else {
		t := defaultTempAnth
		reqBody.Temperature = &t
	}
	if o.topP != nil {
		reqBody.TopP = o.topP
	} else {
		p := defaultTopPAnth
		reqBody.TopP = &p
	}

	start := time.Now()
	raw, err := a.doRequest(ctx, &reqBody)
	if err != nil {
		return nil, err
	}
	latency := time.Since(start).Milliseconds()

	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	resp := &Response{
		Provider:  "anthropic",
		Model:     a.model,
		LatencyMS: latency,
		TokensIn:  parsed.Usage.InputTokens,
		TokensOut: parsed.Usage.OutputTokens,
	}

	// Concat text blocks; collect tool_use blocks preservando ID original (R11).
	var textBuf bytes.Buffer
	for _, blk := range parsed.Content {
		switch blk.Type {
		case "text":
			textBuf.WriteString(blk.Text)
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:   blk.ID,
				Name: blk.Name,
				Args: blk.Input,
			})
		}
	}
	resp.Content = textBuf.String()

	// Logging R12.
	slog.Info("llm.chat",
		"provider", "anthropic",
		"model", resp.Model,
		"latency_ms", resp.LatencyMS,
		"tokens_in", resp.TokensIn,
		"tokens_out", resp.TokensOut,
		"tool_calls", len(resp.ToolCalls),
	)

	return resp, nil
}

// TestConnection envía un mensaje mínimo para validar credenciales.
func (a *anthropicAdapter) TestConnection(ctx context.Context) error {
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqBody := anthropicRequest{
		Model:     a.model,
		MaxTokens: 10,
		Messages: []anthropicMessage{
			{Role: "user", Content: "di: ok"},
		},
	}

	raw, err := a.doRequest(tctx, &reqBody)
	if err != nil {
		return err
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("anthropic: test decode: %w", err)
	}
	if len(parsed.Content) == 0 || parsed.Content[0].Type != "text" || parsed.Content[0].Text == "" {
		return errors.New("anthropic: test response empty")
	}
	return nil
}

// ---------- Helpers ----------

// translateHistory convierte []llm.Message al formato Anthropic.
// Anthropic sólo acepta roles "user" y "assistant":
//   - RoleUser   -> "user"
//   - RoleModel  -> "assistant"
//   - RoleTool   -> "user" con content block type="tool_result"
func translateHistory(history []Message) ([]anthropicMessage, error) {
	out := make([]anthropicMessage, 0, len(history))
	for _, m := range history {
		switch m.Role {
		case RoleUser:
			// Si hay imágenes o es respuesta a tool (no debería pasar con RoleUser pero cuidamos)
			// usamos content blocks. Si solo texto plano -> string.
			if len(m.Images) > 0 {
				blocks := make([]contentBlock, 0, len(m.Images)+1)
				for _, img := range m.Images {
					blocks = append(blocks, contentBlock{
						Type: "image",
						Source: &imageSource{
							Type:      "base64",
							MediaType: img.MIMEType,
							Data:      base64.StdEncoding.EncodeToString(img.Data),
						},
					})
				}
				if m.Content != "" {
					blocks = append(blocks, contentBlock{Type: "text", Text: m.Content})
				}
				out = append(out, anthropicMessage{Role: "user", Content: blocks})
			} else {
				out = append(out, anthropicMessage{Role: "user", Content: m.Content})
			}

		case RoleModel:
			// Assistant: puede llevar texto y/o tool_use blocks.
			if len(m.ToolCalls) > 0 {
				blocks := make([]contentBlock, 0, len(m.ToolCalls)+1)
				if m.Content != "" {
					blocks = append(blocks, contentBlock{Type: "text", Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					blocks = append(blocks, contentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: tc.Args,
					})
				}
				out = append(out, anthropicMessage{Role: "assistant", Content: blocks})
			} else {
				out = append(out, anthropicMessage{Role: "assistant", Content: m.Content})
			}

		case RoleTool:
			// Tool result va como mensaje de "user" con content block tool_result.
			if m.ToolCallID == "" {
				return nil, errors.New("anthropic: tool message requires ToolCallID")
			}
			block := contentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			}
			out = append(out, anthropicMessage{
				Role:    "user",
				Content: []contentBlock{block},
			})

		default:
			return nil, fmt.Errorf("anthropic: unknown role %q", m.Role)
		}
	}
	return out, nil
}

// translateTools convierte []llm.ToolDef al formato Anthropic. JSON Schema es nativo.
func translateTools(tools []ToolDef) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	return out
}

// doRequest ejecuta el POST con retry según R13.
// - 429/529: backoff 1s, 2s, 4s hasta 3 intentos (respeta retry-after si viene).
// - 5xx: un retry adicional con 1s.
// - 2xx: devuelve body.
// - Otros: error inmediato con status y body.
func (a *anthropicAdapter) doRequest(ctx context.Context, req *anthropicRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	const maxRateLimitAttempts = 3
	rateLimitAttempt := 0
	serverErrorRetried := false

	for {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("anthropic: build request: %w", err)
		}
		httpReq.Header.Set("x-api-key", a.apiKey)
		httpReq.Header.Set("anthropic-version", anthropicVersion)
		httpReq.Header.Set("content-type", "application/json")

		resp, err := a.http.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("anthropic: http error: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("anthropic: read body: %w", readErr)
		}

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return respBody, nil

		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 529:
			if rateLimitAttempt >= maxRateLimitAttempts {
				return nil, anthropicHTTPError(resp.StatusCode, respBody)
			}
			wait := retryAfterOrBackoff(resp.Header.Get("retry-after"), rateLimitAttempt)
			rateLimitAttempt++
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, err
			}

		case resp.StatusCode >= 500 && resp.StatusCode < 600:
			if serverErrorRetried {
				return nil, anthropicHTTPError(resp.StatusCode, respBody)
			}
			serverErrorRetried = true
			if err := sleepCtx(ctx, 1*time.Second); err != nil {
				return nil, err
			}

		default:
			return nil, anthropicHTTPError(resp.StatusCode, respBody)
		}
	}
}

// anthropicHTTPError formatea un error HTTP intentando extraer el mensaje de la API.
func anthropicHTTPError(status int, body []byte) error {
	var env anthropicErrorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		return fmt.Errorf("anthropic: http %d: %s (%s)", status, env.Error.Message, env.Error.Type)
	}
	return fmt.Errorf("anthropic: http %d: %s", status, string(body))
}
