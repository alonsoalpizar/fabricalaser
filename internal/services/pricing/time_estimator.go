package pricing

import (
	"github.com/alonsoalpizar/fabricalaser/internal/models"
)

// NOTE: Base speeds are now loaded from system_config table
// No more hardcoded constants - all values from database

// TimeEstimate contains calculated time estimates
type TimeEstimate struct {
	EngraveMins  float64 // Time for raster + vector engraving (combined, for backwards compat)
	VectorMins   float64 // Time for vector engraving only (blue lines)
	RasterMins   float64 // Time for raster engraving only (black fills)
	CutMins      float64 // Time for cutting (red lines)
	SetupMins    float64 // Setup time (one-time)
	TotalMins    float64 // Total time
	UsedFallback bool    // true if specific speed not found
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

	// Get spot size for the technology (used to convert head speed to raster area speed)
	spotSize := e.config.GetSpotSize(techID)

	// Try to get specific speeds from tech_material_speeds table
	specificSpeed := e.config.GetMaterialSpeed(techID, materialID, thickness)

	// Get base speeds from system_config (used as fallback)
	baseEngraveLineSpeed := e.config.GetBaseEngraveLineSpeed()
	baseCutSpeed := e.config.GetBaseCutSpeed()
	setupTimeMinutes := e.config.GetSetupTimeMinutes()

	// Calculate head engrave speed (mm/min) - same for both raster and vector
	var engraveSpeedMmMin float64
	if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
		engraveSpeedMmMin = *specificSpeed.EngraveSpeedMmMin * speedMult
	} else {
		engraveSpeedMmMin = baseEngraveLineSpeed * speedMult / materialFactor
	}

	// Calculate raster engrave time (area-based)
	// Raster speed = head speed × spot_size (mm²/min)
	// Each pass of the head covers a width equal to the spot size
	if analysis.RasterAreaMM2 > 0 {
		effectiveRasterSpeed := engraveSpeedMmMin * spotSize // mm/min × mm = mm²/min
		rasterTime := analysis.RasterAreaMM2 / effectiveRasterSpeed
		estimate.RasterMins = rasterTime
		estimate.EngraveMins += rasterTime
	}

	// Calculate vector engrave time (line-based)
	// Vector uses head speed directly (mm/min)
	if analysis.VectorLengthMM > 0 {
		vectorTime := analysis.VectorLengthMM / engraveSpeedMmMin
		estimate.VectorMins = vectorTime
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
	estimate.VectorMins *= float64(quantity)
	estimate.RasterMins *= float64(quantity)
	estimate.CutMins *= float64(quantity)

	// Set fallback flag
	estimate.UsedFallback = !specificSpeed.Found

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

// EstimateWithGeometry calcula tiempo con geometría explícita (ya escalada por qty)
// Usa spot_size para convertir velocidad cabezal a velocidad raster automáticamente
func (e *TimeEstimator) EstimateWithGeometry(
	rasterAreaMM2 float64,
	vectorLengthMM float64,
	cutLengthMM float64,
	techID uint,
	materialID uint,
	engraveTypeID uint,
	thickness float64,
) TimeEstimate {
	estimate := TimeEstimate{}

	speedMult := e.config.GetEngraveTypeSpeedMultiplier(engraveTypeID)
	if speedMult <= 0 {
		speedMult = 1.0
	}

	materialFactor := e.config.GetMaterialFactor(materialID)
	if materialFactor <= 0 {
		materialFactor = 1.0
	}

	spotSize := e.config.GetSpotSize(techID)
	specificSpeed := e.config.GetMaterialSpeed(techID, materialID, thickness)

	baseEngraveLineSpeed := e.config.GetBaseEngraveLineSpeed()
	baseCutSpeed := e.config.GetBaseCutSpeed()
	setupTimeMinutes := e.config.GetSetupTimeMinutes()

	// Raster (área): convertir velocidad cabezal a mm²/min con spot_size
	if rasterAreaMM2 > 0 {
		var engraveSpeedMmMin float64
		if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
			engraveSpeedMmMin = *specificSpeed.EngraveSpeedMmMin * speedMult
		} else {
			engraveSpeedMmMin = baseEngraveLineSpeed * speedMult / materialFactor
			estimate.UsedFallback = true
		}
		effectiveRasterSpeed := engraveSpeedMmMin * spotSize
		rasterTime := rasterAreaMM2 / effectiveRasterSpeed
		estimate.RasterMins = rasterTime
		estimate.EngraveMins += rasterTime
	}

	// Vector (líneas): velocidad cabezal directa en mm/min
	if vectorLengthMM > 0 {
		var effectiveVectorSpeed float64
		if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
			effectiveVectorSpeed = *specificSpeed.EngraveSpeedMmMin * speedMult
		} else {
			effectiveVectorSpeed = baseEngraveLineSpeed * speedMult / materialFactor
			estimate.UsedFallback = true
		}
		vectorTime := vectorLengthMM / effectiveVectorSpeed
		estimate.VectorMins = vectorTime
		estimate.EngraveMins += vectorTime
	}

	// Corte
	if cutLengthMM > 0 {
		var effectiveCutSpeed float64
		if specificSpeed.Found && specificSpeed.CutSpeedMmMin != nil && *specificSpeed.CutSpeedMmMin > 0 {
			effectiveCutSpeed = *specificSpeed.CutSpeedMmMin
		} else {
			effectiveCutSpeed = baseCutSpeed / materialFactor
			estimate.UsedFallback = true
		}
		estimate.CutMins = cutLengthMM / effectiveCutSpeed
	}

	// Setup UNA vez (no se multiplica)
	estimate.SetupMins = setupTimeMinutes
	estimate.TotalMins = estimate.SetupMins + estimate.EngraveMins + estimate.CutMins

	return estimate
}

// SpeedInfo returns speed information for display/debugging
type SpeedInfo struct {
	BaseEngraveLineSpeed  float64  // mm/min from system_config
	BaseCutSpeed          float64  // mm/min from system_config
	SpecificEngraveSpeed  *float64 // From tech_material_speeds - head speed (mm/min)
	SpecificCutSpeed      *float64 // From tech_material_speeds (if exists)
	SpotSizeMM            float64  // From technology - laser spot diameter
	EffectiveRasterSpeed  float64  // engraveSpeed × spotSize (mm²/min)
	EffectiveVectorSpeed  float64  // engraveSpeed directly (mm/min)
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

	// Get spot size for the technology
	spotSize := e.config.GetSpotSize(techID)

	// Get base speeds from system_config
	baseEngraveLineSpeed := e.config.GetBaseEngraveLineSpeed()
	baseCutSpeed := e.config.GetBaseCutSpeed()

	// Try to get specific speeds
	specificSpeed := e.config.GetMaterialSpeed(techID, materialID, thickness)

	info := SpeedInfo{
		BaseEngraveLineSpeed: baseEngraveLineSpeed,
		BaseCutSpeed:         baseCutSpeed,
		SpotSizeMM:           spotSize,
		SpeedMultiplier:      speedMult,
		MaterialFactor:       materialFactor,
		HasSpecificSpeed:     specificSpeed.Found,
	}

	// Calculate effective engrave speed (head speed in mm/min)
	var engraveSpeedMmMin float64
	if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil {
		info.SpecificEngraveSpeed = specificSpeed.EngraveSpeedMmMin
		engraveSpeedMmMin = *specificSpeed.EngraveSpeedMmMin * speedMult
	} else {
		engraveSpeedMmMin = baseEngraveLineSpeed * speedMult / materialFactor
	}

	// Raster = head speed × spot size (mm²/min)
	info.EffectiveRasterSpeed = engraveSpeedMmMin * spotSize
	// Vector = head speed directly (mm/min)
	info.EffectiveVectorSpeed = engraveSpeedMmMin

	if specificSpeed.Found && specificSpeed.CutSpeedMmMin != nil {
		info.SpecificCutSpeed = specificSpeed.CutSpeedMmMin
		info.EffectiveCutSpeed = *specificSpeed.CutSpeedMmMin
	} else {
		info.EffectiveCutSpeed = baseCutSpeed / materialFactor
	}

	return info
}
