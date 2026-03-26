package whatsapp

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type pgAdapter struct {
	db *gorm.DB
}

// NewPGAdapter crea un PGClient que implementa la interfaz usando GORM.
func NewPGAdapter(db *gorm.DB) PGClient {
	return &pgAdapter{db: db}
}

func (p *pgAdapter) SaveTurn(ctx context.Context, turn ConversationTurn) error {
	result := p.db.WithContext(ctx).Exec(
		"INSERT INTO whatsapp_conversations (phone, role, content, created_at) VALUES (?, ?, ?, ?)",
		turn.Phone, turn.Role, turn.Content, turn.CreatedAt,
	)
	if result.Error != nil {
		return fmt.Errorf("pgAdapter.SaveTurn: %w", result.Error)
	}
	return nil
}
