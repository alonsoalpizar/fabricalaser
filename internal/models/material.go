package models

import (
	"time"

	"gorm.io/datatypes"
)

type Material struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	Name       string         `gorm:"type:varchar(100);not null" json:"name"`
	Category   string         `gorm:"type:varchar(50);not null" json:"category"` // madera, acrilico, metal, etc.
	Factor     float64        `gorm:"type:decimal(5,4);default:1.0" json:"factor"` // 1.0 - 1.8
	Thicknesses datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"thicknesses"` // [3, 5, 6, 10] mm
	Notes      *string        `gorm:"type:text" json:"notes,omitempty"`
	IsActive   bool           `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

func (Material) TableName() string {
	return "materials"
}
