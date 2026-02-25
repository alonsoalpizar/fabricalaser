package models

import (
	"time"
)

type Technology struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	Code            string  `gorm:"type:varchar(20);uniqueIndex;not null" json:"code"` // CO2, UV, FIBRA, MOPA
	Name            string  `gorm:"type:varchar(100);not null" json:"name"`
	Description     *string `gorm:"type:text" json:"description,omitempty"`
	UVPremiumFactor float64 `gorm:"type:decimal(5,4);default:0" json:"uv_premium_factor"` // 0.15-0.25 for UV
	IsActive        bool    `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Relations
	TechRates []TechRate `gorm:"foreignKey:TechnologyID" json:"tech_rates,omitempty"`
}

func (Technology) TableName() string {
	return "technologies"
}
