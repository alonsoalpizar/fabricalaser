package models

import (
	"time"

	"gorm.io/datatypes"
)

// QuoteStatus represents the status of a quote
type QuoteStatus string

const (
	QuoteStatusDraft        QuoteStatus = "draft"         // Initial calculation
	QuoteStatusAutoApproved QuoteStatus = "auto_approved" // Simple design, auto-approved
	QuoteStatusNeedsReview  QuoteStatus = "needs_review"  // Complex design, needs admin review
	QuoteStatusRejected     QuoteStatus = "rejected"      // Design cannot be processed
	QuoteStatusApproved     QuoteStatus = "approved"      // Admin approved
	QuoteStatusExpired      QuoteStatus = "expired"       // Quote validity expired
	QuoteStatusConverted    QuoteStatus = "converted"     // Converted to order
)

// Quote represents a pricing quotation for a laser job
type Quote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Source analysis
	SVGAnalysisID uint `gorm:"not null;index" json:"svg_analysis_id"`

	// Selected options (FK to config tables)
	TechnologyID  uint `gorm:"not null" json:"technology_id"`
	MaterialID    uint `gorm:"not null" json:"material_id"`
	EngraveTypeID uint `gorm:"not null" json:"engrave_type_id"`

	// Job parameters
	Quantity  int     `gorm:"not null;default:1" json:"quantity"`
	Thickness float64 `json:"thickness"` // Material thickness in mm (optional)

	// Calculated time estimates (minutes)
	TimeEngraveMins float64 `json:"time_engrave_mins"`
	TimeCutMins     float64 `json:"time_cut_mins"`
	TimeSetupMins   float64 `json:"time_setup_mins"`
	TimeTotalMins   float64 `json:"time_total_mins"`

	// Pricing breakdown (from DB config, NOT hardcoded)
	CostEngrave  float64 `json:"cost_engrave"`  // time × rate
	CostCut      float64 `json:"cost_cut"`      // time × rate
	CostSetup    float64 `json:"cost_setup"`    // setup fee
	CostBase     float64 `json:"cost_base"`     // subtotal before factors
	CostMaterial float64 `json:"cost_material"` // base × material factor
	CostOverhead float64 `json:"cost_overhead"` // overhead rate

	// Factors applied (from DB)
	FactorMaterial    float64 `json:"factor_material"`     // From materials table
	FactorEngrave     float64 `json:"factor_engrave"`      // From engrave_types table
	FactorUVPremium   float64 `json:"factor_uv_premium"`   // From technologies table
	FactorMargin      float64 `json:"factor_margin"`       // From tech_rates table
	DiscountVolumePct float64 `json:"discount_volume_pct"` // From volume_discounts table

	// Final prices (two models)
	PriceHybridUnit  float64 `json:"price_hybrid_unit"`  // Hybrid model: time-based with factors
	PriceHybridTotal float64 `json:"price_hybrid_total"` // × quantity - volume discount
	PriceValueUnit   float64 `json:"price_value_unit"`   // Value model: market-based
	PriceValueTotal  float64 `json:"price_value_total"`  // × quantity - volume discount

	// Admin can select which price to use
	PriceFinal float64 `json:"price_final"` // Final quoted price

	// Adjustments (JSONB for flexibility)
	Adjustments datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"adjustments"` // {reason: string, amount: float, type: "add"|"discount"}

	// Status and workflow
	Status        QuoteStatus `gorm:"type:varchar(20);default:'draft'" json:"status"`
	ReviewNotes   *string     `gorm:"type:text" json:"review_notes,omitempty"`
	ReviewedBy    *uint       `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time  `json:"reviewed_at,omitempty"`
	ValidUntil    time.Time   `json:"valid_until"`              // Quote expiration
	ConvertedToID *uint       `json:"converted_to_id,omitempty"` // Order ID if converted

	// Relations
	User        *User        `gorm:"foreignKey:UserID" json:"-"`
	SVGAnalysis *SVGAnalysis `gorm:"foreignKey:SVGAnalysisID" json:"svg_analysis,omitempty"`
	Technology  *Technology  `gorm:"foreignKey:TechnologyID" json:"technology,omitempty"`
	Material    *Material    `gorm:"foreignKey:MaterialID" json:"material,omitempty"`
	EngraveType *EngraveType `gorm:"foreignKey:EngraveTypeID" json:"engrave_type,omitempty"`
}

func (Quote) TableName() string {
	return "quotes"
}

// IsExpired returns true if the quote has expired
func (q *Quote) IsExpired() bool {
	return time.Now().After(q.ValidUntil)
}

// CanBeConverted returns true if quote can be converted to order
func (q *Quote) CanBeConverted() bool {
	return (q.Status == QuoteStatusAutoApproved || q.Status == QuoteStatusApproved) && !q.IsExpired()
}

// NeedsReview returns true if quote needs admin review
func (q *Quote) NeedsReview() bool {
	return q.Status == QuoteStatusNeedsReview
}

// ToSummary returns quote summary for list views
func (q *Quote) ToSummary() map[string]interface{} {
	return map[string]interface{}{
		"id":                q.ID,
		"svg_analysis_id":   q.SVGAnalysisID,
		"quantity":          q.Quantity,
		"price_hybrid_unit": q.PriceHybridUnit,
		"price_final":       q.PriceFinal,
		"status":            q.Status,
		"valid_until":       q.ValidUntil,
		"created_at":        q.CreatedAt,
	}
}

// ToDetailedJSON returns full quote details for API
func (q *Quote) ToDetailedJSON() map[string]interface{} {
	result := map[string]interface{}{
		"id":         q.ID,
		"user_id":    q.UserID,
		"created_at": q.CreatedAt,

		"svg_analysis_id": q.SVGAnalysisID,
		"technology_id":   q.TechnologyID,
		"material_id":     q.MaterialID,
		"engrave_type_id": q.EngraveTypeID,

		"quantity":  q.Quantity,
		"thickness": q.Thickness,

		"time_breakdown": map[string]interface{}{
			"engrave_mins": q.TimeEngraveMins,
			"cut_mins":     q.TimeCutMins,
			"setup_mins":   q.TimeSetupMins,
			"total_mins":   q.TimeTotalMins,
		},

		"cost_breakdown": map[string]interface{}{
			"engrave":  q.CostEngrave,
			"cut":      q.CostCut,
			"setup":    q.CostSetup,
			"base":     q.CostBase,
			"material": q.CostMaterial,
			"overhead": q.CostOverhead,
		},

		"factors": map[string]interface{}{
			"material":         q.FactorMaterial,
			"engrave":          q.FactorEngrave,
			"uv_premium":       q.FactorUVPremium,
			"margin":           q.FactorMargin,
			"volume_discount":  q.DiscountVolumePct,
		},

		"pricing": map[string]interface{}{
			"hybrid_unit":  q.PriceHybridUnit,
			"hybrid_total": q.PriceHybridTotal,
			"value_unit":   q.PriceValueUnit,
			"value_total":  q.PriceValueTotal,
			"final":        q.PriceFinal,
		},

		"status":      q.Status,
		"valid_until": q.ValidUntil,
	}

	if q.ReviewNotes != nil {
		result["review_notes"] = *q.ReviewNotes
	}
	if q.Technology != nil {
		result["technology"] = q.Technology.Name
	}
	if q.Material != nil {
		result["material"] = q.Material.Name
	}
	if q.EngraveType != nil {
		result["engrave_type"] = q.EngraveType.Name
	}

	return result
}
