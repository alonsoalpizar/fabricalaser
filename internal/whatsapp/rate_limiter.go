package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─── Claves Redis ─────────────────────────────────────────────────────────────
//
// wa:daily:phones:{YYYY-MM-DD}  → Redis SET de teléfonos únicos del día (UTC)
// wa:daily:count:{YYYY-MM-DD}   → entero — conversaciones nuevas del día
//
// Ambas claves tienen TTL de 48h para auto-limpieza.

const rateLimiterTTL = 48 * time.Hour

// RateLimiter decide si un mensaje entrante debe procesarse según el límite diario.
type RateLimiter struct {
	rc        *redis.Client
	limit     int // WHATSAPP_DAILY_LIMIT (default 230)
	alertAt   int // WHATSAPP_ALERT_THRESHOLD (default 200)
	alertMail string
}

// NewRateLimiter construye el limitador leyendo configuración de env vars.
func NewRateLimiter(rc *redis.Client) *RateLimiter {
	limit := envInt("WHATSAPP_DAILY_LIMIT", 230)
	alertAt := envInt("WHATSAPP_ALERT_THRESHOLD", 200)
	alertMail := os.Getenv("WHATSAPP_ALERT_EMAIL")
	if alertMail == "" {
		alertMail = digestTo // reutiliza la constante de summary_mailer.go
	}
	return &RateLimiter{
		rc:        rc,
		limit:     limit,
		alertAt:   alertAt,
		alertMail: alertMail,
	}
}

// CheckResult es el resultado de la verificación del rate limiter.
type CheckResult int

const (
	// Allow — procesar normalmente
	Allow CheckResult = iota
	// AllowContinuation — conversación ya existente hoy, no cuenta contra el límite
	AllowContinuation
	// Deny — límite alcanzado, no procesar
	Deny
)

// Check verifica si el teléfono puede ser procesado.
//
// Lógica:
//  1. Si el teléfono ya está en el SET del día → es continuación → Allow sin contar
//  2. Si el contador ya llegó al límite → Deny
//  3. Si hay error de Redis → fail open (Allow) para no bloquear clientes
//  4. SADD al set + INCR al contador → Allow
//  5. Si el contador cruzó el umbral de alerta → enviar email
func (rl *RateLimiter) Check(ctx context.Context, phone string) CheckResult {
	if rl.rc == nil {
		// Fail open: sin Redis no limitamos
		slog.Warn("whatsapp: rate limiter sin Redis — fail open", "phone", phone)
		return Allow
	}

	dateKey := utcDateKey()
	phonesKey := fmt.Sprintf("wa:daily:phones:%s", dateKey)
	countKey := fmt.Sprintf("wa:daily:count:%s", dateKey)

	// ── 1. ¿Conversación ya existente hoy? ───────────────────────────────────
	isMember, err := rl.rc.SIsMember(ctx, phonesKey, phone).Result()
	if err != nil {
		slog.Warn("whatsapp: rate limiter error SIsMember — fail open", "error", err, "phone", phone)
		return Allow
	}
	if isMember {
		slog.Debug("whatsapp: rate limiter — conversación continuada", "phone", phone)
		return AllowContinuation
	}

	// ── 2. ¿Límite ya alcanzado? ──────────────────────────────────────────────
	current, err := rl.rc.Get(ctx, countKey).Int()
	if err != nil && err != redis.Nil {
		slog.Warn("whatsapp: rate limiter error GET count — fail open", "error", err)
		return Allow
	}
	if current >= rl.limit {
		slog.Warn("whatsapp: rate limiter — límite alcanzado, mensaje descartado",
			"phone", phone,
			"count", current,
			"limit", rl.limit,
		)
		go rl.sendLimitAlert(current)
		return Deny
	}

	// ── 3. Registrar nueva conversación ──────────────────────────────────────
	// SADD es idempotente — si por alguna race condition ya está, no duplica
	if err := rl.rc.SAdd(ctx, phonesKey, phone).Err(); err != nil {
		slog.Warn("whatsapp: rate limiter error SAdd — continuando", "error", err)
	}
	rl.rc.Expire(ctx, phonesKey, rateLimiterTTL)

	// INCR es atómico en Redis
	newCount, err := rl.rc.Incr(ctx, countKey).Result()
	if err != nil {
		slog.Warn("whatsapp: rate limiter error INCR — continuando", "error", err)
	} else {
		rl.rc.Expire(ctx, countKey, rateLimiterTTL)
		slog.Info("whatsapp: rate limiter — nueva conversación",
			"phone", phone,
			"count", newCount,
			"limit", rl.limit,
		)
		// ── 4. Alerta de umbral ───────────────────────────────────────────────
		if int(newCount) >= rl.alertAt {
			go rl.sendThresholdAlert(int(newCount))
		}
	}

	return Allow
}

// DailyStats retorna estadísticas del día actual (para el admin o logs).
func (rl *RateLimiter) DailyStats(ctx context.Context) (count int, phones []string, err error) {
	dateKey := utcDateKey()
	countKey := fmt.Sprintf("wa:daily:count:%s", dateKey)
	phonesKey := fmt.Sprintf("wa:daily:phones:%s", dateKey)

	count, err = rl.rc.Get(ctx, countKey).Int()
	if err == redis.Nil {
		count, err = 0, nil
	}
	if err != nil {
		return
	}

	phones, err = rl.rc.SMembers(ctx, phonesKey).Result()
	return
}

// ─── Alertas de email ────────────────────────────────────────────────────────

func (rl *RateLimiter) sendThresholdAlert(count int) {
	subject := fmt.Sprintf("⚠️ WhatsApp FabricaLaser — %d/%d conversaciones hoy", count, rl.limit)
	body := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;padding:20px">
<h2 style="color:#b45309">⚠️ Alerta de capacidad WhatsApp</h2>
<p>Se han iniciado <strong>%d conversaciones</strong> hoy de un máximo configurado de <strong>%d</strong>.</p>
<p>Meta permite hasta 250 conversaciones diarias sin verificación de negocio.</p>
<p>Si se acerca al límite real de Meta, los mensajes dejarán de enviarse silenciosamente.</p>
<hr>
<p style="color:#6b7280;font-size:12px">FabricaLaser · Rate Limiter automático</p>
</body></html>`, count, rl.limit)
	if err := sendMail(subject, body); err != nil {
		slog.Error("whatsapp: error enviando alerta de umbral", "error", err)
	} else {
		slog.Info("whatsapp: alerta de umbral enviada", "count", count, "to", rl.alertMail)
	}
}

func (rl *RateLimiter) sendLimitAlert(count int) {
	subject := fmt.Sprintf("🚫 WhatsApp FabricaLaser — Límite alcanzado (%d conversaciones)", count)
	body := fmt.Sprintf(`<html><body style="font-family:Arial,sans-serif;padding:20px">
<h2 style="color:#dc2626">🚫 Límite diario de WhatsApp alcanzado</h2>
<p>Se alcanzaron <strong>%d conversaciones</strong> hoy (límite: %d).</p>
<p>Los mensajes de nuevos contactos están siendo <strong>descartados</strong> hasta la medianoche UTC.</p>
<p>Las conversaciones ya iniciadas hoy <strong>siguen respondiendo</strong> normalmente.</p>
<p><strong>Acción recomendada:</strong> Verificar el negocio en Meta Business Manager para subir el límite a 1,000 diarias.</p>
<hr>
<p style="color:#6b7280;font-size:12px">FabricaLaser · Rate Limiter automático</p>
</body></html>`, count, rl.limit)
	if err := sendMail(subject, body); err != nil {
		slog.Error("whatsapp: error enviando alerta de límite", "error", err)
	} else {
		slog.Info("whatsapp: alerta de límite enviada", "count", count, "to", rl.alertMail)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// utcDateKey retorna la fecha UTC actual en formato YYYY-MM-DD.
// Crítico: Meta resetea su contador en medianoche UTC, no en la hora local del servidor.
func utcDateKey() string {
	return time.Now().UTC().Format("2006-01-02")
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}
