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
	TimeEngraveMins float64 // Combined engrave time (vector + raster)
	TimeVectorMins  float64 // Vector engrave time only (blue lines)
	TimeRasterMins  float64 // Raster engrave time only (black fills)
	TimeCutMins     float64 // Cut time (red lines)
	TimeSetupMins   float64
	TimeTotalMins   float64

	// Cost breakdown (from DB rates)
	CostEngrave  float64 // time × rate
	CostCut      float64 // time × rate
	CostSetup    float64 // setup fee from tech_rates
	CostBase     float64 // subtotal before factors (machine cost)
	CostMaterial float64 // Alias de CostMaterialWithWaste — mantenido por compatibilidad
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
	PriceHybridUnit    float64 // Per-unit hybrid price
	PriceHybridTotal   float64 // Total with quantity and discount
	PriceValueUnit     float64 // Per-unit value-based price
	PriceValueTotal    float64 // Total with quantity and discount
	PriceModel         string  // "hybrid" o "value" — indica cuál modelo determinó el precio final
	PriceModelDetail   string  // "area" o "perimeter" — detalle del modelo value

	// Simulation: What if we apply FactorMaterial to Hybrid?
	SimHybridWithMaterialFactor float64 // What hybrid would be WITH material factor
	SimDifferencePct            float64 // Percentage difference

	// Cut technology (when different from main engrave tech)
	CutTechnologyID *uint // nil = misma tech principal

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
	cutTechnologyID *uint, // nil = usar techID para corte
	ignoreCutLines bool,   // true = ignorar líneas de corte (material no cortable)
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

	// Ignorar líneas de corte si material no es cortable
	if ignoreCutLines {
		scaledCutLength = 0
	}
	result.CutTechnologyID = cutTechnologyID

	// Create time estimator with fresh config
	timeEstimator := NewTimeEstimator(config)

	// Calcular tiempo sobre geometría escalada, qty=1
	// Si cutTechnologyID es diferente a techID, dos estimaciones separadas
	var timeEst TimeEstimate
	if cutTechnologyID != nil && *cutTechnologyID != techID {
		engraveEst := timeEstimator.EstimateWithGeometry(
			scaledRasterArea, scaledVectorLength, 0,
			techID, materialID, engraveTypeID, thickness,
		)
		cutEst := timeEstimator.EstimateWithGeometry(
			0, 0, scaledCutLength,
			*cutTechnologyID, materialID, engraveTypeID, thickness,
		)
		timeEst = TimeEstimate{
			EngraveMins:  engraveEst.EngraveMins,
			VectorMins:   engraveEst.VectorMins,
			RasterMins:   engraveEst.RasterMins,
			CutMins:      cutEst.CutMins,
			SetupMins:    engraveEst.SetupMins,
			TotalMins:    engraveEst.TotalMins + cutEst.CutMins,
			UsedFallback: engraveEst.UsedFallback || cutEst.UsedFallback,
		}
	} else {
		timeEst = timeEstimator.EstimateWithGeometry(
			scaledRasterArea, scaledVectorLength, scaledCutLength,
			techID, materialID, engraveTypeID, thickness,
		)
	}

	result.TimeEngraveMins = timeEst.EngraveMins
	result.TimeVectorMins = timeEst.VectorMins
	result.TimeRasterMins = timeEst.RasterMins
	result.TimeCutMins = timeEst.CutMins
	result.TimeSetupMins = timeEst.SetupMins
	result.TimeTotalMins = timeEst.TotalMins

	// Set fallback warning if specific speeds not found
	if timeEst.UsedFallback {
		result.UsedFallbackSpeeds = true
		result.FallbackWarning = "Precio estimado con velocidades base. No hay calibración específica para esta combinación tech/material/grosor."
	}

	// Get rates from DB config
	// Para corte, usar cutTechID si hay tecnología de corte separada
	cutTechID := techID
	if cutTechnologyID != nil {
		cutTechID = *cutTechnologyID
	}
	costPerMinEngrave := config.GetCostPerMinEngrave(techID)
	costPerMinCut := config.GetCostPerMinCut(cutTechID)
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

	if cutTechnologyID != nil && *cutTechnologyID != techID {
		// Dos tecnologías: márgenes separados por tech, setups sumados
		marginEngrave := config.GetMarginPercent(techID)
		marginCut := config.GetMarginPercent(cutTechID)

		costEngraveBase := (result.CostEngrave + materialCost) *
			(1 + marginEngrave) * result.FactorEngrave * (1 + result.FactorUVPremium)
		costCutBase := result.CostCut * (1 + marginCut)

		// Setup de ambas máquinas
		result.CostSetup += config.GetSetupFee(cutTechID)

		hybridTotal := (costEngraveBase + costCutBase) * (1 - result.DiscountVolumePct) + result.CostSetup
		result.PriceHybridTotal = math.Round(hybridTotal*100) / 100
		result.PriceHybridUnit = math.Round((hybridTotal/float64(quantity))*100) / 100
	} else {
		hybridTotal := totalCostBase
		hybridTotal *= (1 + result.FactorMargin)
		hybridTotal *= result.FactorEngrave
		hybridTotal *= (1 + result.FactorUVPremium)
		hybridTotal *= (1 - result.DiscountVolumePct)
		hybridTotal += result.CostSetup
		result.PriceHybridTotal = math.Round(hybridTotal*100) / 100
		result.PriceHybridUnit = math.Round((hybridTotal/float64(quantity))*100) / 100
	}

	// =============================================================
	// VALUE-BASED PRICING — adaptativo por tipo de trabajo
	// Solo corte → usa perímetro × price_per_mm_cut
	// Con grabado → usa área × price_per_mm2 (como antes)
	// =============================================================

	var valueBase float64

	hasEngrave := scaledRasterArea > 0 || scaledVectorLength > 0
	isOnlyCut := !hasEngrave && scaledCutLength > 0

	minValueBase := config.GetMinValueBase()

	if isOnlyCut {
		// Solo corte: valor por perímetro
		pricePerMmCut := config.GetPricePerMmCut()
		valueBase = math.Max(minValueBase, scaledCutLength*pricePerMmCut)
		result.PriceModelDetail = "perimeter"
	} else {
		// Con grabado: valor por área grabada real (raster + vector), no por canvas.
		// El canvas es relevante para costo de material, no para el servicio de grabado.
		// Si el diseño ocupa 1% del canvas, no se cobra el 100% del canvas.
		workArea := scaledRasterArea + scaledVectorLength*0.5 // vector contribuye 50% como área equiv.
		if workArea <= 0 {
			workArea = scaledMaterialArea // fallback si no hay geometría medible
		}
		minAreaMM2 := config.GetMinAreaMM2()
		if workArea < minAreaMM2 {
			workArea = minAreaMM2
		}
		pricePerMM2 := config.GetPricePerMM2()
		valueBase = math.Max(minValueBase, workArea*pricePerMM2)
		result.PriceModelDetail = "area"
	}

	// Aplicar factores (igual para ambos casos)
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
	cutTechnologyID *uint,
	ignoreCutLines bool,
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
		TechnologyID:    techID,
		MaterialID:      materialID,
		EngraveTypeID:   engraveTypeID,
		CutTechnologyID: cutTechnologyID,
		IgnoreCutLines:  ignoreCutLines,
		Quantity:        quantity,
		Thickness:       thickness,

		TimeEngraveMins: result.TimeEngraveMins,
		TimeVectorMins:  result.TimeVectorMins,
		TimeRasterMins:  result.TimeRasterMins,
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
		PriceModelDetail: result.PriceModelDetail,

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
