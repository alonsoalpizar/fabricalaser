package llm

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
)

// Factory orquesta la vida de un único llm.Client activo para toda la aplicación.
// Lee la configuración desde system_config al arrancar, desencripta la API key,
// construye el adapter correspondiente y expone Reload() para conmutar de proveedor
// en tiempo real sin reiniciar el servicio.
//
// Reglas aplicadas:
//   - R2: Reload nunca deja el sistema sin cliente. Si buildClient falla, f.current no cambia.
//   - R3: API key encriptada AES-256-GCM antes de persistir; nunca retornada en GET /config.
//   - R6: Requests en vuelo completan normalmente con el cliente que obtuvieron vía Client().
//     El swap atómico aplica solo a llamadas posteriores.
type Factory struct {
	mu      sync.RWMutex
	current Client
	config  Config
	sysRepo *repository.SystemConfigRepository
	secret  []byte // sha256(APP_SECRET) — 32 bytes exactos para AES-256
}

// Keys en system_config — constantes centralizadas para evitar typos
const (
	KeyProvider = "llm_provider"
	KeyAPIKey   = "llm_api_key"
	KeyModel    = "llm_model"
	KeyEndpoint = "llm_endpoint"
)

// NewFactory construye el factory. Orden estricto:
//  1. Derivar clave de encriptación: sha256(appSecret) → 32 bytes
//  2. Cargar 4 claves desde system_config (llm_provider, llm_api_key, llm_model, llm_endpoint)
//  3. Si llm_provider está vacío, default "vertex" (seguro — usa ADC)
//  4. Desencriptar llm_api_key si no está vacío
//  5. buildClient(cfg) → adapter con TestConnection interno de 10s
//  6. Si buildClient falla, retornar error — el servicio NO debe levantar
//
// Rotar APP_SECRET invalida todas las API keys encriptadas. El gestor debe reingresar
// credenciales desde la UI tras rotación.
func NewFactory(sysRepo *repository.SystemConfigRepository, appSecret string) (*Factory, error) {
	if appSecret == "" {
		return nil, errors.New("llm.NewFactory: APP_SECRET requerido para encriptar credenciales")
	}

	// sha256 produce exactamente 32 bytes — requerido por AES-256
	secretHash := sha256.Sum256([]byte(appSecret))

	f := &Factory{
		sysRepo: sysRepo,
		secret:  secretHash[:],
	}

	cfg, err := f.loadConfigFromDB()
	if err != nil {
		return nil, fmt.Errorf("llm.NewFactory: cargando config desde DB: %w", err)
	}

	client, err := buildClient(cfg)
	if err != nil {
		// Fallback no-catastrófico: si el cliente inicial falla (ej. Vertex ADC mal
		// configurado, API key expirada), arrancamos con un stub que retorna error
		// descriptivo en cada llamada. El gestor puede entonces conmutar a otro
		// proveedor desde /admin/configuracion-llm.html sin tocar el servidor.
		slog.Error("llm.factory: cliente inicial falló, arrancando con stub — admin debe reconfigurar desde UI",
			"provider", cfg.Provider,
			"error", err,
		)
		f.current = newStubClient(cfg.Provider, err)
		f.config = cfg
		return f, nil
	}

	f.current = client
	f.config = cfg

	slog.Info("llm.factory initialized",
		"provider", cfg.Provider,
		"model", ResolveModel(cfg.Provider, cfg.Model),
		"has_api_key", cfg.APIKey != "",
	)

	return f, nil
}

// Client retorna el cliente activo. Thread-safe con RLock — requests en vuelo
// obtienen el mismo client y completan aunque ocurra un Reload simultáneo (R6).
func (f *Factory) Client() Client {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.current
}

// CurrentConfig retorna una copia de la configuración actual con APIKey vaciada.
// Usado por GET /api/v1/admin/llm/config — nunca exponer la API key (R3).
func (f *Factory) CurrentConfig() Config {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return Config{
		Provider: f.config.Provider,
		APIKey:   "", // nunca se expone
		Model:    f.config.Model,
		Endpoint: f.config.Endpoint,
	}
}

// HasAPIKey retorna si hay una API key configurada (sin exponerla).
// Usado por GET /api/v1/admin/llm/config para el flag api_key_set.
func (f *Factory) HasAPIKey() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config.APIKey != ""
}

// Reload construye un nuevo cliente con cfg, persiste en DB y hace swap atómico.
// Orden estricto (R2 — nunca deja sin cliente):
//  1. Validar cfg.Provider en whitelist
//  2. Si provider != vertex && APIKey vacío → error (ADC solo para vertex)
//  3. buildClient(cfg) — esto llama TestConnection interno de 10s
//  4. Si falla → retorna error, f.current NO cambia, DB NO se toca
//  5. Si éxito → encriptar APIKey, persistir las 4 claves en DB
//  6. Si persistencia falla → cerrar el cliente nuevo, retornar error, f.current NO cambia
//  7. Si todo OK → Lock, swap f.current y f.config, Unlock
//
// Fallos en DB después de que el nuevo cliente funciona: se reporta el error pero
// el cliente ANTERIOR sigue activo. El gestor debe reintentar — la inconsistencia
// momentánea es DB con config vieja + memoria con cliente viejo, coherente.
func (f *Factory) Reload(cfg Config) error {
	if !IsValidProvider(cfg.Provider) {
		return fmt.Errorf("llm.Reload: provider inválido %q (válidos: %v)", cfg.Provider, ValidProviders)
	}

	if cfg.Provider != "vertex" && cfg.APIKey == "" {
		return fmt.Errorf("llm.Reload: provider %q requiere API key (solo vertex usa ADC)", cfg.Provider)
	}

	// Paso 3: construir e implícitamente probar conexión (buildClient → TestConnection interno)
	newClient, err := buildClient(cfg)
	if err != nil {
		return fmt.Errorf("llm.Reload: construcción falló para %s: %w", cfg.Provider, err)
	}

	// Paso 5: encriptar API key antes de persistir
	var apiKeyCipher string
	if cfg.APIKey != "" {
		apiKeyCipher, err = f.encryptKey(cfg.APIKey)
		if err != nil {
			// No debería ocurrir — sha256(appSecret) siempre da clave válida
			return fmt.Errorf("llm.Reload: encriptando API key: %w", err)
		}
	}

	// Paso 5: persistir las 4 claves
	if err := f.persistConfig(cfg.Provider, apiKeyCipher, cfg.Model, cfg.Endpoint); err != nil {
		return fmt.Errorf("llm.Reload: persistiendo config en DB (cliente anterior sigue activo): %w", err)
	}

	// Paso 7: swap atómico
	f.mu.Lock()
	old := f.current
	f.current = newClient
	f.config = cfg
	f.mu.Unlock()

	// Cerrar el cliente viejo si implementa io.Closer (ej. Vertex gRPC)
	// Fuera del Lock para no bloquear lectores.
	if closer, ok := old.(io.Closer); ok {
		if cerr := closer.Close(); cerr != nil {
			slog.Warn("llm.Reload: error cerrando cliente anterior (no bloqueante)", "error", cerr)
		}
	}

	slog.Info("llm.factory reloaded",
		"provider", cfg.Provider,
		"model", ResolveModel(cfg.Provider, cfg.Model),
	)

	return nil
}

// ReloadKeepKey recarga con provider/model/endpoint nuevos pero preservando la
// API key actualmente en memoria. Útil cuando el gestor cambia solo el modelo
// (ej. de deepseek-chat a deepseek-reasoner) sin querer reingresar la api_key.
//
// PRECONDICIÓN: cfg.Provider debe coincidir con el provider actual. Si es distinto,
// las credenciales del provider anterior no aplican — retorna error pidiendo api_key.
func (f *Factory) ReloadKeepKey(cfg Config) error {
	f.mu.RLock()
	currentAPIKey := f.config.APIKey
	currentProvider := f.config.Provider
	f.mu.RUnlock()

	if cfg.Provider != currentProvider {
		return fmt.Errorf("ReloadKeepKey: provider cambió de %q a %q — se requiere nueva api_key", currentProvider, cfg.Provider)
	}

	cfg.APIKey = currentAPIKey
	return f.Reload(cfg)
}

// TestEphemeral construye un cliente temporal con cfg, lo prueba, lo descarta.
// NO toca f.current ni la DB. Usado por POST /api/v1/admin/llm/test (R4b) para
// que el gestor pueda validar credenciales antes de guardar.
//
// Timeout interno: usa el TestConnection que cada adapter encapsula (10s).
// Retorna latencia en ms y error si falla.
func (f *Factory) TestEphemeral(ctx context.Context, cfg Config) (latencyMS int64, err error) {
	if !IsValidProvider(cfg.Provider) {
		return 0, fmt.Errorf("test: provider inválido %q", cfg.Provider)
	}

	if cfg.Provider != "vertex" && cfg.APIKey == "" {
		return 0, fmt.Errorf("test: provider %q requiere API key", cfg.Provider)
	}

	start := time.Now()
	client, err := buildClient(cfg)
	if err != nil {
		return time.Since(start).Milliseconds(), err
	}
	// Cerrar el cliente efímero si implementa io.Closer
	if closer, ok := client.(io.Closer); ok {
		defer closer.Close()
	}

	// buildClient ya ejecutó TestConnection internamente. No repetirlo — sería latencia doble.
	return time.Since(start).Milliseconds(), nil
}

// =============================================================================
// Internos
// =============================================================================

// buildClient es el switch central. Si cfg.Provider no es reconocido → error.
// Cada constructor ejecuta TestConnection con timeout interno de 10s antes de retornar.
// Si falla, retorna error sin dejar conexiones colgadas.
func buildClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "vertex":
		return NewVertexAdapter(cfg)
	case "openai", "deepseek", "kimi":
		return NewOpenAICompatAdapter(cfg)
	case "anthropic":
		return NewAnthropicAdapter(cfg)
	default:
		return nil, fmt.Errorf("llm.buildClient: provider desconocido %q", cfg.Provider)
	}
}

// loadConfigFromDB lee las 4 claves de system_config y desencripta la API key.
// Si llm_provider no existe en DB, usa "vertex" como default seguro.
// Si la desencriptación falla (APP_SECRET rotado), retorna error explícito.
func (f *Factory) loadConfigFromDB() (Config, error) {
	cfg := Config{
		Provider: "vertex", // default seguro (usa ADC, no requiere API key)
	}

	if p, err := f.sysRepo.FindByKey(KeyProvider); err == nil && p.ConfigValue != "" {
		cfg.Provider = p.ConfigValue
	}

	if !IsValidProvider(cfg.Provider) {
		return cfg, fmt.Errorf("provider en DB inválido: %q", cfg.Provider)
	}

	if m, err := f.sysRepo.FindByKey(KeyModel); err == nil {
		cfg.Model = m.ConfigValue
	}

	if e, err := f.sysRepo.FindByKey(KeyEndpoint); err == nil {
		cfg.Endpoint = e.ConfigValue
	}

	if k, err := f.sysRepo.FindByKey(KeyAPIKey); err == nil && k.ConfigValue != "" {
		plain, err := f.decryptKey(k.ConfigValue)
		if err != nil {
			return cfg, fmt.Errorf("desencriptando API key (¿APP_SECRET rotado?): %w", err)
		}
		cfg.APIKey = plain
	}

	return cfg, nil
}

// persistConfig upserta las 4 claves en system_config usando Update sobre
// registros existentes (creados por migración 031).
func (f *Factory) persistConfig(provider, apiKeyCipher, model, endpoint string) error {
	kv := map[string]string{
		KeyProvider: provider,
		KeyAPIKey:   apiKeyCipher, // ya encriptado (o vacío para vertex)
		KeyModel:    model,
		KeyEndpoint: endpoint,
	}

	for key, value := range kv {
		existing, err := f.sysRepo.FindByKey(key)
		if err != nil {
			return fmt.Errorf("leyendo %s de DB: %w", key, err)
		}
		existing.ConfigValue = value
		if err := f.sysRepo.Update(existing); err != nil {
			return fmt.Errorf("escribiendo %s a DB: %w", key, err)
		}
	}

	// Silencia warning de variable no usada
	_ = models.SystemConfig{}

	return nil
}

// =============================================================================
// Encriptación AES-256-GCM
// =============================================================================
// GCM provee autenticación + confidencialidad. Nonce aleatorio de 12 bytes por
// encriptación — almacenado junto al ciphertext (base64). El nonce no es secreto,
// solo debe ser único por clave. Sobre disco se ve como base64(nonce || ciphertext).

// encryptKey encripta plain con AES-256-GCM usando f.secret.
// Formato de salida: base64(nonce_12bytes || ciphertext_variable).
// Retorna string vacío si plain es vacío (sin overhead para Vertex).
func (f *Factory) encryptKey(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}

	block, err := aes.NewCipher(f.secret)
	if err != nil {
		return "", fmt.Errorf("AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("GCM mode: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation: %w", err)
	}

	// Seal concatena nonce como prefijo automáticamente si pasamos nonce como primer arg
	// pero la convención aquí es prefijarlo manualmente para que el formato sea obvio
	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(payload), nil
}

// decryptKey desencripta lo que encryptKey produjo. Retorna error si:
//   - base64 inválido
//   - payload muy corto (< NonceSize)
//   - autenticación GCM falla (APP_SECRET rotado o ciphertext corrupto)
func (f *Factory) decryptKey(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(f.secret)
	if err != nil {
		return "", fmt.Errorf("AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("GCM mode: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", errors.New("ciphertext demasiado corto — inválido")
	}

	nonce, ciphertext := payload[:nonceSize], payload[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("autenticación GCM falló (APP_SECRET rotado?): %w", err)
	}

	return string(plain), nil
}
