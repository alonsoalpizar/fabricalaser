package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConversationRepository persiste sesiones y mensajes del chat administrativo.
// Acceso GORM directo sobre tablas admin_chat_sessions y admin_chat_messages
// (definidas en migration 029).
type ConversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository construye el repo con la conexión global.
func NewConversationRepository() *ConversationRepository {
	return &ConversationRepository{db: database.Get()}
}

// ─── Tipos de retorno ────────────────────────────────────────────────────────

// ChatTurn es la representación reducida de un mensaje para alimentar al LLM.
type ChatTurn struct {
	Role    string `json:"role"`    // "user" | "model"
	Content string `json:"content"` // texto plano
}

// MessageRow es la representación completa de un mensaje persistido.
type MessageRow struct {
	ID        int64     `gorm:"column:id"          json:"id"`
	SessionID string    `gorm:"column:session_id"  json:"session_id"`
	AdminID   uint      `gorm:"column:admin_id"    json:"admin_id"`
	Role      string    `gorm:"column:role"        json:"role"`
	Content   string    `gorm:"column:content"     json:"content"`
	ToolCalls *string   `gorm:"column:tool_calls"  json:"tool_calls,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at"  json:"created_at"`
}

// SessionRow es lo que retorna ListSessions: incluye admin_nombre y preview
// resueltos por SQL (R5 del plan — no cargar mensajes en memoria).
type SessionRow struct {
	ID                     string     `gorm:"column:id"                       json:"id"`
	AdminID                uint       `gorm:"column:admin_id"                 json:"admin_id"`
	AdminNombre            string     `gorm:"column:admin_nombre"             json:"admin_nombre"`
	StartedAt              time.Time  `gorm:"column:started_at"               json:"started_at"`
	EndedAt                *time.Time `gorm:"column:ended_at"                 json:"ended_at,omitempty"`
	MessageCount           int        `gorm:"column:message_count"            json:"message_count"`
	PrimeraConsultaPreview *string    `gorm:"column:primera_consulta_preview" json:"primera_consulta_preview,omitempty"`
}

// ─── Sesiones ────────────────────────────────────────────────────────────────

// CreateSession crea una nueva sesión vacía y devuelve su UUID.
// El cierre de huérfanas previas es responsabilidad del caller (handler.go),
// que debe invocar EndAllOpenForAdmin antes según el flujo del plan.
func (r *ConversationRepository) CreateSession(adminID uint) (string, error) {
	id := uuid.New().String()
	err := r.db.Exec(`
		INSERT INTO admin_chat_sessions (id, admin_id, started_at, message_count)
		VALUES (?, ?, NOW(), 0)
	`, id, adminID).Error
	if err != nil {
		return "", fmt.Errorf("CreateSession: %w", err)
	}
	return id, nil
}

// EndSession marca una sesión específica como cerrada (ended_at = NOW()).
func (r *ConversationRepository) EndSession(sessionID string) error {
	return r.db.Exec(`
		UPDATE admin_chat_sessions
		SET ended_at = NOW()
		WHERE id = ? AND ended_at IS NULL
	`, sessionID).Error
}

// EndAllOpenForAdmin cierra todas las sesiones abiertas del admin.
//
// Contrato (R6 del plan):
//   - Si exceptID == "": cierra TODAS las sesiones abiertas del admin sin excepción.
//     Caso usado en Reset y antes de crear primera sesión cuando no hay nada activo.
//   - Si exceptID != "": cierra todas excepto la indicada.
//     Caso usado tras crear una nueva sesión, para no auto-cerrarla.
//
// Implementa los dos casos con queries distintas (no concatenación de SQL)
// para evitar inyecciones y mantener queries preparables.
func (r *ConversationRepository) EndAllOpenForAdmin(adminID uint, exceptID string) error {
	if exceptID == "" {
		return r.db.Exec(`
			UPDATE admin_chat_sessions
			SET ended_at = NOW()
			WHERE admin_id = ? AND ended_at IS NULL
		`, adminID).Error
	}
	return r.db.Exec(`
		UPDATE admin_chat_sessions
		SET ended_at = NOW()
		WHERE admin_id = ? AND ended_at IS NULL AND id != ?
	`, adminID, exceptID).Error
}

// ─── Mensajes ────────────────────────────────────────────────────────────────

// SaveMessage inserta un mensaje y actualiza message_count atómicamente
// dentro de una transacción (R4 del plan).
//
// toolCalls puede ser nil (mayoría de turnos) o un JSON serializado
// (cuando el modelo invocó funciones).
func (r *ConversationRepository) SaveMessage(
	ctx context.Context,
	sessionID string,
	adminID uint,
	role string,
	content string,
	toolCalls *string,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			INSERT INTO admin_chat_messages (session_id, admin_id, role, content, tool_calls, created_at)
			VALUES (?, ?, ?, ?, ?, NOW())
		`, sessionID, adminID, role, content, toolCalls).Error; err != nil {
			return fmt.Errorf("SaveMessage insert: %w", err)
		}

		if err := tx.Exec(`
			UPDATE admin_chat_sessions
			SET message_count = message_count + 1
			WHERE id = ?
		`, sessionID).Error; err != nil {
			return fmt.Errorf("SaveMessage update count: %w", err)
		}

		return nil
	})
}

// LoadActiveSession busca la sesión abierta más reciente del admin y carga su
// historial completo. Devuelve (turnos, sessionID, error). Si no hay sesión
// abierta retorna ([], "", nil) — no es error.
func (r *ConversationRepository) LoadActiveSession(adminID uint) ([]ChatTurn, string, error) {
	var sessionID string
	err := r.db.Raw(`
		SELECT id FROM admin_chat_sessions
		WHERE admin_id = ? AND ended_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1
	`, adminID).Scan(&sessionID).Error
	if err != nil {
		return nil, "", fmt.Errorf("LoadActiveSession: %w", err)
	}
	if sessionID == "" {
		return []ChatTurn{}, "", nil
	}

	msgs, err := r.GetSessionMessages(sessionID)
	if err != nil {
		return nil, "", err
	}

	turns := make([]ChatTurn, 0, len(msgs))
	for _, m := range msgs {
		// Solo user/model son contexto para el LLM. role='tool' es metadata
		// de auditoría y no se replay-ea en la próxima llamada.
		if m.Role == "user" || m.Role == "model" {
			turns = append(turns, ChatTurn{Role: m.Role, Content: m.Content})
		}
	}
	return turns, sessionID, nil
}

// GetSessionMessages devuelve todos los mensajes de una sesión en orden cronológico.
func (r *ConversationRepository) GetSessionMessages(sessionID string) ([]MessageRow, error) {
	var rows []MessageRow
	err := r.db.Raw(`
		SELECT id, session_id, admin_id, role, content, tool_calls, created_at
		FROM admin_chat_messages
		WHERE session_id = ?
		ORDER BY created_at ASC, id ASC
	`, sessionID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("GetSessionMessages: %w", err)
	}
	return rows, nil
}

// ─── Auditoría ───────────────────────────────────────────────────────────────

// ListSessions retorna sesiones paginadas (más recientes primero) con admin_nombre
// y primera_consulta_preview resueltos por SQL (R5 del plan).
func (r *ConversationRepository) ListSessions(limit, offset int) ([]SessionRow, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var rows []SessionRow
	err := r.db.Raw(`
		SELECT
			s.id, s.admin_id, s.started_at, s.ended_at, s.message_count,
			u.nombre AS admin_nombre,
			(SELECT LEFT(content, 120)
			 FROM admin_chat_messages
			 WHERE session_id = s.id AND role = 'user'
			 ORDER BY created_at ASC
			 LIMIT 1) AS primera_consulta_preview
		FROM admin_chat_sessions s
		JOIN users u ON u.id = s.admin_id
		ORDER BY s.started_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("ListSessions: %w", err)
	}

	var total int64
	if err := r.db.Raw(`SELECT COUNT(*) FROM admin_chat_sessions`).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("ListSessions count: %w", err)
	}

	return rows, total, nil
}
