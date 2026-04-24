package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"gorm.io/gorm"
)

type WhatsappRepository struct {
	db *gorm.DB
}

func NewWhatsappRepository() *WhatsappRepository {
	return &WhatsappRepository{db: database.Get()}
}

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

type ContactSummary struct {
	Phone         string    `json:"phone"`
	LastMessage   string    `json:"last_message"`
	LastRole      string    `json:"last_role"`
	LastMessageAt time.Time `json:"last_message_at"`
	TotalMessages int       `json:"total_messages"`
	UserID        *int      `json:"user_id,omitempty"`
	UserNombre    *string   `json:"user_nombre,omitempty"`
	UserEmail     *string   `json:"user_email,omitempty"`
}

type ConversationMessage struct {
	ID        int64     `json:"id"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Session represents one day's conversation with one phone number.
type Session struct {
	Phone            string    `gorm:"column:phone"              json:"phone"`
	SessionDate      string    `gorm:"column:session_date"       json:"session_date"`
	MessageCount     int       `gorm:"column:message_count"      json:"message_count"`
	FirstMessageAt   time.Time `gorm:"column:first_message_at"   json:"first_message_at"`
	LastMessageAt    time.Time `gorm:"column:last_message_at"    json:"last_message_at"`
	FirstUserMessage string    `gorm:"column:first_user_message" json:"first_user_message"`
	UserID           *int      `gorm:"column:user_id"            json:"user_id,omitempty"`
	UserNombre       *string   `gorm:"column:user_nombre"        json:"user_nombre,omitempty"`
	UserEmail        *string   `gorm:"column:user_email"         json:"user_email,omitempty"`
}

type SessionFilter struct {
	Query   string
	From    string // "YYYY-MM-DD"
	To      string // "YYYY-MM-DD"
	Channel string // "wa" (whatsapp), "tg" (telegram), "" / "all" (ambos)
	Page    int
	Limit   int
}

type SessionsPage struct {
	Sessions []Session `json:"sessions"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
	Pages    int       `json:"pages"`
}

// DigestMessage is used for the email digest (includes user info alongside the message).
type DigestMessage struct {
	Phone      string
	Role       string
	Content    string
	CreatedAt  time.Time
	UserNombre string
	UserEmail  string
}

// ─────────────────────────────────────────────
// Sessions (grouped by phone + day)
// ─────────────────────────────────────────────

// CTE shared by data and count queries.
// first_msgs uses DISTINCT ON to get the first user message per (phone, day) without
// a correlated subquery (which is invalid in a GROUP BY context).
const sessionCTE = `
	WITH first_msgs AS (
		SELECT DISTINCT ON (phone, DATE(created_at AT TIME ZONE 'America/Costa_Rica'))
			phone,
			DATE(created_at AT TIME ZONE 'America/Costa_Rica') AS session_date,
			content AS first_user_message
		FROM whatsapp_conversations
		WHERE role = 'user'
		ORDER BY phone, DATE(created_at AT TIME ZONE 'America/Costa_Rica'), created_at ASC
	),
	sessions AS (
		SELECT
			phone,
			DATE(created_at AT TIME ZONE 'America/Costa_Rica') AS session_date,
			COUNT(*)        AS message_count,
			MIN(created_at) AS first_message_at,
			MAX(created_at) AS last_message_at
		FROM whatsapp_conversations
		GROUP BY phone, DATE(created_at AT TIME ZONE 'America/Costa_Rica')
	),
	user_lookup AS (
		-- Match por últimos 8 dígitos del teléfono (WhatsApp/Telegram con número CR).
		-- Paréntesis necesarios para combinar DISTINCT ON+ORDER BY con UNION ALL.
		(
			SELECT DISTINCT ON (RIGHT(REGEXP_REPLACE(telefono, '[^0-9]', '', 'g'), 8))
				RIGHT(REGEXP_REPLACE(telefono, '[^0-9]', '', 'g'), 8) AS phone_key,
				id      AS user_id,
				nombre  AS user_nombre,
				email   AS user_email
			FROM users
			WHERE telefono IS NOT NULL AND activo = true
			ORDER BY RIGHT(REGEXP_REPLACE(telefono, '[^0-9]', '', 'g'), 8), id ASC
		)

		UNION ALL

		-- Match exacto para chat web logueado (phone="web:<user_id>").
		(
			SELECT 'web:' || id::text AS phone_key,
				id     AS user_id,
				nombre AS user_nombre,
				email  AS user_email
			FROM users
			WHERE activo = true
		)
	)
`

// buildSessionWhere builds a WHERE clause and arg slice from a SessionFilter.
//
// El filtro Channel discrimina por prefijo de phone: las sesiones de Telegram
// se almacenan con prefijo "tg:<chat_id>" en la columna phone (ver paquete
// internal/telegram). WhatsApp usa el número crudo sin prefijo.
func buildSessionWhere(f SessionFilter) (string, []interface{}) {
	var conds []string
	var args []interface{}

	if f.Query != "" {
		conds = append(conds, "(s.phone ILIKE '%' || ? || '%' OR ul.user_nombre ILIKE '%' || ? || '%')")
		args = append(args, f.Query, f.Query)
	}
	if f.From != "" {
		conds = append(conds, "s.session_date >= ?::date")
		args = append(args, f.From)
	}
	if f.To != "" {
		conds = append(conds, "s.session_date <= ?::date")
		args = append(args, f.To)
	}
	switch f.Channel {
	case "wa":
		// WhatsApp = números crudos, sin prefijo de canal
		conds = append(conds, "s.phone NOT LIKE 'tg:%' AND s.phone NOT LIKE 'web:%'")
	case "tg":
		conds = append(conds, "s.phone LIKE 'tg:%'")
	case "web":
		conds = append(conds, "s.phone LIKE 'web:%'")
	}

	if len(conds) == 0 {
		return "WHERE 1=1", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// GetSessions returns a paginated list of conversation sessions (phone + day).
func (r *WhatsappRepository) GetSessions(f SessionFilter) (SessionsPage, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit

	where, whereArgs := buildSessionWhere(f)

	// Count
	countSQL := sessionCTE + fmt.Sprintf(`
		SELECT COUNT(*) FROM sessions s
		LEFT JOIN user_lookup ul ON ul.phone_key = CASE WHEN s.phone LIKE 'web:%%' THEN s.phone ELSE RIGHT(REGEXP_REPLACE(s.phone, '[^0-9]', '', 'g'), 8) END
		%s`, where)

	var total int64
	if err := r.db.Raw(countSQL, whereArgs...).Scan(&total).Error; err != nil {
		return SessionsPage{}, err
	}

	// Data
	dataArgs := append(whereArgs, f.Limit, offset)
	dataSQL := sessionCTE + fmt.Sprintf(`
		SELECT
			s.phone,
			s.session_date::text,
			s.message_count,
			s.first_message_at,
			s.last_message_at,
			COALESCE(fm.first_user_message, '') AS first_user_message,
			ul.user_id,
			ul.user_nombre,
			ul.user_email
		FROM sessions s
		LEFT JOIN first_msgs fm ON fm.phone = s.phone AND fm.session_date = s.session_date
		LEFT JOIN user_lookup ul ON ul.phone_key = CASE WHEN s.phone LIKE 'web:%%' THEN s.phone ELSE RIGHT(REGEXP_REPLACE(s.phone, '[^0-9]', '', 'g'), 8) END
		%s
		ORDER BY s.last_message_at DESC
		LIMIT ? OFFSET ?`, where)

	var sessions []Session
	if err := r.db.Raw(dataSQL, dataArgs...).Scan(&sessions).Error; err != nil {
		return SessionsPage{}, err
	}

	if sessions == nil {
		sessions = []Session{}
	}

	pages := int(total) / f.Limit
	if int(total)%f.Limit > 0 {
		pages++
	}

	return SessionsPage{
		Sessions: sessions,
		Total:    int(total),
		Page:     f.Page,
		Limit:    f.Limit,
		Pages:    pages,
	}, nil
}

// GetSessionMessages returns messages for a specific phone on a specific day (YYYY-MM-DD).
func (r *WhatsappRepository) GetSessionMessages(phone, date string) ([]ConversationMessage, error) {
	var results []ConversationMessage
	sql := `
		SELECT id, phone, role, content, created_at
		FROM whatsapp_conversations
		WHERE phone = ?
		  AND DATE(created_at AT TIME ZONE 'America/Costa_Rica') = ?::date
		ORDER BY created_at ASC
	`
	if err := r.db.Raw(sql, phone, date).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// ─────────────────────────────────────────────
// Legacy (kept for digest and backward compat)
// ─────────────────────────────────────────────

func (r *WhatsappRepository) GetContactSummaries() ([]ContactSummary, error) {
	var results []ContactSummary
	sql := `
		WITH latest AS (
			SELECT DISTINCT ON (phone)
				phone, content AS last_message, role AS last_role, created_at AS last_message_at
			FROM whatsapp_conversations
			ORDER BY phone, created_at DESC
		),
		counts AS (
			SELECT phone, COUNT(*) AS total_messages FROM whatsapp_conversations GROUP BY phone
		)
		SELECT l.phone, l.last_message, l.last_role, l.last_message_at, c.total_messages,
		       u.id AS user_id, u.nombre AS user_nombre, u.email AS user_email
		FROM latest l
		JOIN counts c ON l.phone = c.phone
		LEFT JOIN users u
			ON u.telefono IS NOT NULL AND u.activo = true
			AND RIGHT(REGEXP_REPLACE(u.telefono, '[^0-9]', '', 'g'), 8)
			  = RIGHT(REGEXP_REPLACE(l.phone, '[^0-9]', '', 'g'), 8)
		ORDER BY l.last_message_at DESC
	`
	if err := r.db.Raw(sql).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *WhatsappRepository) GetConversation(phone string) ([]ConversationMessage, error) {
	var results []ConversationMessage
	sql := `SELECT id, phone, role, content, created_at FROM whatsapp_conversations WHERE phone = ? ORDER BY created_at ASC`
	if err := r.db.Raw(sql, phone).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *WhatsappRepository) GetDigestMessagesSince(since time.Time) ([]DigestMessage, error) {
	var results []DigestMessage
	sql := `
		SELECT wc.phone, wc.role, wc.content, wc.created_at,
		       COALESCE(u.nombre, '') AS user_nombre,
		       COALESCE(u.email, '')  AS user_email
		FROM whatsapp_conversations wc
		LEFT JOIN users u
			ON u.telefono IS NOT NULL AND u.activo = true
			AND RIGHT(REGEXP_REPLACE(u.telefono, '[^0-9]', '', 'g'), 8)
			  = RIGHT(REGEXP_REPLACE(wc.phone, '[^0-9]', '', 'g'), 8)
		WHERE wc.created_at > ?
		ORDER BY wc.phone, wc.created_at ASC
	`
	if err := r.db.Raw(sql, since).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// ─────────────────────────────────────────────
// Purge
// ─────────────────────────────────────────────

// CountOlderThan returns how many messages would be deleted (dry run).
func (r *WhatsappRepository) CountOlderThan(days int) (int64, error) {
	var count int64
	err := r.db.Raw(
		`SELECT COUNT(*) FROM whatsapp_conversations WHERE created_at < NOW() - (? * INTERVAL '1 day')`,
		days,
	).Scan(&count).Error
	return count, err
}

// PurgeOlderThan deletes messages older than `days` days. Returns rows deleted.
func (r *WhatsappRepository) PurgeOlderThan(days int) (int64, error) {
	result := r.db.Exec(
		`DELETE FROM whatsapp_conversations WHERE created_at < NOW() - (? * INTERVAL '1 day')`,
		days,
	)
	return result.RowsAffected, result.Error
}
