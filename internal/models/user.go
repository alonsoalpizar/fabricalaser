package models

import (
	"time"

	"gorm.io/datatypes"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Cedula       string         `gorm:"type:varchar(10);uniqueIndex:idx_users_cedula_unique,where:password_hash IS NOT NULL" json:"cedula"`
	CedulaType   string         `gorm:"type:varchar(10);default:'fisica'" json:"cedula_type"` // fisica, juridica
	Nombre       string         `gorm:"type:varchar(100);not null" json:"nombre"`
	Apellido     *string        `gorm:"type:varchar(100)" json:"apellido,omitempty"`
	Email        string         `gorm:"type:varchar(255);not null" json:"email"`
	Telefono     *string        `gorm:"type:varchar(20)" json:"telefono,omitempty"`
	PasswordHash *string        `gorm:"type:varchar(255)" json:"-"`
	Role         string         `gorm:"type:varchar(20);default:'customer'" json:"role"` // customer, admin
	QuoteQuota   int            `gorm:"default:5" json:"quote_quota"`                    // -1 = unlimited
	QuotesUsed   int            `gorm:"default:0" json:"quotes_used"`
	Activo       bool           `gorm:"default:true" json:"activo"`
	UltimoLogin  *time.Time     `json:"ultimo_login,omitempty"`
	Metadata     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

// HasPassword returns true if user has a password set
func (u *User) HasPassword() bool {
	return u.PasswordHash != nil && *u.PasswordHash != ""
}

// CanQuote returns true if user can still create quotes
func (u *User) CanQuote() bool {
	if u.QuoteQuota == -1 {
		return true // unlimited
	}
	return u.QuotesUsed < u.QuoteQuota
}

// RemainingQuotes returns the number of remaining quotes
func (u *User) RemainingQuotes() int {
	if u.QuoteQuota == -1 {
		return -1 // unlimited
	}
	remaining := u.QuoteQuota - u.QuotesUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsAdmin returns true if user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

// ToPublicJSON returns user data safe for API responses
func (u *User) ToPublicJSON() map[string]interface{} {
	return map[string]interface{}{
		"id":              u.ID,
		"cedula":          u.Cedula,
		"cedula_type":     u.CedulaType,
		"nombre":          u.Nombre,
		"apellido":        u.Apellido,
		"nombre_completo": u.NombreCompleto(),
		"email":           u.Email,
		"telefono":        u.Telefono,
		"role":            u.Role,
		"quote_quota":     u.QuoteQuota,
		"quotes_used":     u.QuotesUsed,
		"activo":          u.Activo,
		"created_at":      u.CreatedAt,
	}
}

// NombreCompleto returns full name
func (u *User) NombreCompleto() string {
	if u.Apellido != nil && *u.Apellido != "" {
		return u.Nombre + " " + *u.Apellido
	}
	return u.Nombre
}
