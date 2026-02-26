package pricing

import (
	"github.com/alonsoalpizar/fabricalaser/internal/models"
)

// NOTE: Base speeds are now loaded from system_config table
// No more hardcoded constants - all values from database

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
// thickness parameter is used to look up specific speeds from tech_material_speeds table
func (e *TimeEstimator) Estimate(
	analysis *models.SVGAnalysis,
	techID uint,
	materialID uint,
	engraveTypeID uint,
	thickness float64,
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

	// Try to get specific speeds from tech_material_speeds table
	specificSpeed := e.config.GetMaterialSpeed(techID, materialID, thickness)

	// Get base speeds from system_config (used as fallback)
	baseEngraveAreaSpeed := e.config.GetBaseEngraveAreaSpeed()
	baseEngraveLineSpeed := e.config.GetBaseEngraveLineSpeed()
	baseCutSpeed := e.config.GetBaseCutSpeed()
	setupTimeMinutes := e.config.GetSetupTimeMinutes()

	// Calculate engrave time
	// If specific engrave speed exists, use it; otherwise use base with multipliers
	if analysis.RasterAreaMM2 > 0 {
		var effectiveRasterSpeed float64
		if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
			// Use specific speed (already calibrated for this combination)
			effectiveRasterSpeed = *specificSpeed.EngraveSpeedMmMin * speedMult
		} else {
			// Fallback to base speed with material factor
			effectiveRasterSpeed = baseEngraveAreaSpeed * speedMult / materialFactor
		}
		rasterTime := analysis.RasterAreaMM2 / effectiveRasterSpeed
		estimate.EngraveMins += rasterTime
	}

	// Vector line engrave
	if analysis.VectorLengthMM > 0 {
		var effectiveVectorSpeed float64
		if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
			effectiveVectorSpeed = *specificSpeed.EngraveSpeedMmMin * speedMult
		} else {
			effectiveVectorSpeed = baseEngraveLineSpeed * speedMult / materialFactor
		}
		vectorTime := analysis.VectorLengthMM / effectiveVectorSpeed
		estimate.EngraveMins += vectorTime
	}

	// Calculate cut time
	// If specific cut speed exists, use it; otherwise use base with material factor
	if analysis.CutLengthMM > 0 {
		var effectiveCutSpeed float64
		if specificSpeed.Found && specificSpeed.CutSpeedMmMin != nil && *specificSpeed.CutSpeedMmMin > 0 {
			// Use specific speed (already calibrated for thickness)
			effectiveCutSpeed = *specificSpeed.CutSpeedMmMin
		} else {
			// Fallback to base speed with material factor
			effectiveCutSpeed = baseCutSpeed / materialFactor
		}
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
	thickness float64,
) TimeEstimate {
	return e.Estimate(analysis, techID, materialID, engraveTypeID, thickness, 1)
}

// SpeedInfo returns speed information for display/debugging
type SpeedInfo struct {
	BaseEngraveAreaSpeed  float64  // mm²/min from system_config
	BaseEngraveLineSpeed  float64  // mm/min from system_config
	BaseCutSpeed          float64  // mm/min from system_config
	SpecificCutSpeed      *float64 // From tech_material_speeds (if exists)
	SpecificEngraveSpeed  *float64 // From tech_material_speeds (if exists)
	EffectiveRasterSpeed  float64  // After multipliers
	EffectiveVectorSpeed  float64  // After multipliers
	EffectiveCutSpeed     float64  // After multipliers
	SpeedMultiplier       float64  // From engrave type
	MaterialFactor        float64  // From material
	HasSpecificSpeed      bool     // True if specific speed found in tech_material_speeds
}

// GetSpeedInfo returns detailed speed information for debugging
func (e *TimeEstimator) GetSpeedInfo(techID, materialID, engraveTypeID uint, thickness float64) SpeedInfo {
	speedMult := e.config.GetEngraveTypeSpeedMultiplier(engraveTypeID)
	if speedMult <= 0 {
		speedMult = 1.0
	}

	materialFactor := e.config.GetMaterialFactor(materialID)
	if materialFactor <= 0 {
		materialFactor = 1.0
	}

	// Get base speeds from system_config
	baseEngraveAreaSpeed := e.config.GetBaseEngraveAreaSpeed()
	baseEngraveLineSpeed := e.config.GetBaseEngraveLineSpeed()
	baseCutSpeed := e.config.GetBaseCutSpeed()

	// Try to get specific speeds
	specificSpeed := e.config.GetMaterialSpeed(techID, materialID, thickness)

	info := SpeedInfo{
		BaseEngraveAreaSpeed: baseEngraveAreaSpeed,
		BaseEngraveLineSpeed: baseEngraveLineSpeed,
		BaseCutSpeed:         baseCutSpeed,
		SpeedMultiplier:      speedMult,
		MaterialFactor:       materialFactor,
		HasSpecificSpeed:     specificSpeed.Found,
	}

	// Calculate effective speeds
	if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil {
		info.SpecificEngraveSpeed = specificSpeed.EngraveSpeedMmMin
		info.EffectiveRasterSpeed = *specificSpeed.EngraveSpeedMmMin * speedMult
		info.EffectiveVectorSpeed = *specificSpeed.EngraveSpeedMmMin * speedMult
	} else {
		info.EffectiveRasterSpeed = baseEngraveAreaSpeed * speedMult / materialFactor
		info.EffectiveVectorSpeed = baseEngraveLineSpeed * speedMult / materialFactor
	}

	if specificSpeed.Found && specificSpeed.CutSpeedMmMin != nil {
		info.SpecificCutSpeed = specificSpeed.CutSpeedMmMin
		info.EffectiveCutSpeed = *specificSpeed.CutSpeedMmMin
	} else {
		info.EffectiveCutSpeed = baseCutSpeed / materialFactor
	}

	return info
}
