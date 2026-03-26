package repository

import (
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

type ContactSummary struct {
	Phone         string    `json:"phone"`
	LastMessage   string    `json:"last_message"`
	LastRole      string    `json:"last_role"`
	LastMessageAt time.Time `json:"last_message_at"`
	TotalMessages int       `json:"total_messages"`
	// Mapped user (nullable)
	UserID     *int    `json:"user_id,omitempty"`
	UserNombre *string `json:"user_nombre,omitempty"`
	UserEmail  *string `json:"user_email,omitempty"`
}

type ConversationMessage struct {
	ID        int64     `json:"id"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
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

// phoneNormSQL returns a SQL expression that strips non-digits and removes the 506 prefix,
// leaving the 8-digit Costa Rica local number for comparison.
// RIGHT(..., 8) handles both "50686091954" and "86091954" correctly.
const phoneNormSQL = `RIGHT(REGEXP_REPLACE(%s, '[^0-9]', '', 'g'), 8)`

// GetContactSummaries returns one row per phone with last message info, total count,
// and the matched FabricaLaser user (if any) via phone normalization.
func (r *WhatsappRepository) GetContactSummaries() ([]ContactSummary, error) {
	var results []ContactSummary
	sql := `
		WITH latest AS (
			SELECT DISTINCT ON (phone)
				phone,
				content    AS last_message,
				role       AS last_role,
				created_at AS last_message_at
			FROM whatsapp_conversations
			ORDER BY phone, created_at DESC
		),
		counts AS (
			SELECT phone, COUNT(*) AS total_messages
			FROM whatsapp_conversations
			GROUP BY phone
		)
		SELECT
			l.phone, l.last_message, l.last_role, l.last_message_at, c.total_messages,
			u.id    AS user_id,
			u.nombre AS user_nombre,
			u.email  AS user_email
		FROM latest l
		JOIN counts c ON l.phone = c.phone
		LEFT JOIN users u
			ON u.telefono IS NOT NULL
			AND u.activo = true
			AND RIGHT(REGEXP_REPLACE(u.telefono, '[^0-9]', '', 'g'), 8)
			  = RIGHT(REGEXP_REPLACE(l.phone,    '[^0-9]', '', 'g'), 8)
		ORDER BY l.last_message_at DESC
	`
	if err := r.db.Raw(sql).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// GetConversation returns all messages for a phone number in chronological order.
func (r *WhatsappRepository) GetConversation(phone string) ([]ConversationMessage, error) {
	var results []ConversationMessage
	sql := `
		SELECT id, phone, role, content, created_at
		FROM whatsapp_conversations
		WHERE phone = ?
		ORDER BY created_at ASC
	`
	if err := r.db.Raw(sql, phone).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// GetDigestMessagesSince returns all messages after `since`, joined with user info,
// ordered by phone and timestamp. Used for the email digest.
func (r *WhatsappRepository) GetDigestMessagesSince(since time.Time) ([]DigestMessage, error) {
	var results []DigestMessage
	sql := `
		SELECT
			wc.phone,
			wc.role,
			wc.content,
			wc.created_at,
			COALESCE(u.nombre, '')  AS user_nombre,
			COALESCE(u.email, '')   AS user_email
		FROM whatsapp_conversations wc
		LEFT JOIN users u
			ON u.telefono IS NOT NULL
			AND u.activo = true
			AND RIGHT(REGEXP_REPLACE(u.telefono, '[^0-9]', '', 'g'), 8)
			  = RIGHT(REGEXP_REPLACE(wc.phone,   '[^0-9]', '', 'g'), 8)
		WHERE wc.created_at > ?
		ORDER BY wc.phone, wc.created_at ASC
	`
	if err := r.db.Raw(sql, since).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
