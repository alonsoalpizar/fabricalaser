package models

import (
	"time"
)

type MaterialCost struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	MaterialID    uint      `gorm:"not null;index" json:"material_id"`
	Thickness     float64   `gorm:"type:decimal(5,2);not null" json:"thickness"`
	CostPerMm2    float64   `gorm:"column:cost_per_mm2;type:decimal(12,8);not null" json:"cost_per_mm2"`
	WastePct      float64   `gorm:"column:waste_pct;type:decimal(5,4);not null;default:0.15" json:"waste_pct"`
	SheetCost     *float64  `gorm:"column:sheet_cost;type:decimal(10,2)" json:"sheet_cost,omitempty"`
	SheetWidthMm  *float64  `gorm:"column:sheet_width_mm;type:decimal(8,2)" json:"sheet_width_mm,omitempty"`
	SheetHeightMm *float64  `gorm:"column:sheet_height_mm;type:decimal(8,2)" json:"sheet_height_mm,omitempty"`
	Notes         *string   `gorm:"type:text" json:"notes,omitempty"`
	IsActive      bool      `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Relations
	Material Material `gorm:"foreignKey:MaterialID" json:"material,omitempty"`
}

func (MaterialCost) TableName() string {
	return "material_costs"
}

// CalculateCostPerMm2 calculates cost_per_mm2 from sheet dimensions and cost
func (mc *MaterialCost) CalculateCostPerMm2() float64 {
	if mc.SheetCost == nil || mc.SheetWidthMm == nil || mc.SheetHeightMm == nil {
		return mc.CostPerMm2
	}
	if *mc.SheetWidthMm <= 0 || *mc.SheetHeightMm <= 0 {
		return mc.CostPerMm2
	}
	area := *mc.SheetWidthMm * *mc.SheetHeightMm
	return *mc.SheetCost / area
}
