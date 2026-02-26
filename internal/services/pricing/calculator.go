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
	CostBase     float64 // subtotal before factors (machine cost)
	CostMaterial float64 // DEPRECATED: base × (material factor - 1)
	CostOverhead float64 // overhead calculation

	// Material Cost (Fase 7 - raw material pricing)
	MaterialIncluded    bool    // true if we provide material, false if client provides
	AreaConsumedMM2     float64 // width × height of SVG
	WastePct            float64 // waste percentage applied
	CostMaterialRaw     float64 // area × cost_per_mm2
	CostMaterialWithWaste float64 // raw × (1 + waste_pct)

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
	PriceModel       string  // "hybrid" o "value" — indica cuál modelo determinó el precio final

	// Simulation: What if we apply FactorMaterial to Hybrid?
	SimHybridWithMaterialFactor float64 // What hybrid would be WITH material factor
	SimDifferencePct            float64 // Percentage difference

	// Fallback warning
	UsedFallbackSpeeds bool
	FallbackWarning    string

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
// materialIncluded indicates whether we provide material (true) or client provides (false)
func (c *Calculator) Calculate(
	analysis *models.SVGAnalysis,
	techID uint,
	materialID uint,
	engraveTypeID uint,
	thickness float64,
	quantity int,
	materialIncluded bool,
) (*PriceResult, error) {
	// Load current config from DB
	config, err := c.configLoader.Load()
	if err != nil {
		return nil, err
	}

	result := &PriceResult{
		MaterialIncluded: materialIncluded,
	}

	// =============================================================
	// ESCALAR GEOMETRÍA POR CANTIDAD
	// Multiplicamos la geometría × qty ANTES de calcular.
	// Esto simula "un SVG con todas las piezas" y refleja la operación real:
	// un solo setup, un solo job de máquina, material sobre área total.
	// =============================================================

	// Geometría escalada (Cambio A: TotalArea en vez de Width×Height)
	scaledCutLength := analysis.CutLengthMM * float64(quantity)
	scaledVectorLength := analysis.VectorLengthMM * float64(quantity)
	scaledRasterArea := analysis.RasterAreaMM2 * float64(quantity)
	scaledMaterialArea := analysis.TotalArea() * float64(quantity) // Bounding box real, no canvas

	// Create time estimator with fresh config
	timeEstimator := NewTimeEstimator(config)

	// Calcular tiempo sobre geometría escalada, qty=1
	timeEst := timeEstimator.EstimateWithGeometry(
		scaledRasterArea,
		scaledVectorLength,
		scaledCutLength,
		techID, materialID, engraveTypeID, thickness,
	)

	result.TimeEngraveMins = timeEst.EngraveMins
	result.TimeCutMins = timeEst.CutMins
	result.TimeSetupMins = timeEst.SetupMins
	result.TimeTotalMins = timeEst.TotalMins

	// Set fallback warning if specific speeds not found
	if timeEst.UsedFallback {
		result.UsedFallbackSpeeds = true
		result.FallbackWarning = "Precio estimado con velocidades base. No hay calibración específica para esta combinación tech/material/grosor."
	}

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

	// Calculate base costs (time × rate) - MACHINE COST
	result.CostEngrave = timeEst.EngraveMins * costPerMinEngrave
	result.CostCut = timeEst.CutMins * costPerMinCut
	result.CostSetup = setupFee

	// Base machine cost (without material)
	result.CostBase = result.CostEngrave + result.CostCut

	// =============================================================
	// MATERIAL COST (Fase 7 + Cambio A: bounding box real)
	// area_consumida = TotalArea × qty (bounding box real, no canvas)
	// costo_material = area × cost_per_mm2 × (1 + waste_pct)
	// =============================================================

	result.AreaConsumedMM2 = scaledMaterialArea

	if materialIncluded {
		// Get material cost from DB
		matCost := config.GetMaterialCost(materialID, thickness)

		if matCost.Found && matCost.CostPerMm2 > 0 {
			result.WastePct = matCost.WastePct
			result.CostMaterialRaw = result.AreaConsumedMM2 * matCost.CostPerMm2
			result.CostMaterialWithWaste = result.CostMaterialRaw * (1 + result.WastePct)
		} else {
			// No material cost configured - use default waste but zero cost
			result.WastePct = config.GetDefaultWastePct()
			result.CostMaterialRaw = 0
			result.CostMaterialWithWaste = 0
		}
	} else {
		// Client provides material
		result.WastePct = 0
		result.CostMaterialRaw = 0
		result.CostMaterialWithWaste = 0
	}

	// =============================================================
	// HYBRID PRICING — sobre geometría escalada
	// El costo base YA incluye todas las piezas (geometría × qty)
	// =============================================================

	machineCost := result.CostBase
	materialCost := result.CostMaterialWithWaste

	totalCostBase := machineCost + materialCost

	hybridTotal := totalCostBase
	hybridTotal *= (1 + result.FactorMargin)
	hybridTotal *= result.FactorEngrave
	hybridTotal *= (1 + result.FactorUVPremium)

	// Descuento volumen
	hybridTotal *= (1 - result.DiscountVolumePct)

	// Setup UNA vez
	hybridTotal += result.CostSetup

	result.PriceHybridTotal = math.Round(hybridTotal*100) / 100

	// Unitario es referencia: total / qty
	result.PriceHybridUnit = math.Round((hybridTotal/float64(quantity))*100) / 100

	// =============================================================
	// VALUE-BASED PRICING — sobre área escalada
	// =============================================================

	totalArea := scaledMaterialArea
	minAreaMM2 := config.GetMinAreaMM2()
	if totalArea < minAreaMM2 {
		totalArea = minAreaMM2
	}

	minValueBase := config.GetMinValueBase()
	pricePerMM2 := config.GetPricePerMM2()
	valueBase := math.Max(minValueBase, totalArea*pricePerMM2)
	valueBase *= result.FactorMaterial
	valueBase *= result.FactorEngrave
	valueBase *= (1 + result.FactorUVPremium)

	valueTotal := valueBase
	valueTotal *= (1 - result.DiscountVolumePct)
	valueTotal += result.CostSetup

	result.PriceValueTotal = math.Round(valueTotal*100) / 100
	result.PriceValueUnit = math.Round((valueTotal/float64(quantity))*100) / 100

	// =============================================================
	// PriceFinal = MAX(Hybrid, Value) — protección de piso
	// =============================================================
	if result.PriceHybridTotal >= result.PriceValueTotal {
		result.PriceModel = "hybrid"
	} else {
		result.PriceModel = "value"
	}

	// =============================================================
	// SIMULACIÓN: ¿Qué pasaría si aplicamos FactorMaterial al Hybrid?
	// =============================================================
	simHybridTotal := totalCostBase
	simHybridTotal *= (1 + result.FactorMargin)
	simHybridTotal *= result.FactorEngrave
	simHybridTotal *= (1 + result.FactorUVPremium)
	simHybridTotal *= result.FactorMaterial // <-- CAMBIO SIMULADO
	simHybridTotal *= (1 - result.DiscountVolumePct)
	simHybridTotal += result.CostSetup

	result.SimHybridWithMaterialFactor = math.Round(simHybridTotal*100) / 100
	if result.PriceHybridTotal > 0 {
		result.SimDifferencePct = (result.SimHybridWithMaterialFactor - result.PriceHybridTotal) / result.PriceHybridTotal * 100
	}

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

	// Convert MaterialIncluded to pointer
	materialIncl := result.MaterialIncluded

	// Convert FallbackWarning to pointer (only if not empty)
	var fallbackWarn *string
	if result.FallbackWarning != "" {
		fallbackWarn = &result.FallbackWarning
	}

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
		CostMaterial: result.CostMaterialWithWaste, // Use new material cost
		CostOverhead: result.CostOverhead,

		// Material cost fields (Fase 7)
		MaterialIncluded:      &materialIncl,
		AreaConsumedMM2:       result.AreaConsumedMM2,
		WastePct:              result.WastePct,
		CostMaterialRaw:       result.CostMaterialRaw,
		CostMaterialWithWaste: result.CostMaterialWithWaste,

		FactorMaterial:    result.FactorMaterial,
		FactorEngrave:     result.FactorEngrave,
		FactorUVPremium:   result.FactorUVPremium,
		FactorMargin:      result.FactorMargin,
		DiscountVolumePct: result.DiscountVolumePct,

		PriceHybridUnit:  result.PriceHybridUnit,
		PriceHybridTotal: result.PriceHybridTotal,
		PriceValueUnit:   result.PriceValueUnit,
		PriceValueTotal:  result.PriceValueTotal,
		PriceFinal:       math.Max(result.PriceHybridTotal, result.PriceValueTotal),
		PriceModel:       result.PriceModel,

		// Simulation fields
		SimHybridWithMaterialFactor: result.SimHybridWithMaterialFactor,
		SimDifferencePct:            result.SimDifferencePct,

		// Fallback warning fields
		UsedFallbackSpeeds: result.UsedFallbackSpeeds,
		FallbackWarning:    fallbackWarn,

		Status:     result.Status,
		ValidUntil: validUntil,
	}
}
