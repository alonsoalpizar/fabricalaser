package models

import (
	"time"
)

type PriceReference struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	ServiceType string  `gorm:"type:varchar(100);not null" json:"service_type"` // grabado_basico, fotograbado, corte_simple, etc.
	MinUSD      float64 `gorm:"type:decimal(10,2);not null" json:"min_usd"`
	MaxUSD      float64 `gorm:"type:decimal(10,2);not null" json:"max_usd"`
	TypicalTime string  `gorm:"type:varchar(50)" json:"typical_time"` // "1-3 min", "15-40 min"
	Description *string `gorm:"type:text" json:"description,omitempty"`
	IsActive    bool    `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (PriceReference) TableName() string {
	return "price_references"
}
