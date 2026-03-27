package models

import (
	"time"

	"gorm.io/datatypes"
)

// Blank representa un producto preconfigurado del catálogo de FabricaLaser
// (llaveros, medallas, etc.) que se vende con grabado láser personalizado.
type Blank struct {
	ID          uint           `json:"id"           gorm:"primaryKey"`
	Name        string         `json:"name"         gorm:"not null"`
	Category    string         `json:"category"     gorm:"not null"`
	Description string         `json:"description"`
	Dimensions  *string        `json:"dimensions"`

	// CostPrice: costo de adquisición del blank (uso interno, análisis de margen).
	// NUNCA exponer en respuestas de API pública ni al agente de WhatsApp.
	CostPrice int `json:"cost_price" gorm:"column:cost_price;not null;default:0"`

	// BasePrice: precio de venta unitario al cliente (a min_qty, base antes de price_breaks).
	BasePrice int `json:"base_price" gorm:"column:base_price;not null;default:0"`

	MinQty int `json:"min_qty" gorm:"column:min_qty;not null;default:1"`

	// PriceBreaks: tabla de precios por volumen.
	// Formato: [{"qty": 25, "unit_price": 240}, {"qty": 50, "unit_price": 220}]
	PriceBreaks datatypes.JSON `json:"price_breaks" gorm:"column:price_breaks;type:jsonb;default:'[]'"`

	// Accessories: accesorios opcionales vendidos con el blank.
	// Formato: [{"name": "Argolla metálica", "price": 150, "min_qty_pack": 25}]
	Accessories datatypes.JSON `json:"accessories" gorm:"column:accessories;type:jsonb;default:'[]'"`

	StockQty   int  `json:"stock_qty"   gorm:"column:stock_qty;not null;default:0"`
	StockAlert int  `json:"stock_alert" gorm:"column:stock_alert;not null;default:10"`
	IsFeatured bool `json:"is_featured" gorm:"column:is_featured;default:false"`
	QuoteCount int  `json:"quote_count" gorm:"column:quote_count;default:0"`
	IsActive   bool `json:"is_active"   gorm:"column:is_active;default:true"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
