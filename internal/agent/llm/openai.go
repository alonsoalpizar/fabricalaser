package llm

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
	"strconv"
	"time"
)

// openAIAdapter implementa llm.Client para proveedores compatibles con la API
// OpenAI Chat Completions (POST /v1/chat/completions). Cubre:
//   - openai   (api.openai.com)
//   - deepseek (api.deepseek.com)
//   - kimi     (api.moonshot.ai por default — .cn disponible vía cfg.Endpoint override)
//   - groq u otros OpenAI-compat via cfg.Endpoint override
//
// La diferencia entre proveedores es solo la baseURL y el modelo por default —
// el request/response shape es idéntico.
type openAIAdapter struct {
	cfg     Config
	baseURL string
	model   string
	http    *http.Client
}

// ---------- request/response wire types ----------

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type openAIToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openAIFuncCall `json:"function"`
}

type openAIFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIToolFunc `json:"function"`
}

type openAIToolFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// ---------- constructor ----------

// NewOpenAICompatAdapter construye un adapter para cualquier proveedor OpenAI-compat.
// REQUIERE cfg.APIKey. Resuelve baseURL según cfg.Provider y cfg.Endpoint (override).
// Al final llama TestConnection con timeout de 10s; si falla retorna error.
func NewOpenAICompatAdapter(cfg Config) (Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("openai: APIKey required")
	}

	baseURL := cfg.Endpoint
	if baseURL == "" {
		switch cfg.Provider {
		case "openai":
			baseURL = "https://api.openai.com"
		case "deepseek":
			baseURL = "https://api.deepseek.com"
		case "kimi":
			// Moonshot tiene dos plataformas: api.moonshot.cn (China) y api.moonshot.ai (international).
			// Las keys NO son intercambiables — se validan en la plataforma de registro.
			// Default international porque las keys desde LATAM se emiten ahí. Gestor puede
			// override a .cn si tiene key china.
			baseURL = "https://api.moonshot.ai"
		default:
			return nil, fmt.Errorf("openai: provider %q requires explicit Endpoint", cfg.Provider)
		}
	}

	a := &openAIAdapter{
		cfg:     cfg,
		baseURL: baseURL,
		model:   ResolveModel(cfg.Provider, cfg.Model),
		http: &http.Client{
			Timeout: 120 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("openai: test connection failed: %w", err)
	}

	return a, nil
}

// ---------- Chat ----------

// Chat envía system + history + tools al endpoint /v1/chat/completions y retorna
// la respuesta del modelo.
//
// Retry (R13):
//   - HTTP 429 Too Many Requests: respeta header Retry-After (segundos); si no
//     está presente, backoff exponencial 1s, 2s, 4s hasta 3 intentos.
//   - HTTP 5xx: un solo retry con 1s de espera.
//   - Otros errores de red: propagados al caller.
func (a *openAIAdapter) Chat(ctx context.Context, system string, history []Message, tools []ToolDef, opts ...ChatOption) (*Response, error) {
	o := resolveOpts(opts)

	// Construir mensajes: system primero, luego history traducido.
	messages := make([]openAIMessage, 0, len(history)+1)
	if system != "" {
		messages = append(messages, openAIMessage{
			Role:    "system",
			Content: system,
		})
	}

	for _, msg := range history {
		tm, err := translateMessage(msg)
		if err != nil {
			return nil, err
		}
		messages = append(messages, tm)
	}

	// Traducir tools: Parameters (JSON Schema) va directo, sin transformación.
	var apiTools []openAITool
	if len(tools) > 0 {
		apiTools = make([]openAITool, 0, len(tools))
		for _, t := range tools {
			apiTools = append(apiTools, openAITool{
				Type: "function",
				Function: openAIToolFunc{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}

	// Aplicar defaults / overrides por-call.
	temperature := 0.7
	maxTokens := 2048
	topP := 0.95
	if o.temperature != nil {
		temperature = *o.temperature
	}
	if o.maxTokens != nil {
		maxTokens = *o.maxTokens
	}
	if o.topP != nil {
		topP = *o.topP
	}

	req := openAIRequest{
		Model:       a.model,
		Messages:    messages,
		Tools:       apiTools,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		TopP:        &topP,
	}

	start := time.Now()
	parsed, err := a.doRequestWithRetry(ctx, req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}

	if parsed.Error != nil {
		return nil, fmt.Errorf("openai: api error: %s (%s)", parsed.Error.Message, parsed.Error.Code)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("openai: response has no choices")
	}

	choice := parsed.Choices[0].Message

	// Parsear tool_calls preservando el ID original (R11).
	var toolCalls []ToolCall
	if len(choice.ToolCalls) > 0 {
		toolCalls = make([]ToolCall, 0, len(choice.ToolCalls))
		for _, tc := range choice.ToolCalls {
			args := map[string]any{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return nil, fmt.Errorf("openai: tool_call %s arguments parse failed: %w", tc.ID, err)
				}
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			})
		}
	}

	resp := &Response{
		Content:   choice.Content,
		ToolCalls: toolCalls,
		Provider:  a.cfg.Provider,
		Model:     a.model,
		LatencyMS: latency,
		TokensIn:  parsed.Usage.PromptTokens,
		TokensOut: parsed.Usage.CompletionTokens,
	}

	// R12: logging estructurado.
	slog.Info("llm.chat",
		"provider", a.cfg.Provider,
		"model", resp.Model,
		"latency_ms", resp.LatencyMS,
		"tokens_in", resp.TokensIn,
		"tokens_out", resp.TokensOut,
		"tool_calls", len(resp.ToolCalls),
	)

	return resp, nil
}

// translateMessage convierte un llm.Message al shape OpenAI. Role translation:
//   - RoleUser  → "user"   (con soporte multimodal si Images no vacío)
//   - RoleModel → "assistant" (con tool_calls si Message.ToolCalls no vacío)
//   - RoleTool  → "tool" (REQUIERE ToolCallID — R11)
func translateMessage(msg Message) (openAIMessage, error) {
	switch msg.Role {
	case RoleUser:
		out := openAIMessage{Role: "user"}
		if len(msg.Images) == 0 {
			out.Content = msg.Content
			return out, nil
		}
		// Multimodal: content es un array de parts.
		parts := make([]any, 0, len(msg.Images)+1)
		parts = append(parts, map[string]any{
			"type": "text",
			"text": msg.Content,
		})
		for _, img := range msg.Images {
			parts = append(parts, map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": fmt.Sprintf("data:%s;base64,%s",
						img.MIMEType,
						base64.StdEncoding.EncodeToString(img.Data)),
				},
			})
		}
		out.Content = parts
		return out, nil

	case RoleModel:
		out := openAIMessage{
			Role:    "assistant",
			Content: msg.Content,
		}
		if len(msg.ToolCalls) > 0 {
			calls := make([]openAIToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				argsBytes, err := json.Marshal(tc.Args)
				if err != nil {
					return openAIMessage{}, fmt.Errorf("openai: marshal tool_call %s args: %w", tc.ID, err)
				}
				calls = append(calls, openAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openAIFuncCall{
						Name:      tc.Name,
						Arguments: string(argsBytes),
					},
				})
			}
			out.ToolCalls = calls
		}
		return out, nil

	case RoleTool:
		if msg.ToolCallID == "" {
			return openAIMessage{}, errors.New("openai: tool message requires ToolCallID")
		}
		return openAIMessage{
			Role:       "tool",
			ToolCallID: msg.ToolCallID,
			Content:    msg.Content,
		}, nil

	default:
		return openAIMessage{}, fmt.Errorf("openai: unknown role %q", msg.Role)
	}
}

// doRequestWithRetry envía el request y maneja 429 (con Retry-After o backoff
// exponencial hasta 3 intentos) y 5xx (un solo retry con 1s).
func (a *openAIAdapter) doRequestWithRetry(ctx context.Context, req openAIRequest) (*openAIResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	url := a.baseURL + "/v1/chat/completions"

	var (
		parsed     *openAIResponse
		lastErr    error
		attempt429 = 0
		retried5xx = false
	)

	for {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("openai: build request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := a.http.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("openai: http do: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("openai: read response: %w", readErr)
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			if attempt429 >= 3 {
				return nil, fmt.Errorf("openai: 429 rate limit after %d retries: %s", attempt429, string(respBody))
			}
			wait := retryAfterOrBackoff(resp.Header.Get("Retry-After"), attempt429)
			attempt429++
			lastErr = fmt.Errorf("openai: 429 rate limit, retry %d after %s", attempt429, wait)
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, err
			}
			continue

		case resp.StatusCode >= 500 && resp.StatusCode < 600:
			if retried5xx {
				return nil, fmt.Errorf("openai: 5xx status %d after retry: %s", resp.StatusCode, string(respBody))
			}
			retried5xx = true
			lastErr = fmt.Errorf("openai: 5xx status %d, retrying once", resp.StatusCode)
			if err := sleepCtx(ctx, time.Second); err != nil {
				return nil, err
			}
			continue

		case resp.StatusCode >= 400:
			return nil, fmt.Errorf("openai: http %d: %s", resp.StatusCode, string(respBody))
		}

		parsed = &openAIResponse{}
		if err := json.Unmarshal(respBody, parsed); err != nil {
			return nil, fmt.Errorf("openai: decode response: %w (body: %s)", err, string(respBody))
		}
		break
	}

	_ = lastErr // último error informativo, solo retorna si se exceden intentos
	return parsed, nil
}

// retryAfterOrBackoff parsea Retry-After (segundos) o retorna backoff exponencial
// 1s, 2s, 4s según el número de intento (0-indexed).
func retryAfterOrBackoff(header string, attempt int) time.Duration {
	if header != "" {
		if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	// 1s, 2s, 4s
	return time.Duration(1<<attempt) * time.Second
}

// sleepCtx espera d o cancela si el contexto expira.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ---------- TestConnection ----------

// TestConnection hace un roundtrip mínimo para validar credenciales y conectividad.
// Usa un timeout local de 10 segundos independiente del contexto entrante.
func (a *openAIAdapter) TestConnection(ctx context.Context) error {
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	maxTokens := 10
	temp := 0.0
	req := openAIRequest{
		Model: a.model,
		Messages: []openAIMessage{
			{Role: "user", Content: "di: ok"},
		},
		MaxTokens:   &maxTokens,
		Temperature: &temp,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("openai: test marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(tctx, http.MethodPost, a.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("openai: test build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("openai: test http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("openai: test http %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("openai: test decode: %w", err)
	}
	if parsed.Error != nil {
		return fmt.Errorf("openai: test api error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return errors.New("openai: test returned empty content")
	}
	return nil
}

// ---------- Capabilities ----------

// SupportsImages indica que el adapter puede procesar ImageBlob (via content parts).
// Es true aun para proveedores sin soporte visual real — la matriz de capacidades
// por modelo específico es responsabilidad del factory/admin.
func (a *openAIAdapter) SupportsImages() bool {
	return true
}

// Provider retorna el identificador efectivo del proveedor (no hardcodeado).
func (a *openAIAdapter) Provider() string {
	return a.cfg.Provider
}
