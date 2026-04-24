// Package llm (admin) expone los endpoints del panel admin para configurar el proveedor
// LLM activo (chat web, WhatsApp/Telegram, admin chat).
//
// Endpoints (todos bajo /api/v1/admin/llm/, con AuthMiddleware + RoleMiddleware("admin")):
//   GET  /config → {provider, model, endpoint, api_key_set: bool, supports_images: bool}
//   POST /config → body: {provider, api_key, model, endpoint} → {ok, reloaded_at, provider, model}
//   POST /test   → body: {provider, api_key, model, endpoint} → {ok, latency_ms, error?}
//
// Reglas aplicadas:
//   R2: POST /config solo persiste si el Reload es exitoso. Si falla, retorna 422 y DB no cambia.
//   R3: GET /config NUNCA retorna el api_key — solo api_key_set: bool.
//   R4b: POST /test usa cliente efímero (factory.TestEphemeral) — no toca DB ni factory activa.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	agentllm "github.com/alonsoalpizar/fabricalaser/internal/agent/llm"
)

// Handler agrupa las dependencias para los 3 endpoints.
type Handler struct {
	factory *agentllm.Factory
}

// NewHandler construye el handler. El factory se crea una sola vez en router.go
// y se comparte con todos los agentes LLM (chat web, WA, admin chat).
func NewHandler(factory *agentllm.Factory) *Handler {
	return &Handler{factory: factory}
}

// =============================================================================
// GET /config
// =============================================================================

type getConfigResponse struct {
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Endpoint       string `json:"endpoint"`
	APIKeySet      bool   `json:"api_key_set"`
	SupportsImages bool   `json:"supports_images"`
}

// GetConfig retorna la configuración actual sin exponer el api_key (R3).
// api_key_set indica al frontend si debe mostrar "••••••••" (con botón Reemplazar)
// o un campo vacío para ingresar una key por primera vez.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.factory.CurrentConfig()
	client := h.factory.Client()

	resp := getConfigResponse{
		Provider:       cfg.Provider,
		Model:          cfg.Model,
		Endpoint:       cfg.Endpoint,
		APIKeySet:      h.factory.HasAPIKey(),
		SupportsImages: client.SupportsImages(),
	}

	writeJSON(w, http.StatusOK, resp)
}

// =============================================================================
// POST /config
// =============================================================================

type saveConfigRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
}

type saveConfigResponse struct {
	OK         bool      `json:"ok"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	ReloadedAt time.Time `json:"reloaded_at"`
}

// SaveConfig persiste nueva configuración y dispara hot reload atómico.
// Orden estricto (R2):
//  0. Validar provider en whitelist (lo hace factory.Reload internamente)
//  1. Si provider != vertex && APIKey vacío && DB ya tiene key → usar la existente
//     (gestor cambió solo model o endpoint, no quiere re-tipear la key)
//  2. factory.Reload(cfg) → construye cliente, prueba, si éxito: persiste DB y swap
//  3. Si falla cualquier paso → 422 con error técnico, DB no cambia, cliente anterior sigue
func (h *Handler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req saveConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido: "+err.Error())
		return
	}

	// Si el gestor cambia solo model/endpoint y no tipea key, usar la existente.
	// Esto evita que tenga que reingresar la key cada vez que cambia el modelo.
	if req.APIKey == "" && req.Provider != "vertex" {
		currentCfg := h.factory.CurrentConfig()
		// CurrentConfig retorna APIKey vaciada (R3). Debemos obtener la real.
		// El factory NO expone la api_key plain (R3), pero sí tiene HasAPIKey.
		// La solución correcta: si el provider que guardamos coincide con el actual
		// Y había api_key configurada, no hace falta re-enviarla — el factory ya la tiene.
		// Pero si el provider cambia, necesitamos la nueva key.
		if req.Provider != currentCfg.Provider {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("provider %q requiere api_key", req.Provider))
			return
		}
		if !h.factory.HasAPIKey() {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("provider %q requiere api_key (DB no tiene una guardada)", req.Provider))
			return
		}
		// Caso: mismo provider, sin nueva key — reloadeamos usando la config actual
		// pero con model/endpoint nuevos. Necesitamos la key actual. El factory
		// la tiene internamente pero no la expone. Solución: crear método Factory.ReloadWithCurrentKey.
		//
		// Por ahora, si no cambia el provider y hay key configurada, reusamos la key
		// llamando a un Reload interno que conserve la api_key. Lo expongo vía
		// factory.ReloadKeepKey(cfg) — agrego ese método al factory.
		if err := h.factory.ReloadKeepKey(agentllm.Config{
			Provider: req.Provider,
			Model:    req.Model,
			Endpoint: req.Endpoint,
		}); err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		cfg := h.factory.CurrentConfig()
		writeJSON(w, http.StatusOK, saveConfigResponse{
			OK:         true,
			Provider:   cfg.Provider,
			Model:      cfg.Model,
			ReloadedAt: time.Now().UTC(),
		})
		return
	}

	// Caso normal: Reload con credenciales completas
	cfg := agentllm.Config{
		Provider: req.Provider,
		APIKey:   req.APIKey,
		Model:    req.Model,
		Endpoint: req.Endpoint,
	}

	if err := h.factory.Reload(cfg); err != nil {
		// Reload falló — 422 Unprocessable Entity, DB no cambió (R2)
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, saveConfigResponse{
		OK:         true,
		Provider:   cfg.Provider,
		Model:      cfg.Model,
		ReloadedAt: time.Now().UTC(),
	})
}

// =============================================================================
// POST /test
// =============================================================================

type testRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
}

type testResponse struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// TestConnection (R4b) construye un cliente EFÍMERO con las credenciales del body,
// ejecuta TestConnection, descarta el cliente. No toca DB ni factory activa.
// Permite al gestor validar credenciales antes de guardar.
//
// Si no se pasa api_key pero sí provider y el gestor quiere probar la config
// actualmente guardada, puede hacer GET /config y luego POST /test con esa data
// (sin key, ya está guardada encriptada — el endpoint rechaza porque no puede
// decriptar aquí). Para testear la config guardada, alternativa: POST /test sin body
// se interpreta como "probar la config activa" — implementado abajo.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req testRequest
	// Si el body es vacío, probamos el cliente activo
	if r.ContentLength == 0 {
		h.testActiveClient(w, r)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido: "+err.Error())
		return
	}

	// Si el body está vacío tras decodificar (ej. "{}"), también probamos el activo
	if req.Provider == "" {
		h.testActiveClient(w, r)
		return
	}

	cfg := agentllm.Config{
		Provider: req.Provider,
		APIKey:   req.APIKey,
		Model:    req.Model,
		Endpoint: req.Endpoint,
	}

	// Timeout total generoso — TestEphemeral internamente usa 10s por TestConnection,
	// aquí agregamos headroom para construcción del cliente.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	latencyMS, err := h.factory.TestEphemeral(ctx, cfg)
	if err != nil {
		writeJSON(w, http.StatusOK, testResponse{
			OK:        false,
			LatencyMS: latencyMS,
			Error:     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, testResponse{
		OK:        true,
		LatencyMS: latencyMS,
	})
}

// testActiveClient prueba el cliente actualmente activo en el factory.
// Útil cuando el gestor quiere verificar que el sistema sigue funcionando
// sin cambiar credenciales.
func (h *Handler) testActiveClient(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	start := time.Now()
	client := h.factory.Client()
	if client == nil {
		writeJSON(w, http.StatusOK, testResponse{
			OK:    false,
			Error: "no hay cliente activo en el factory",
		})
		return
	}

	if err := client.TestConnection(ctx); err != nil {
		writeJSON(w, http.StatusOK, testResponse{
			OK:        false,
			LatencyMS: time.Since(start).Milliseconds(),
			Error:     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, testResponse{
		OK:        true,
		LatencyMS: time.Since(start).Milliseconds(),
	})
}

// =============================================================================
// Helpers
// =============================================================================

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// último recurso — ya enviamos headers
		_ = err
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": msg,
	})
}

