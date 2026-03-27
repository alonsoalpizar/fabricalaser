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

// FindUserByPhone busca un usuario registrado por su número de teléfono local (8 dígitos CR).
// Solo retorna usuarios activos con contraseña establecida.
func (p *pgAdapter) FindUserByPhone(ctx context.Context, phone string) (*UserProfile, error) {
	var row struct {
		Nombre     string
		Apellido   *string
		CedulaType string
		Email      string
		Provincia  *string
		Canton     *string
		Direccion  *string
	}

	// Raw().Scan() garantiza RowsAffected correcto con el driver pgx.
	result := p.db.WithContext(ctx).Raw(
		"SELECT nombre, apellido, cedula_type, email, provincia, canton, direccion FROM users WHERE telefono = ? AND activo = true AND password_hash IS NOT NULL",
		phone,
	).Scan(&row)

	if result.Error != nil {
		return nil, fmt.Errorf("pgAdapter.FindUserByPhone: %w", result.Error)
	}
	if row.Nombre == "" {
		return nil, fmt.Errorf("pgAdapter.FindUserByPhone: no encontrado (phone=%s)", phone)
	}

	profile := &UserProfile{
		Nombre:     row.Nombre,
		CedulaType: row.CedulaType,
		Email:      row.Email,
	}
	if row.Apellido != nil {
		profile.Apellido = *row.Apellido
	}
	if row.Provincia != nil {
		profile.Provincia = *row.Provincia
	}
	if row.Canton != nil {
		profile.Canton = *row.Canton
	}
	if row.Direccion != nil {
		profile.Direccion = *row.Direccion
	}
	return profile, nil
}
