package models

import (
	"time"
)

type TechMaterialSpeed struct {
	ID                uint     `gorm:"primaryKey" json:"id"`
	TechnologyID      uint     `gorm:"not null;index" json:"technology_id"`
	MaterialID        uint     `gorm:"not null;index" json:"material_id"`
	Thickness         float64  `gorm:"type:decimal(5,2);not null" json:"thickness"`
	CutSpeedMmMin     *float64 `gorm:"column:cut_speed_mm_min;type:decimal(10,2)" json:"cut_speed_mm_min"`
	// EngraveSpeedMmMin: velocidad cabezal grabado en mm/min (lineal)
	// Para raster se multiplica por spot_size de la tecnología para obtener mm²/min
	EngraveSpeedMmMin *float64 `gorm:"column:engrave_speed_mm_min;type:decimal(10,2)" json:"engrave_speed_mm_min"`
	IsCompatible      bool     `gorm:"default:true" json:"is_compatible"`
	Notes             *string  `gorm:"type:text" json:"notes,omitempty"`
	IsActive          bool     `gorm:"default:true" json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// Relations
	Technology Technology `gorm:"foreignKey:TechnologyID" json:"technology,omitempty"`
	Material   Material   `gorm:"foreignKey:MaterialID" json:"material,omitempty"`
}

func (TechMaterialSpeed) TableName() string {
	return "tech_material_speeds"
}
