package models

import (
	"time"
)

type TechRate struct {
	ID               uint    `gorm:"primaryKey" json:"id"`
	TechnologyID     uint    `gorm:"not null;index" json:"technology_id"`
	EngraveRateHour  float64 `gorm:"type:decimal(10,4);not null" json:"engrave_rate_hour"`  // USD/hour
	CutRateHour      float64 `gorm:"type:decimal(10,4);not null" json:"cut_rate_hour"`      // USD/hour
	DesignRateHour   float64 `gorm:"type:decimal(10,4);not null" json:"design_rate_hour"`   // USD/hour
	OverheadRateHour float64 `gorm:"type:decimal(10,4);default:3.78" json:"overhead_rate_hour"` // Fixed costs USD/hour
	SetupFee         float64 `gorm:"type:decimal(10,4);default:0" json:"setup_fee"`         // One-time setup fee
	CostPerMinEngrave float64 `gorm:"type:decimal(10,6);not null" json:"cost_per_min_engrave"` // Calculated: (engrave + overhead) / 60
	CostPerMinCut    float64 `gorm:"type:decimal(10,6);not null" json:"cost_per_min_cut"`    // Calculated: (cut + overhead) / 60
	MarginPercent    float64 `gorm:"type:decimal(5,4);default:0.40" json:"margin_percent"`  // Default 40%
	IsActive         bool    `gorm:"default:true" json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Relations
	Technology Technology `gorm:"foreignKey:TechnologyID" json:"technology,omitempty"`
}

func (TechRate) TableName() string {
	return "tech_rates"
}
