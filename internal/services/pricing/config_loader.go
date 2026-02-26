package pricing

import (
	"strconv"
	"sync"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

// PricingConfig holds all configuration needed for pricing calculations
// All values are loaded from the database - NO hardcoded values
type PricingConfig struct {
	TechRates          map[uint]*models.TechRate          // tech_id → rates
	Technologies       map[uint]*models.Technology        // tech_id → tech info
	Materials          map[uint]*models.Material          // material_id → material info
	EngraveTypes       map[uint]*models.EngraveType       // engrave_type_id → engrave info
	VolumeDiscounts    []models.VolumeDiscount            // Sorted by min_qty
	SystemConfigs      map[string]*models.SystemConfig    // config_key → config
	TechMaterialSpeeds []models.TechMaterialSpeed         // All speed configurations
	MaterialCosts      []models.MaterialCost              // Material costs by thickness

	LoadedAt time.Time
}

// ConfigLoader loads and caches pricing configuration from database
type ConfigLoader struct {
	db       *gorm.DB
	cache    *PricingConfig
	cacheTTL time.Duration
	mu       sync.RWMutex
}

// NewConfigLoader creates a loader with the given database connection
func NewConfigLoader(db *gorm.DB) *ConfigLoader {
	return &ConfigLoader{
		db:       db,
		cacheTTL: 5 * time.Minute, // Cache for 5 minutes
	}
}

// Load fetches all pricing configuration from database
// Uses cache if available and not expired
func (l *ConfigLoader) Load() (*PricingConfig, error) {
	l.mu.RLock()
	if l.cache != nil && time.Since(l.cache.LoadedAt) < l.cacheTTL {
		config := l.cache
		l.mu.RUnlock()
		return config, nil
	}
	l.mu.RUnlock()

	// Cache expired or not loaded, fetch from DB
	return l.refresh()
}

// Refresh forces a reload from database
func (l *ConfigLoader) Refresh() (*PricingConfig, error) {
	return l.refresh()
}

// refresh loads all config from database
func (l *ConfigLoader) refresh() (*PricingConfig, error) {
	config := &PricingConfig{
		TechRates:          make(map[uint]*models.TechRate),
		Technologies:       make(map[uint]*models.Technology),
		Materials:          make(map[uint]*models.Material),
		EngraveTypes:       make(map[uint]*models.EngraveType),
		VolumeDiscounts:    make([]models.VolumeDiscount, 0),
		SystemConfigs:      make(map[string]*models.SystemConfig),
		TechMaterialSpeeds: make([]models.TechMaterialSpeed, 0),
		MaterialCosts:      make([]models.MaterialCost, 0),
		LoadedAt:           time.Now(),
	}

	// Load technologies
	var technologies []models.Technology
	if err := l.db.Where("is_active = ?", true).Find(&technologies).Error; err != nil {
		return nil, err
	}
	for i := range technologies {
		config.Technologies[technologies[i].ID] = &technologies[i]
	}

	// Load tech rates
	var techRates []models.TechRate
	if err := l.db.Where("is_active = ?", true).Find(&techRates).Error; err != nil {
		return nil, err
	}
	for i := range techRates {
		config.TechRates[techRates[i].TechnologyID] = &techRates[i]
	}

	// Load materials
	var materials []models.Material
	if err := l.db.Where("is_active = ?", true).Find(&materials).Error; err != nil {
		return nil, err
	}
	for i := range materials {
		config.Materials[materials[i].ID] = &materials[i]
	}

	// Load engrave types
	var engraveTypes []models.EngraveType
	if err := l.db.Where("is_active = ?", true).Find(&engraveTypes).Error; err != nil {
		return nil, err
	}
	for i := range engraveTypes {
		config.EngraveTypes[engraveTypes[i].ID] = &engraveTypes[i]
	}

	// Load volume discounts (sorted by min_qty)
	var volumeDiscounts []models.VolumeDiscount
	if err := l.db.Where("is_active = ?", true).Order("min_qty ASC").Find(&volumeDiscounts).Error; err != nil {
		return nil, err
	}
	config.VolumeDiscounts = volumeDiscounts

	// Load system configs
	var systemConfigs []models.SystemConfig
	if err := l.db.Where("is_active = ?", true).Find(&systemConfigs).Error; err != nil {
		return nil, err
	}
	for i := range systemConfigs {
		config.SystemConfigs[systemConfigs[i].ConfigKey] = &systemConfigs[i]
	}

	// Load tech material speeds (for specific speed lookups AND compatibility checks)
	// NOTE: Changed to load ALL active records (not just compatible ones)
	// This enables IsCompatible() to check and return proper error messages
	var techMaterialSpeeds []models.TechMaterialSpeed
	if err := l.db.Where("is_active = ?", true).Find(&techMaterialSpeeds).Error; err != nil {
		return nil, err
	}
	config.TechMaterialSpeeds = techMaterialSpeeds

	// Load material costs (for raw material pricing)
	var materialCosts []models.MaterialCost
	if err := l.db.Where("is_active = ?", true).Find(&materialCosts).Error; err != nil {
		return nil, err
	}
	config.MaterialCosts = materialCosts

	// Update cache
	l.mu.Lock()
	l.cache = config
	l.mu.Unlock()

	return config, nil
}

// GetTechRate returns the rate for a technology
func (c *PricingConfig) GetTechRate(techID uint) *models.TechRate {
	return c.TechRates[techID]
}

// GetTechnology returns technology info
func (c *PricingConfig) GetTechnology(techID uint) *models.Technology {
	return c.Technologies[techID]
}

// GetMaterial returns material info
func (c *PricingConfig) GetMaterial(materialID uint) *models.Material {
	return c.Materials[materialID]
}

// GetEngraveType returns engrave type info
func (c *PricingConfig) GetEngraveType(engraveTypeID uint) *models.EngraveType {
	return c.EngraveTypes[engraveTypeID]
}

// GetVolumeDiscount returns the applicable discount for a quantity
func (c *PricingConfig) GetVolumeDiscount(quantity int) float64 {
	var discount float64
	for _, vd := range c.VolumeDiscounts {
		if quantity >= vd.MinQty && (vd.MaxQty == nil || quantity <= *vd.MaxQty) {
			discount = vd.DiscountPct
		}
	}
	return discount
}

// GetCostPerMinEngrave returns the cost per minute for engraving
func (c *PricingConfig) GetCostPerMinEngrave(techID uint) float64 {
	if rate := c.TechRates[techID]; rate != nil {
		return rate.CostPerMinEngrave
	}
	return 0
}

// GetCostPerMinCut returns the cost per minute for cutting
func (c *PricingConfig) GetCostPerMinCut(techID uint) float64 {
	if rate := c.TechRates[techID]; rate != nil {
		return rate.CostPerMinCut
	}
	return 0
}

// GetMarginPercent returns the margin percentage for a technology
func (c *PricingConfig) GetMarginPercent(techID uint) float64 {
	if rate := c.TechRates[techID]; rate != nil {
		return rate.MarginPercent
	}
	// Fallback from system_config instead of hardcoded value
	return c.GetSystemConfigFloat("default_margin_percent", 0.40)
}

// GetSetupFee returns the setup fee for a technology
func (c *PricingConfig) GetSetupFee(techID uint) float64 {
	if rate := c.TechRates[techID]; rate != nil {
		return rate.SetupFee
	}
	return 0
}

// GetMaterialFactor returns the pricing factor for a material
func (c *PricingConfig) GetMaterialFactor(materialID uint) float64 {
	if mat := c.Materials[materialID]; mat != nil {
		return mat.Factor
	}
	return 1.0 // Default no adjustment
}

// GetEngraveTypeFactor returns the pricing factor for an engrave type
func (c *PricingConfig) GetEngraveTypeFactor(engraveTypeID uint) float64 {
	if et := c.EngraveTypes[engraveTypeID]; et != nil {
		return et.Factor
	}
	return 1.0 // Default no adjustment
}

// GetEngraveTypeSpeedMultiplier returns the speed multiplier for time calculation
func (c *PricingConfig) GetEngraveTypeSpeedMultiplier(engraveTypeID uint) float64 {
	if et := c.EngraveTypes[engraveTypeID]; et != nil {
		return et.SpeedMultiplier
	}
	return 1.0 // Default no adjustment
}

// GetUVPremiumFactor returns the UV premium factor for a technology
func (c *PricingConfig) GetUVPremiumFactor(techID uint) float64 {
	if tech := c.Technologies[techID]; tech != nil {
		return tech.UVPremiumFactor
	}
	return 0 // Default no premium
}

// =============================================================
// System Config Methods
// =============================================================

// GetSystemConfigString returns a string value from system_config
func (c *PricingConfig) GetSystemConfigString(key string) string {
	if cfg := c.SystemConfigs[key]; cfg != nil {
		return cfg.ConfigValue
	}
	return ""
}

// GetSystemConfigFloat returns a float64 value from system_config
func (c *PricingConfig) GetSystemConfigFloat(key string, defaultVal float64) float64 {
	if cfg := c.SystemConfigs[key]; cfg != nil {
		if val, err := strconv.ParseFloat(cfg.ConfigValue, 64); err == nil {
			return val
		}
	}
	return defaultVal
}

// GetSystemConfigInt returns an int value from system_config
func (c *PricingConfig) GetSystemConfigInt(key string, defaultVal int) int {
	if cfg := c.SystemConfigs[key]; cfg != nil {
		if val, err := strconv.Atoi(cfg.ConfigValue); err == nil {
			return val
		}
	}
	return defaultVal
}

// =============================================================
// Base Speeds from System Config
// =============================================================

// GetBaseEngraveAreaSpeed returns base engrave area speed from system_config
func (c *PricingConfig) GetBaseEngraveAreaSpeed() float64 {
	return c.GetSystemConfigFloat("base_engrave_area_speed", 500.0)
}

// GetBaseEngraveLineSpeed returns base engrave line speed from system_config
func (c *PricingConfig) GetBaseEngraveLineSpeed() float64 {
	return c.GetSystemConfigFloat("base_engrave_line_speed", 100.0)
}

// GetBaseCutSpeed returns base cut speed from system_config
func (c *PricingConfig) GetBaseCutSpeed() float64 {
	return c.GetSystemConfigFloat("base_cut_speed", 20.0)
}

// GetSetupTimeMinutes returns setup time from system_config
func (c *PricingConfig) GetSetupTimeMinutes() float64 {
	return c.GetSystemConfigFloat("setup_time_minutes", 5.0)
}

// GetComplexityAutoApprove returns complexity threshold for auto-approval
func (c *PricingConfig) GetComplexityAutoApprove() float64 {
	return c.GetSystemConfigFloat("complexity_auto_approve", 6.0)
}

// GetComplexityNeedsReview returns complexity threshold for review
func (c *PricingConfig) GetComplexityNeedsReview() float64 {
	return c.GetSystemConfigFloat("complexity_needs_review", 12.0)
}

// GetQuoteValidityDays returns quote validity in days
func (c *PricingConfig) GetQuoteValidityDays() int {
	return c.GetSystemConfigInt("quote_validity_days", 7)
}

// GetMinValueBase returns minimum value base price in CRC
func (c *PricingConfig) GetMinValueBase() float64 {
	return c.GetSystemConfigFloat("min_value_base", 2575.0)
}

// GetPricePerMM2 returns price per mm² in CRC
func (c *PricingConfig) GetPricePerMM2() float64 {
	return c.GetSystemConfigFloat("price_per_mm2", 0.515)
}

// GetMinAreaMM2 returns minimum area for pricing
func (c *PricingConfig) GetMinAreaMM2() float64 {
	return c.GetSystemConfigFloat("min_area_mm2", 100.0)
}

// =============================================================
// Tech Material Speed Methods
// =============================================================

// TechMaterialSpeedResult holds speed info for a specific combination
type TechMaterialSpeedResult struct {
	CutSpeedMmMin     *float64
	EngraveSpeedMmMin *float64 // Velocidad cabezal (mm/min) - para raster se multiplica por spot_size
	Found             bool
}

// GetMaterialSpeed returns the specific speed for a tech/material/thickness combination
// Returns nil speeds if no specific configuration exists (use base speeds as fallback)
func (c *PricingConfig) GetMaterialSpeed(techID, materialID uint, thickness float64) TechMaterialSpeedResult {
	for _, s := range c.TechMaterialSpeeds {
		if s.TechnologyID == techID && s.MaterialID == materialID && s.Thickness == thickness {
			return TechMaterialSpeedResult{
				CutSpeedMmMin:     s.CutSpeedMmMin,
				EngraveSpeedMmMin: s.EngraveSpeedMmMin,
				Found:             true,
			}
		}
	}
	// Try with thickness 0 (for materials without specific thickness)
	if thickness != 0 {
		for _, s := range c.TechMaterialSpeeds {
			if s.TechnologyID == techID && s.MaterialID == materialID && s.Thickness == 0 {
				return TechMaterialSpeedResult{
					CutSpeedMmMin:     s.CutSpeedMmMin,
					EngraveSpeedMmMin: s.EngraveSpeedMmMin,
					Found:             true,
				}
			}
		}
	}
	return TechMaterialSpeedResult{Found: false}
}

// GetSpotSize returns the laser spot size for a technology
// Used to convert head speed (mm/min) to raster area speed (mm²/min)
func (c *PricingConfig) GetSpotSize(techID uint) float64 {
	if tech := c.Technologies[techID]; tech != nil && tech.SpotSizeMM > 0 {
		return tech.SpotSizeMM
	}
	return 0.1 // Default CO2 spot size
}

// =============================================================
// Material Cost Methods
// =============================================================

// MaterialCostResult holds material cost info for a specific combination
type MaterialCostResult struct {
	CostPerMm2 float64
	WastePct   float64
	Found      bool
}

// GetMaterialCost returns the material cost for a material/thickness combination
// Returns zero cost if no specific configuration exists (client provides material)
func (c *PricingConfig) GetMaterialCost(materialID uint, thickness float64) MaterialCostResult {
	for _, mc := range c.MaterialCosts {
		if mc.MaterialID == materialID && mc.Thickness == thickness {
			return MaterialCostResult{
				CostPerMm2: mc.CostPerMm2,
				WastePct:   mc.WastePct,
				Found:      true,
			}
		}
	}
	// Try with thickness 0 (for materials without specific thickness)
	if thickness != 0 {
		for _, mc := range c.MaterialCosts {
			if mc.MaterialID == materialID && mc.Thickness == 0 {
				return MaterialCostResult{
					CostPerMm2: mc.CostPerMm2,
					WastePct:   mc.WastePct,
					Found:      true,
				}
			}
		}
	}
	// Default: no material cost (client provides)
	return MaterialCostResult{
		CostPerMm2: 0,
		WastePct:   0,
		Found:      false,
	}
}

// GetDefaultWastePct returns the default waste percentage from system_config
func (c *PricingConfig) GetDefaultWastePct() float64 {
	return c.GetSystemConfigFloat("default_waste_pct", 0.15)
}

// =============================================================
// Tech×Material Compatibility Methods
// =============================================================

// IsCompatible checks if a technology can work with a material
// Returns: compatible bool, reason string (from notes field if incompatible)
// If no specific record exists, assumes compatible (will use fallback speeds)
func (c *PricingConfig) IsCompatible(techID, materialID uint, thickness float64) (bool, string) {
	// First try exact thickness match
	for _, s := range c.TechMaterialSpeeds {
		if s.TechnologyID == techID && s.MaterialID == materialID && s.Thickness == thickness {
			if !s.IsCompatible {
				reason := "Combinación tecnología/material no compatible"
				if s.Notes != nil && *s.Notes != "" {
					reason = *s.Notes
				}
				return false, reason
			}
			return true, ""
		}
	}

	// Try thickness 0 (generic material entry)
	for _, s := range c.TechMaterialSpeeds {
		if s.TechnologyID == techID && s.MaterialID == materialID && s.Thickness == 0 {
			if !s.IsCompatible {
				reason := "Combinación tecnología/material no compatible"
				if s.Notes != nil && *s.Notes != "" {
					reason = *s.Notes
				}
				return false, reason
			}
			return true, ""
		}
	}

	// No specific record found - assume compatible (uses fallback speeds)
	return true, ""
}
