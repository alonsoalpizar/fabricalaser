package pricing

import (
	"sync"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

// PricingConfig holds all configuration needed for pricing calculations
// All values are loaded from the database - NO hardcoded values
type PricingConfig struct {
	TechRates       map[uint]*models.TechRate       // tech_id → rates
	Technologies    map[uint]*models.Technology     // tech_id → tech info
	Materials       map[uint]*models.Material       // material_id → material info
	EngraveTypes    map[uint]*models.EngraveType    // engrave_type_id → engrave info
	VolumeDiscounts []models.VolumeDiscount         // Sorted by min_qty

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
		TechRates:       make(map[uint]*models.TechRate),
		Technologies:    make(map[uint]*models.Technology),
		Materials:       make(map[uint]*models.Material),
		EngraveTypes:    make(map[uint]*models.EngraveType),
		VolumeDiscounts: make([]models.VolumeDiscount, 0),
		LoadedAt:        time.Now(),
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
	return 0.40 // Default 40%
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
