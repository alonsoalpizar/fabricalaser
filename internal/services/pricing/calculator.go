package pricing

import (
	"math"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
)

// NOTE: Complexity thresholds and quote validity are now loaded from system_config table
// No more hardcoded constants - all values from database

// PriceResult contains all calculated pricing information
type PriceResult struct {
	// Time breakdown
	TimeEngraveMins float64
	TimeCutMins     float64
	TimeSetupMins   float64
	TimeTotalMins   float64

	// Cost breakdown (from DB rates)
	CostEngrave  float64 // time × rate
	CostCut      float64 // time × rate
	CostSetup    float64 // setup fee from tech_rates
	CostBase     float64 // subtotal before factors
	CostMaterial float64 // base × (material factor - 1) (the extra cost)
	CostOverhead float64 // overhead calculation

	// Factors applied (from DB)
	FactorMaterial    float64
	FactorEngrave     float64
	FactorUVPremium   float64
	FactorMargin      float64
	DiscountVolumePct float64

	// Final prices (two models)
	PriceHybridUnit  float64 // Per-unit hybrid price
	PriceHybridTotal float64 // Total with quantity and discount
	PriceValueUnit   float64 // Per-unit value-based price
	PriceValueTotal  float64 // Total with quantity and discount

	// Recommended status
	Status         models.QuoteStatus
	ComplexityNote string
}

// Calculator calculates prices using DB configuration
type Calculator struct {
	configLoader  *ConfigLoader
	timeEstimator *TimeEstimator
}

// NewCalculator creates a calculator with the given config loader
func NewCalculator(configLoader *ConfigLoader) *Calculator {
	return &Calculator{
		configLoader: configLoader,
	}
}

// Calculate computes full pricing for an SVG analysis with given options
// thickness is used to look up specific speeds from tech_material_speeds
func (c *Calculator) Calculate(
	analysis *models.SVGAnalysis,
	techID uint,
	materialID uint,
	engraveTypeID uint,
	thickness float64,
	quantity int,
) (*PriceResult, error) {
	// Load current config from DB
	config, err := c.configLoader.Load()
	if err != nil {
		return nil, err
	}

	result := &PriceResult{}

	// Create time estimator with fresh config
	timeEstimator := NewTimeEstimator(config)

	// Calculate time estimates (now includes thickness for specific speed lookup)
	timeEst := timeEstimator.Estimate(analysis, techID, materialID, engraveTypeID, thickness, quantity)
	result.TimeEngraveMins = timeEst.EngraveMins
	result.TimeCutMins = timeEst.CutMins
	result.TimeSetupMins = timeEst.SetupMins
	result.TimeTotalMins = timeEst.TotalMins

	// Get rates from DB config
	costPerMinEngrave := config.GetCostPerMinEngrave(techID)
	costPerMinCut := config.GetCostPerMinCut(techID)
	setupFee := config.GetSetupFee(techID)
	marginPct := config.GetMarginPercent(techID)

	// Get factors from DB config
	result.FactorMaterial = config.GetMaterialFactor(materialID)
	result.FactorEngrave = config.GetEngraveTypeFactor(engraveTypeID)
	result.FactorUVPremium = config.GetUVPremiumFactor(techID)
	result.FactorMargin = marginPct
	result.DiscountVolumePct = config.GetVolumeDiscount(quantity)

	// Calculate base costs (time × rate)
	result.CostEngrave = timeEst.EngraveMins * costPerMinEngrave
	result.CostCut = timeEst.CutMins * costPerMinCut
	result.CostSetup = setupFee

	// Base cost before factors
	result.CostBase = result.CostEngrave + result.CostCut + result.CostSetup

	// Material cost adjustment
	result.CostMaterial = result.CostBase * (result.FactorMaterial - 1)

	// =============================================================
	// HYBRID PRICING MODEL (from roadmap)
	// Formula: Costo_Base × (1 + margin) × factor_material × factor_grabado × (1 + uv_premium)
	// =============================================================

	// Calculate per-unit price (divide total time costs by quantity for unit price)
	perUnitCostBase := (result.CostEngrave + result.CostCut) / float64(quantity)
	if quantity == 1 {
		perUnitCostBase = result.CostEngrave + result.CostCut
	}

	// Apply factors to get per-unit price
	hybridUnit := perUnitCostBase
	hybridUnit *= (1 + result.FactorMargin)         // Add margin
	hybridUnit *= result.FactorMaterial             // Material factor
	hybridUnit *= result.FactorEngrave              // Engrave type factor
	hybridUnit *= (1 + result.FactorUVPremium)      // UV premium if applicable

	result.PriceHybridUnit = math.Round(hybridUnit*100) / 100

	// Total price = (unit price × quantity) - volume discount + setup
	hybridTotal := result.PriceHybridUnit * float64(quantity)
	hybridTotal *= (1 - result.DiscountVolumePct) // Apply volume discount
	hybridTotal += result.CostSetup               // Add one-time setup

	result.PriceHybridTotal = math.Round(hybridTotal*100) / 100

	// =============================================================
	// VALUE-BASED PRICING MODEL
	// Simpler model based on area/complexity for comparison
	// =============================================================

	// Value model uses area-based pricing with complexity factor
	totalArea := analysis.TotalArea()
	minAreaMM2 := config.GetMinAreaMM2()
	if totalArea < minAreaMM2 {
		totalArea = minAreaMM2 // Minimum area for pricing (from system_config)
	}

	// Base value price: area-based with minimum (valores en colones from system_config)
	minValueBase := config.GetMinValueBase()
	pricePerMM2 := config.GetPricePerMM2()
	valueBase := math.Max(minValueBase, totalArea*pricePerMM2)

	// Apply same factors as hybrid
	valueUnit := valueBase
	valueUnit *= result.FactorMaterial
	valueUnit *= result.FactorEngrave
	valueUnit *= (1 + result.FactorUVPremium)

	result.PriceValueUnit = math.Round(valueUnit*100) / 100

	// Total with quantity and discount
	valueTotal := result.PriceValueUnit * float64(quantity)
	valueTotal *= (1 - result.DiscountVolumePct)
	valueTotal += result.CostSetup

	result.PriceValueTotal = math.Round(valueTotal*100) / 100

	// =============================================================
	// AUTO-APPROVAL CLASSIFICATION
	// Based on design complexity factor (thresholds from system_config)
	// =============================================================

	complexityFactor := analysis.ComplexityFactor()
	complexityAutoApprove := config.GetComplexityAutoApprove()
	complexityNeedsReview := config.GetComplexityNeedsReview()

	if complexityFactor <= complexityAutoApprove {
		result.Status = models.QuoteStatusAutoApproved
		result.ComplexityNote = "Design is simple, auto-approved"
	} else if complexityFactor <= complexityNeedsReview {
		result.Status = models.QuoteStatusNeedsReview
		result.ComplexityNote = "Design complexity requires admin review"
	} else {
		result.Status = models.QuoteStatusRejected
		result.ComplexityNote = "Design is too complex for automated processing"
	}

	return result, nil
}

// ToQuoteModel converts calculation result to a Quote model
func (c *Calculator) ToQuoteModel(
	result *PriceResult,
	userID uint,
	analysisID uint,
	techID uint,
	materialID uint,
	engraveTypeID uint,
	quantity int,
	thickness float64,
) *models.Quote {
	now := time.Now()

	// Get quote validity days from config (with fallback)
	validityDays := 7
	if config, err := c.configLoader.Load(); err == nil {
		validityDays = config.GetQuoteValidityDays()
	}
	validUntil := now.AddDate(0, 0, validityDays)

	return &models.Quote{
		UserID:        userID,
		SVGAnalysisID: analysisID,
		TechnologyID:  techID,
		MaterialID:    materialID,
		EngraveTypeID: engraveTypeID,
		Quantity:      quantity,
		Thickness:     thickness,

		TimeEngraveMins: result.TimeEngraveMins,
		TimeCutMins:     result.TimeCutMins,
		TimeSetupMins:   result.TimeSetupMins,
		TimeTotalMins:   result.TimeTotalMins,

		CostEngrave:  result.CostEngrave,
		CostCut:      result.CostCut,
		CostSetup:    result.CostSetup,
		CostBase:     result.CostBase,
		CostMaterial: result.CostMaterial,
		CostOverhead: result.CostOverhead,

		FactorMaterial:    result.FactorMaterial,
		FactorEngrave:     result.FactorEngrave,
		FactorUVPremium:   result.FactorUVPremium,
		FactorMargin:      result.FactorMargin,
		DiscountVolumePct: result.DiscountVolumePct,

		PriceHybridUnit:  result.PriceHybridUnit,
		PriceHybridTotal: result.PriceHybridTotal,
		PriceValueUnit:   result.PriceValueUnit,
		PriceValueTotal:  result.PriceValueTotal,
		PriceFinal:       result.PriceHybridTotal, // Default to hybrid

		Status:     result.Status,
		ValidUntil: validUntil,
	}
}
