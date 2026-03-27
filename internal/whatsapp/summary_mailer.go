package whatsapp

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"log"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/vertexai/genai"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/redis/go-redis/v9"
)

const (
	digestTo         = "info@fabricalaser.com"
	digestFrom       = "noreply@fabricalaser.com"
	smtpAddr         = "localhost:25"
	digestHours      = 4
	summaryThreshold = 4 // conversations with more messages than this get summarized by Gemini
	redisLastSentKey = "fabricalaser:whatsapp:digest:last_sent"
)

// phoneGroup holds all messages for a single phone number, plus the Gemini summary if generated.
type phoneGroup struct {
	phone      string
	userNombre string
	userEmail  string
	messages   []repository.DigestMessage
	summary    string // populated when len(messages) > summaryThreshold
	summarized bool
}

// ─────────────────────────────────────────────
// Scheduler
// ─────────────────────────────────────────────

// StartDigestScheduler launches a background goroutine that sends the WhatsApp
// digest email every 4 hours. Only sends if there are new messages since the last run.
// StartDigestScheduler is a no-op — digest is triggered via cron (system) or admin endpoint.
func StartDigestScheduler(_ *redis.Client) {
	log.Printf("[WhatsApp digest] Listo — usar endpoint admin o cron para disparar")
}

// ─────────────────────────────────────────────
// Send
// ─────────────────────────────────────────────

// SendDigest queries new WhatsApp conversations since the last digest, summarizes long ones
// with Gemini, and emails a digest to info@fabricalaser.com.
// Returns nil without sending if there are no new messages.
func SendDigest(rc *redis.Client) error {
	since := getLastSent(rc)
	sentAt := time.Now()

	repo := repository.NewWhatsappRepository()
	messages, err := repo.GetDigestMessagesSince(since)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if len(messages) == 0 {
		log.Printf("[WhatsApp digest] No new messages since %s — skipping", since.Format("02 Jan 15:04"))
		setLastSent(rc, sentAt)
		return nil
	}

	groups := groupByPhone(messages)

	// Resumir conversaciones largas con Gemini
	if err := summarizeGroups(groups); err != nil {
		log.Printf("[WhatsApp digest] Gemini summarization partial error: %v", err)
	}

	body := buildEmailBody(groups, since, sentAt)
	subject := fmt.Sprintf("Resumen WhatsApp FabricaLaser — %s", sentAt.Format("02 Jan 15:04 MST"))

	if err := sendMail(subject, body); err != nil {
		return fmt.Errorf("smtp: %w", err)
	}

	setLastSent(rc, sentAt)
	log.Printf("[WhatsApp digest] Sent to %s — %d contacto(s), %d mensaje(s)", digestTo, len(groups), len(messages))
	return nil
}

// ─────────────────────────────────────────────
// Grouping
// ─────────────────────────────────────────────

func groupByPhone(messages []repository.DigestMessage) []phoneGroup {
	index := make(map[string]int)
	var groups []phoneGroup

	for _, m := range messages {
		idx, ok := index[m.Phone]
		if !ok {
			groups = append(groups, phoneGroup{
				phone:      m.Phone,
				userNombre: m.UserNombre,
				userEmail:  m.UserEmail,
			})
			idx = len(groups) - 1
			index[m.Phone] = idx
		}
		groups[idx].messages = append(groups[idx].messages, m)
	}
	return groups
}

// ─────────────────────────────────────────────
// Gemini summarization
// ─────────────────────────────────────────────

func summarizeGroups(groups []phoneGroup) error {
	// Check if any group needs summarization before creating the client
	needsSummary := false
	for _, g := range groups {
		if len(g.messages) > summaryThreshold {
			needsSummary = true
			break
		}
	}
	if !needsSummary {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, waProjectID, waLocation)
	if err != nil {
		return fmt.Errorf("vertex ai client: %w", err)
	}
	defer client.Close()

	const maxConcurrent = 3
	sem := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := range groups {
		if len(groups[i].messages) <= summaryThreshold {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			summary, err := geminiSummarize(ctx, client, groups[idx])
			if err != nil {
				log.Printf("[WhatsApp digest] Gemini failed for %s: %v — usando mensajes completos", groups[idx].phone, err)
				return
			}
			mu.Lock()
			groups[idx].summary = summary
			groups[idx].summarized = true
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return nil
}

func geminiSummarize(ctx context.Context, client *genai.Client, g phoneGroup) (string, error) {
	model := client.GenerativeModel(waModelName)
	model.SetTemperature(0.2)
	model.SetMaxOutputTokens(2048)

	// Limitar a los últimos 40 mensajes para no exceder el contexto de Gemini
	msgs := g.messages
	const maxMsgsForSummary = 40
	truncated := len(msgs) > maxMsgsForSummary
	if truncated {
		msgs = msgs[len(msgs)-maxMsgsForSummary:]
	}

	var conv strings.Builder
	for _, m := range msgs {
		label := "Cliente"
		if m.Role == "model" {
			label = "Asistente"
		}
		fmt.Fprintf(&conv, "[%s]: %s\n", label, m.Content)
	}

	contextNote := ""
	if truncated {
		contextNote = fmt.Sprintf("(Nota: se muestran los últimos %d mensajes de %d totales)\n\n", maxMsgsForSummary, len(g.messages))
	}

	prompt := fmt.Sprintf(`Sos el asistente interno de ventas de FabricaLaser. Tu tarea es escribir un resumen de esta conversación de WhatsApp para que el asesor humano entienda exactamente qué pasó y pueda dar seguimiento sin tener que leer todo el hilo.

Escribí un párrafo fluido (no lista, no puntos) que explique: qué quería el cliente, qué productos o servicios consultó, qué materiales y medidas mencionó, qué cantidad necesitaba, si se calculó alguna cotización y a qué precio quedó, cómo respondió el cliente ante el precio, si mostró intención de compra o dudó, y cómo terminó la conversación (cerró, quedó pendiente, escaló al asesor, o simplemente dejó de responder). Si hay algo urgente o inusual que el asesor deba saber, mencionalo al final.

Escribí en español directo, como si le hablaras al asesor de igual a igual. Sin asteriscos, sin listas, sin títulos. Solo el párrafo.

%sConversación (%d mensajes):
%s`, contextNote, len(msgs), conv.String())

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response")
	}

	var result strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			result.WriteString(string(text))
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// ─────────────────────────────────────────────
// Email HTML
// ─────────────────────────────────────────────

func buildEmailBody(groups []phoneGroup, since, sentAt time.Time) string {
	crLoc, _ := time.LoadLocation("America/Costa_Rica")
	if crLoc != nil {
		since = since.In(crLoc)
		sentAt = sentAt.In(crLoc)
	}

	totalMsgs := 0
	for _, g := range groups {
		totalMsgs += len(g.messages)
	}

	var buf bytes.Buffer
	buf.WriteString(`<!DOCTYPE html><html lang="es"><head><meta charset="UTF-8">
<style>
  body{font-family:Arial,sans-serif;background:#f5f5f5;margin:0;padding:20px}
  .wrap{max-width:680px;margin:0 auto;background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 8px rgba(0,0,0,.1)}
  .hdr{background:#9B2020;color:#fff;padding:22px 28px}
  .hdr h1{margin:0;font-size:19px}
  .hdr p{margin:4px 0 0;font-size:12px;opacity:.85}
  .body{padding:18px 28px}
  .kpi{background:#fef3c7;border:1px solid #fcd34d;border-radius:6px;padding:10px 16px;margin-bottom:18px;font-size:13px;color:#92400e}
  .contact{margin-bottom:22px;border:1px solid #e5e7eb;border-radius:6px;overflow:hidden}
  .chdr{background:#1a1a1a;color:#f5f5f5;padding:9px 14px;font-size:13px;font-weight:600;display:flex;align-items:center;gap:8px}
  .chdr .utag{font-size:11px;font-weight:400;color:#fca5a5;margin-left:auto}
  .ai-box{background:#f0fdf4;border-left:3px solid #16a34a;padding:10px 14px;font-size:13px;color:#166534;line-height:1.55}
  .ai-label{font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:.05em;color:#15803d;margin-bottom:4px}
  .msg{padding:9px 14px;border-bottom:1px solid #f0f0f0;display:flex;gap:9px;align-items:flex-start}
  .msg:last-child{border-bottom:none}
  .role{font-size:11px;font-weight:700;text-transform:uppercase;padding:2px 7px;border-radius:9999px;white-space:nowrap}
  .ru{background:#fee2e2;color:#991b1b}
  .rm{background:#dcfce7;color:#166534}
  .mc{font-size:13px;color:#374151;line-height:1.5;flex:1}
  .mt{font-size:11px;color:#9ca3af;white-space:nowrap}
  .ftr{background:#f9fafb;padding:12px 28px;font-size:11px;color:#6b7280;text-align:center;border-top:1px solid #e5e7eb}
  a{color:#9B2020}
</style></head><body><div class="wrap">`)

	fmt.Fprintf(&buf,
		`<div class="hdr"><h1>📱 Resumen de WhatsApp</h1><p>%s → %s (nuevos desde el último envío)</p></div>`,
		since.Format("02 Jan 15:04"), sentAt.Format("15:04 MST"))

	buf.WriteString(`<div class="body">`)
	fmt.Fprintf(&buf,
		`<div class="kpi">%d mensaje(s) de %d contacto(s). Conversaciones con más de %d mensajes fueron resumidas por IA.</div>`,
		totalMsgs, len(groups), summaryThreshold)

	for _, g := range groups {
		// Contact header
		userTag := ""
		if g.userNombre != "" {
			userTag = fmt.Sprintf(`<span class="utag">✓ %s · %s</span>`,
				html.EscapeString(g.userNombre), html.EscapeString(g.userEmail))
		}
		msgCount := fmt.Sprintf("(%d msg)", len(g.messages))
		fmt.Fprintf(&buf, `<div class="contact"><div class="chdr">%s %s%s</div>`,
			html.EscapeString(g.phone), msgCount, userTag)

		if g.summarized {
			// Gemini summary
			fmt.Fprintf(&buf,
				`<div class="ai-box"><div class="ai-label">✨ Resumen generado por IA</div>%s</div>`,
				html.EscapeString(g.summary))
		} else {
			// Full messages (≤ threshold)
			for _, m := range g.messages {
				roleClass, roleLabel := "ru", "Cliente"
				if m.Role == "model" {
					roleClass, roleLabel = "rm", "Bot"
				}
				ts := m.CreatedAt
				if crLoc != nil {
					ts = ts.In(crLoc)
				}
				content := m.Content
				if len(content) > 400 {
					content = content[:400] + "…"
				}
				fmt.Fprintf(&buf,
					`<div class="msg"><span class="role %s">%s</span><span class="mc">%s</span><span class="mt">%s</span></div>`,
					roleClass, roleLabel,
					strings.ReplaceAll(html.EscapeString(content), "\n", "<br>"),
					ts.Format("15:04"),
				)
			}
		}
		buf.WriteString(`</div>`)
	}

	buf.WriteString(`</div>`)
	buf.WriteString(`<div class="ftr">FabricaLaser · <a href="https://fabricalaser.com/admin/whatsapp.html">Ver bitácora completa</a></div>`)
	buf.WriteString(`</div></body></html>`)
	return buf.String()
}

// ─────────────────────────────────────────────
// SMTP
// ─────────────────────────────────────────────

// sendMail sends via local Postfix without STARTTLS (localhost relay, no cert needed).
func sendMail(subject, htmlBody string) error {
	conn, err := net.Dial("tcp", smtpAddr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if err := client.Mail(digestFrom); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(digestTo); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	mime := "MIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n"
	msg := fmt.Sprintf("From: FabricaLaser <%s>\r\nTo: %s\r\nSubject: %s\r\n%s\r\n%s",
		digestFrom, digestTo, subject, mime, htmlBody)
	if _, err := fmt.Fprint(w, msg); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return w.Close()
}

// ─────────────────────────────────────────────
// Redis helpers
// ─────────────────────────────────────────────

func getLastSent(rc *redis.Client) time.Time {
	ctx := context.Background()
	val, err := rc.Get(ctx, redisLastSentKey).Int64()
	if err != nil {
		// First run: cover the last digestHours hours
		return time.Now().Add(-digestHours * time.Hour)
	}
	return time.Unix(val, 0)
}

func setLastSent(rc *redis.Client, t time.Time) {
	ctx := context.Background()
	if err := rc.Set(ctx, redisLastSentKey, t.Unix(), 0).Err(); err != nil {
		log.Printf("[WhatsApp digest] Redis setLastSent error: %v", err)
	}
}
