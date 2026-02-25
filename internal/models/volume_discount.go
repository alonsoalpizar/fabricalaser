package models

import (
	"time"
)

type VolumeDiscount struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	MinQty      int     `gorm:"not null" json:"min_qty"`
	MaxQty      *int    `json:"max_qty,omitempty"` // NULL = unlimited
	DiscountPct float64 `gorm:"type:decimal(5,4);not null" json:"discount_pct"` // 0.00 - 0.20 (0% - 20%)
	IsActive    bool    `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (VolumeDiscount) TableName() string {
	return "volume_discounts"
}

// GetDiscountForQuantity returns the discount percentage for a given quantity
func GetDiscountForQuantity(discounts []VolumeDiscount, qty int) float64 {
	for _, d := range discounts {
		if qty >= d.MinQty {
			if d.MaxQty == nil || qty <= *d.MaxQty {
				return d.DiscountPct
			}
		}
	}
	return 0
}
