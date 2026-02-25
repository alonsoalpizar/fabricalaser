package models

import (
	"time"
)

type EngraveType struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	Name            string  `gorm:"type:varchar(50);not null" json:"name"` // vectorial, rasterizado, fotograbado, 3d_relieve
	Factor          float64 `gorm:"type:decimal(5,4);default:1.0" json:"factor"` // 1.0 - 3.0
	SpeedMultiplier float64 `gorm:"type:decimal(5,4);default:1.0" json:"speed_multiplier"` // Relative speed
	Description     *string `gorm:"type:text" json:"description,omitempty"`
	IsActive        bool    `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (EngraveType) TableName() string {
	return "engrave_types"
}
