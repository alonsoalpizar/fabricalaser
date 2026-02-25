package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/go-chi/chi/v5"
	"gorm.io/datatypes"
)

type AdminHandler struct {
	techRepo      *repository.TechnologyRepository
	materialRepo  *repository.MaterialRepository
	engraveRepo   *repository.EngraveTypeRepository
	rateRepo      *repository.TechRateRepository
	discountRepo  *repository.VolumeDiscountRepository
	priceRefRepo  *repository.PriceReferenceRepository
	userRepo      *repository.UserRepository
}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{
		techRepo:      repository.NewTechnologyRepository(),
		materialRepo:  repository.NewMaterialRepository(),
		engraveRepo:   repository.NewEngraveTypeRepository(),
		rateRepo:      repository.NewTechRateRepository(),
		discountRepo:  repository.NewVolumeDiscountRepository(),
		priceRefRepo:  repository.NewPriceReferenceRepository(),
		userRepo:      repository.NewUserRepository(),
	}
}

// ==================== TECHNOLOGIES ====================

func (h *AdminHandler) CreateTechnology(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code            string  `json:"code"`
		Name            string  `json:"name"`
		Description     string  `json:"description"`
		UVPremiumFactor float64 `json:"uv_premium_factor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Code == "" || req.Name == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Código y nombre son requeridos")
		return
	}

	tech := &models.Technology{
		Code:            req.Code,
		Name:            req.Name,
		UVPremiumFactor: req.UVPremiumFactor,
		IsActive:        true,
	}
	if req.Description != "" {
		tech.Description = &req.Description
	}

	if err := h.techRepo.Create(tech); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear tecnología")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    tech,
	})
}

func (h *AdminHandler) UpdateTechnology(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	tech, err := h.techRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Tecnología no encontrada")
		return
	}

	var req struct {
		Name            string  `json:"name"`
		Description     string  `json:"description"`
		UVPremiumFactor float64 `json:"uv_premium_factor"`
		IsActive        *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Name != "" {
		tech.Name = req.Name
	}
	if req.Description != "" {
		tech.Description = &req.Description
	}
	tech.UVPremiumFactor = req.UVPremiumFactor
	if req.IsActive != nil {
		tech.IsActive = *req.IsActive
	}

	if err := h.techRepo.Update(tech); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar tecnología")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    tech,
	})
}

func (h *AdminHandler) DeleteTechnology(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	if err := h.techRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar tecnología")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tecnología eliminada",
	})
}

// ==================== MATERIALS ====================

func (h *AdminHandler) CreateMaterial(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Category    string  `json:"category"`
		Factor      float64 `json:"factor"`
		Thicknesses []int   `json:"thicknesses"`
		Notes       string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Name == "" || req.Category == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Nombre y categoría son requeridos")
		return
	}

	thicknessJSON, _ := json.Marshal(req.Thicknesses)

	material := &models.Material{
		Name:        req.Name,
		Category:    req.Category,
		Factor:      req.Factor,
		Thicknesses: datatypes.JSON(thicknessJSON),
		IsActive:    true,
	}
	if req.Notes != "" {
		material.Notes = &req.Notes
	}

	if err := h.materialRepo.Create(material); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear material")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    material,
	})
}

func (h *AdminHandler) UpdateMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	material, err := h.materialRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Material no encontrado")
		return
	}

	var req struct {
		Name        string  `json:"name"`
		Category    string  `json:"category"`
		Factor      float64 `json:"factor"`
		Thicknesses []int   `json:"thicknesses"`
		Notes       string  `json:"notes"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Name != "" {
		material.Name = req.Name
	}
	if req.Category != "" {
		material.Category = req.Category
	}
	if req.Factor > 0 {
		material.Factor = req.Factor
	}
	if req.Thicknesses != nil {
		thicknessJSON, _ := json.Marshal(req.Thicknesses)
		material.Thicknesses = datatypes.JSON(thicknessJSON)
	}
	if req.Notes != "" {
		material.Notes = &req.Notes
	}
	if req.IsActive != nil {
		material.IsActive = *req.IsActive
	}

	if err := h.materialRepo.Update(material); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar material")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    material,
	})
}

func (h *AdminHandler) DeleteMaterial(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	if err := h.materialRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar material")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Material eliminado",
	})
}

// ==================== ENGRAVE TYPES ====================

func (h *AdminHandler) CreateEngraveType(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string  `json:"name"`
		Factor          float64 `json:"factor"`
		SpeedMultiplier float64 `json:"speed_multiplier"`
		Description     string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Nombre es requerido")
		return
	}

	engraveType := &models.EngraveType{
		Name:            req.Name,
		Factor:          req.Factor,
		SpeedMultiplier: req.SpeedMultiplier,
		IsActive:        true,
	}
	if req.Description != "" {
		engraveType.Description = &req.Description
	}

	if err := h.engraveRepo.Create(engraveType); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear tipo de grabado")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    engraveType,
	})
}

func (h *AdminHandler) UpdateEngraveType(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	engraveType, err := h.engraveRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Tipo de grabado no encontrado")
		return
	}

	var req struct {
		Name            string  `json:"name"`
		Factor          float64 `json:"factor"`
		SpeedMultiplier float64 `json:"speed_multiplier"`
		Description     string  `json:"description"`
		IsActive        *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Name != "" {
		engraveType.Name = req.Name
	}
	if req.Factor > 0 {
		engraveType.Factor = req.Factor
	}
	if req.SpeedMultiplier > 0 {
		engraveType.SpeedMultiplier = req.SpeedMultiplier
	}
	if req.Description != "" {
		engraveType.Description = &req.Description
	}
	if req.IsActive != nil {
		engraveType.IsActive = *req.IsActive
	}

	if err := h.engraveRepo.Update(engraveType); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar tipo de grabado")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    engraveType,
	})
}

func (h *AdminHandler) DeleteEngraveType(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	if err := h.engraveRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar tipo de grabado")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tipo de grabado eliminado",
	})
}

// ==================== TECH RATES ====================

func (h *AdminHandler) UpdateTechRate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	rate, err := h.rateRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Tarifa no encontrada")
		return
	}

	var req struct {
		EngraveRateHour   float64 `json:"engrave_rate_hour"`
		CutRateHour       float64 `json:"cut_rate_hour"`
		DesignRateHour    float64 `json:"design_rate_hour"`
		OverheadRateHour  float64 `json:"overhead_rate_hour"`
		SetupFee          float64 `json:"setup_fee"`
		CostPerMinEngrave float64 `json:"cost_per_min_engrave"`
		CostPerMinCut     float64 `json:"cost_per_min_cut"`
		MarginPercent     float64 `json:"margin_percent"`
		IsActive          *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.EngraveRateHour > 0 {
		rate.EngraveRateHour = req.EngraveRateHour
	}
	if req.CutRateHour > 0 {
		rate.CutRateHour = req.CutRateHour
	}
	if req.DesignRateHour > 0 {
		rate.DesignRateHour = req.DesignRateHour
	}
	if req.OverheadRateHour > 0 {
		rate.OverheadRateHour = req.OverheadRateHour
	}
	rate.SetupFee = req.SetupFee
	if req.CostPerMinEngrave > 0 {
		rate.CostPerMinEngrave = req.CostPerMinEngrave
	}
	if req.CostPerMinCut > 0 {
		rate.CostPerMinCut = req.CostPerMinCut
	}
	if req.MarginPercent > 0 {
		rate.MarginPercent = req.MarginPercent
	}
	if req.IsActive != nil {
		rate.IsActive = *req.IsActive
	}

	if err := h.rateRepo.Update(rate); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar tarifa")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    rate,
	})
}

// ==================== VOLUME DISCOUNTS ====================

func (h *AdminHandler) CreateVolumeDiscount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MinQty      int     `json:"min_qty"`
		MaxQty      *int    `json:"max_qty"`
		DiscountPct float64 `json:"discount_pct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	discount := &models.VolumeDiscount{
		MinQty:      req.MinQty,
		MaxQty:      req.MaxQty,
		DiscountPct: req.DiscountPct,
		IsActive:    true,
	}

	if err := h.discountRepo.Create(discount); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear descuento")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    discount,
	})
}

func (h *AdminHandler) UpdateVolumeDiscount(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	discount, err := h.discountRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Descuento no encontrado")
		return
	}

	var req struct {
		MinQty      *int    `json:"min_qty"`
		MaxQty      *int    `json:"max_qty"`
		DiscountPct float64 `json:"discount_pct"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.MinQty != nil {
		discount.MinQty = *req.MinQty
	}
	discount.MaxQty = req.MaxQty
	if req.DiscountPct > 0 {
		discount.DiscountPct = req.DiscountPct
	}
	if req.IsActive != nil {
		discount.IsActive = *req.IsActive
	}

	if err := h.discountRepo.Update(discount); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar descuento")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    discount,
	})
}

func (h *AdminHandler) DeleteVolumeDiscount(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	if err := h.discountRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar descuento")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Descuento eliminado",
	})
}

// ==================== PRICE REFERENCES ====================

func (h *AdminHandler) CreatePriceReference(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceType string  `json:"service_type"`
		MinUSD      float64 `json:"min_usd"`
		MaxUSD      float64 `json:"max_usd"`
		TypicalTime string  `json:"typical_time"`
		Description string  `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.ServiceType == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Tipo de servicio es requerido")
		return
	}

	ref := &models.PriceReference{
		ServiceType: req.ServiceType,
		MinUSD:      req.MinUSD,
		MaxUSD:      req.MaxUSD,
		TypicalTime: req.TypicalTime,
		IsActive:    true,
	}
	if req.Description != "" {
		ref.Description = &req.Description
	}

	if err := h.priceRefRepo.Create(ref); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear referencia de precio")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    ref,
	})
}

func (h *AdminHandler) UpdatePriceReference(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	ref, err := h.priceRefRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Referencia no encontrada")
		return
	}

	var req struct {
		ServiceType string  `json:"service_type"`
		MinUSD      float64 `json:"min_usd"`
		MaxUSD      float64 `json:"max_usd"`
		TypicalTime string  `json:"typical_time"`
		Description string  `json:"description"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.ServiceType != "" {
		ref.ServiceType = req.ServiceType
	}
	if req.MinUSD > 0 {
		ref.MinUSD = req.MinUSD
	}
	if req.MaxUSD > 0 {
		ref.MaxUSD = req.MaxUSD
	}
	if req.TypicalTime != "" {
		ref.TypicalTime = req.TypicalTime
	}
	if req.Description != "" {
		ref.Description = &req.Description
	}
	if req.IsActive != nil {
		ref.IsActive = *req.IsActive
	}

	if err := h.priceRefRepo.Update(ref); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar referencia")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    ref,
	})
}

func (h *AdminHandler) DeletePriceReference(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	if err := h.priceRefRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar referencia")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Referencia eliminada",
	})
}

// ==================== USERS (Admin) ====================

func (h *AdminHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User management coming in Phase 2",
	})
}

func (h *AdminHandler) UpdateUserQuota(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	user, err := h.userRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Usuario no encontrado")
		return
	}

	var req struct {
		QuoteQuota int `json:"quote_quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	user.QuoteQuota = req.QuoteQuota
	if err := h.userRepo.Update(user); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar cuota")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":          user.ID,
			"cedula":      user.Cedula,
			"nombre":      user.Nombre,
			"quote_quota": user.QuoteQuota,
			"quotes_used": user.QuotesUsed,
		},
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]interface{}{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
