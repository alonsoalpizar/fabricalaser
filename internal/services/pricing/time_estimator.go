package pricing

import (
	"github.com/alonsoalpizar/fabricalaser/internal/models"
)

// Base speeds for time estimation (these are defaults, actual values come from calculations)
// Speed values depend on material and engrave type
const (
	// Base engrave speed: mm² per minute for raster, mm per minute for vector
	baseEngraveAreaSpeed  = 500.0 // mm²/min for raster engrave (base)
	baseEngraveLineSpeed  = 100.0 // mm/min for vector engrave (base)
	baseCutSpeed          = 20.0  // mm/min for cutting (base)
	setupTimeMinutes      = 5.0   // Base setup time
)

// TimeEstimate contains calculated time estimates
type TimeEstimate struct {
	EngraveMins float64 // Time for raster + vector engraving
	CutMins     float64 // Time for cutting
	SetupMins   float64 // Setup time (one-time)
	TotalMins   float64 // Total time
}

// TimeEstimator calculates processing time based on geometry and config
type TimeEstimator struct {
	config *PricingConfig
}

// NewTimeEstimator creates an estimator with the given config
func NewTimeEstimator(config *PricingConfig) *TimeEstimator {
	return &TimeEstimator{config: config}
}

// Estimate calculates processing time for an analysis with given options
func (e *TimeEstimator) Estimate(
	analysis *models.SVGAnalysis,
	techID uint,
	materialID uint,
	engraveTypeID uint,
	quantity int,
) TimeEstimate {
	estimate := TimeEstimate{}

	// Get speed multiplier from engrave type (affects processing speed)
	speedMult := e.config.GetEngraveTypeSpeedMultiplier(engraveTypeID)
	if speedMult <= 0 {
		speedMult = 1.0
	}

	// Get material factor (harder materials = slower)
	materialFactor := e.config.GetMaterialFactor(materialID)
	if materialFactor <= 0 {
		materialFactor = 1.0
	}

	// Calculate engrave time
	// Raster area engrave: area / (base_speed × speed_multiplier / material_factor)
	if analysis.RasterAreaMM2 > 0 {
		effectiveRasterSpeed := baseEngraveAreaSpeed * speedMult / materialFactor
		rasterTime := analysis.RasterAreaMM2 / effectiveRasterSpeed
		estimate.EngraveMins += rasterTime
	}

	// Vector line engrave: length / (base_speed × speed_multiplier / material_factor)
	if analysis.VectorLengthMM > 0 {
		effectiveVectorSpeed := baseEngraveLineSpeed * speedMult / materialFactor
		vectorTime := analysis.VectorLengthMM / effectiveVectorSpeed
		estimate.EngraveMins += vectorTime
	}

	// Calculate cut time
	// Cut: length / (base_speed / material_factor)
	// Cutting is also affected by material hardness
	if analysis.CutLengthMM > 0 {
		effectiveCutSpeed := baseCutSpeed / materialFactor
		estimate.CutMins = analysis.CutLengthMM / effectiveCutSpeed
	}

	// Setup time (one-time, not multiplied by quantity)
	estimate.SetupMins = setupTimeMinutes

	// Total time for one unit (engrave + cut) multiplied by quantity, plus setup
	perUnitTime := estimate.EngraveMins + estimate.CutMins
	estimate.TotalMins = estimate.SetupMins + (perUnitTime * float64(quantity))

	// Adjust engrave and cut times to reflect per-quantity totals
	estimate.EngraveMins *= float64(quantity)
	estimate.CutMins *= float64(quantity)

	return estimate
}

// EstimatePerUnit calculates time for a single unit (no quantity multiplier)
func (e *TimeEstimator) EstimatePerUnit(
	analysis *models.SVGAnalysis,
	techID uint,
	materialID uint,
	engraveTypeID uint,
) TimeEstimate {
	return e.Estimate(analysis, techID, materialID, engraveTypeID, 1)
}

// GetSpeedInfo returns speed information for display/debugging
type SpeedInfo struct {
	BaseEngraveAreaSpeed  float64 // mm²/min
	BaseEngraveLineSpeed  float64 // mm/min
	BaseCutSpeed          float64 // mm/min
	EffectiveRasterSpeed  float64 // After multipliers
	EffectiveVectorSpeed  float64 // After multipliers
	EffectiveCutSpeed     float64 // After multipliers
	SpeedMultiplier       float64 // From engrave type
	MaterialFactor        float64 // From material
}

// GetSpeedInfo returns detailed speed information for debugging
func (e *TimeEstimator) GetSpeedInfo(materialID, engraveTypeID uint) SpeedInfo {
	speedMult := e.config.GetEngraveTypeSpeedMultiplier(engraveTypeID)
	if speedMult <= 0 {
		speedMult = 1.0
	}

	materialFactor := e.config.GetMaterialFactor(materialID)
	if materialFactor <= 0 {
		materialFactor = 1.0
	}

	return SpeedInfo{
		BaseEngraveAreaSpeed:  baseEngraveAreaSpeed,
		BaseEngraveLineSpeed:  baseEngraveLineSpeed,
		BaseCutSpeed:          baseCutSpeed,
		EffectiveRasterSpeed:  baseEngraveAreaSpeed * speedMult / materialFactor,
		EffectiveVectorSpeed:  baseEngraveLineSpeed * speedMult / materialFactor,
		EffectiveCutSpeed:     baseCutSpeed / materialFactor,
		SpeedMultiplier:       speedMult,
		MaterialFactor:        materialFactor,
	}
}
