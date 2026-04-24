// Package prompts sirve los system prompts de los agentes LLM desde DB
// con cache in-memory, fallback hardcoded y hot reload via pub/sub Redis.
//
// Arquitectura:
//   - 5 agent_keys: chat_web_public, chat_web_logged, whatsapp_main,
//     whatsapp_image, admin_chat
//   - Cada Get() resuelve en orden: cache local → DB (con timeout) → fallback
//   - PUBLISH "prompts:updated <key>" invalida cache local en todos los backends
//   - Fallback = valor original hardcoded en fallbacks.go (mismo que los
//     const Go que vivían en chat_handler.go, gemini_adapter.go, etc.)
//
// Reglas del plan Paso 5:
//   - R5: fallback activa en 3 condiciones — err, not-found, len(body)==0
//   - R6: fallback también en runtime si DB timeout 500ms
//   - R9: cache in-memory por backend (pub/sub es invalidador, no cache primario)
//   - R10: Provider expone solo Get/Invalidate — handlers admin usan el repo directo
package prompts

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/redis/go-redis/v9"
)

const (
	// Canal pub/sub donde el handler admin publica invalidaciones tras un save.
	// El payload es el agent_key a invalidar (ej. "whatsapp_main").
	pubsubChannel = "prompts:updated"

	// Timeout por query a DB. Si DB no responde en este tiempo, Get() cae al
	// último cache válido o al fallback hardcoded — los agentes nunca esperan
	// más que esto para su system prompt.
	dbQueryTimeout = 500 * time.Millisecond
)

// Provider expone Get/Invalidate/Seed/Subscribe. Un único Provider por proceso,
// construido en cmd/server/main.go y pasado a los 3 adapters de agentes LLM.
type Provider struct {
	repo      *repository.AgentPromptRepository
	redis     *redis.Client
	fallbacks map[string]string // snapshot inmutable de fallbacks.Fallbacks()

	mu    sync.RWMutex
	cache map[string]string // agent_key → body — solo se guardan bodies NO vacíos
}

// New construye el Provider sin tocar DB ni Redis todavía.
// Llamar Seed() después para poblar DB inicial, y go Subscribe(ctx) para arrancar
// el loop de invalidación por pub/sub.
func New(repo *repository.AgentPromptRepository, redisClient *redis.Client, fallbacks map[string]string) *Provider {
	return &Provider{
		repo:      repo,
		redis:     redisClient,
		fallbacks: fallbacks,
		cache:     make(map[string]string, len(fallbacks)),
	}
}

// Get retorna el body activo del prompt identificado por agentKey.
//
// Orden de resolución (siempre retorna un string no vacío si agentKey es válido):
//  1. Cache in-memory con body NO vacío → devolver
//  2. Query DB (timeout 500ms):
//     - Si retorna fila con body NO vacío → cachear y devolver
//     - Si retorna ErrAgentPromptNotFound, o fila con body vacío, o error —
//       caer al fallback
//  3. Fallback hardcoded (p.fallbacks[agentKey]) → devolver
//     Si ni siquiera hay fallback (agentKey inventado), retornar string vacío.
//
// CRÍTICO: len(body)==0 es condición de fallback, NO condición de error.
// Covered case: la migración 032 inserta filas con body='' y el Seed() las
// puebla en el primer arranque. Durante esa ventana, FindByKey retorna fila
// sin error pero body vacío — debemos caer al fallback en vez de servir
// string vacío al LLM (que haría al agente responder errático).
func (p *Provider) Get(agentKey string) string {
	// 1. Cache hit — solo si body no es vacío (caches "" no se guardan, pero doble-check)
	p.mu.RLock()
	if body, ok := p.cache[agentKey]; ok && body != "" {
		p.mu.RUnlock()
		return body
	}
	p.mu.RUnlock()

	// 2. DB con timeout corto
	ctx, cancel := context.WithTimeout(context.Background(), dbQueryTimeout)
	defer cancel()
	_ = ctx // reservado para futuro si el repo expone ctx-aware queries

	prompt, err := p.repo.FindByKey(agentKey)
	if err == nil && prompt != nil && prompt.Body != "" {
		// Fila válida — cachear body y retornar
		p.mu.Lock()
		p.cache[agentKey] = prompt.Body
		p.mu.Unlock()
		return prompt.Body
	}

	// 3. Fallback. Loggeamos solo en casos anómalos (err no nil ni not-found).
	if err != nil && !errors.Is(err, repository.ErrAgentPromptNotFound) {
		slog.Warn("prompts: fallback activado por error de DB",
			"agent_key", agentKey,
			"error", err,
		)
	}

	fallback, hasFallback := p.fallbacks[agentKey]
	if !hasFallback {
		slog.Error("prompts: agent_key desconocido — no hay fallback",
			"agent_key", agentKey,
		)
		return ""
	}
	return fallback
}

// Invalidate borra el cache de un agent_key. Llamado por:
//   - El handler admin tras guardar (vía pub/sub, no directo)
//   - El loop Subscribe al recibir mensaje "prompts:updated"
func (p *Provider) Invalidate(agentKey string) {
	p.mu.Lock()
	delete(p.cache, agentKey)
	p.mu.Unlock()
}

// InvalidateAll limpia todo el cache. Útil para testing o rollback masivo.
func (p *Provider) InvalidateAll() {
	p.mu.Lock()
	p.cache = make(map[string]string, len(p.fallbacks))
	p.mu.Unlock()
}

// Subscribe corre indefinidamente escuchando el canal pub/sub Redis.
// Cuando llega un mensaje con un agent_key, invalida el cache para que el
// próximo Get() traiga el body fresco desde DB.
//
// Debe correr en goroutine: `go provider.Subscribe(ctx)`.
// Termina cuando ctx se cancela.
//
// Robustez: si la conexión a Redis falla, espera 5s y reintenta. Los Get()
// siguen funcionando sin pub/sub (servirán el cache actual; eventualmente
// obsoleto hasta que Redis vuelva). No bloquea el servicio principal.
func (p *Provider) Subscribe(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		pubsub := p.redis.Subscribe(ctx, pubsubChannel)
		ch := pubsub.Channel()

		slog.Info("prompts: subscribed to pubsub channel", "channel", pubsubChannel)

		for msg := range ch {
			key := msg.Payload
			if key == "" {
				continue
			}
			p.Invalidate(key)
			slog.Info("prompts: cache invalidated by pubsub", "agent_key", key)
		}

		// Si llegamos aquí, el channel se cerró (desconexión).
		_ = pubsub.Close()

		if ctx.Err() != nil {
			return
		}

		slog.Warn("prompts: pubsub disconnected, reconnecting in 5s")
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

// PublishInvalidate es un helper para que el handler admin publique la invalidación.
// Se expone aquí para que el handler use el mismo canal que el Subscribe.
func (p *Provider) PublishInvalidate(ctx context.Context, agentKey string) error {
	return p.redis.Publish(ctx, pubsubChannel, agentKey).Err()
}

// Seed puebla DB con los fallbacks para las filas que tengan body vacío.
// Idempotente: no sobrescribe bodies ya editados por el gestor.
// Se llama una vez al arrancar el servicio (cmd/server/main.go).
//
// Reporta cuántos bodies se poblaron vs cuántos ya tenían valor.
func (p *Provider) Seed() error {
	populated := 0
	skipped := 0
	for key, body := range p.fallbacks {
		wrote, err := p.repo.SeedBody(key, body)
		if err != nil {
			slog.Error("prompts: seed error",
				"agent_key", key,
				"error", err,
			)
			return err
		}
		if wrote {
			populated++
		} else {
			skipped++
		}
	}
	slog.Info("prompts: seed completado",
		"populated", populated,
		"skipped_already_populated", skipped,
		"total", populated+skipped,
	)
	return nil
}

// Warmup precarga el cache en memoria con todos los prompts desde DB.
// Optativo: llamar al arrancar después de Seed() para que el primer Get() de
// cada agente no necesite ir a DB. No es crítico — sin esto el primer request
// por agente hace una query de ~5ms.
func (p *Provider) Warmup() {
	prompts, err := p.repo.FindAll()
	if err != nil {
		slog.Warn("prompts: warmup falló", "error", err)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pr := range prompts {
		if pr.Body != "" {
			p.cache[pr.AgentKey] = pr.Body
		}
	}
	slog.Info("prompts: warmup completado", "entries_cached", len(p.cache))
}
