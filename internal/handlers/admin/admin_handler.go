package admin

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/alonsoalpizar/fabricalaser/internal/utils"
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
	quoteRepo     *repository.QuoteRepository
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
		quoteRepo:     repository.NewQuoteRepository(),
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
		EngraveRateHour   float64  `json:"engrave_rate_hour"`
		CutRateHour       float64  `json:"cut_rate_hour"`
		DesignRateHour    float64  `json:"design_rate_hour"`
		OverheadRateHour  float64  `json:"overhead_rate_hour"`
		SetupFee          float64  `json:"setup_fee"`
		CostPerMinEngrave float64  `json:"cost_per_min_engrave"`
		CostPerMinCut     float64  `json:"cost_per_min_cut"`
		MarginPercent     float64  `json:"margin_percent"`
		ElectricidadMes   *float64 `json:"electricidad_mes"`
		MantenimientoMes  *float64 `json:"mantenimiento_mes"`
		DepreciacionMes   *float64 `json:"depreciacion_mes"`
		SeguroMes         *float64 `json:"seguro_mes"`
		ConsumiblesMes    *float64 `json:"consumibles_mes"`
		IsActive          *bool    `json:"is_active"`
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
	// Costos fijos por máquina (₡/mes)
	if req.ElectricidadMes != nil {
		rate.ElectricidadMes = *req.ElectricidadMes
	}
	if req.MantenimientoMes != nil {
		rate.MantenimientoMes = *req.MantenimientoMes
	}
	if req.DepreciacionMes != nil {
		rate.DepreciacionMes = *req.DepreciacionMes
	}
	if req.SeguroMes != nil {
		rate.SeguroMes = *req.SeguroMes
	}
	if req.ConsumiblesMes != nil {
		rate.ConsumiblesMes = *req.ConsumiblesMes
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
	// Parse query params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 15
	}
	offset := (page - 1) * limit

	search := r.URL.Query().Get("search")
	role := r.URL.Query().Get("role")
	var isActive *bool
	if activeStr := r.URL.Query().Get("is_active"); activeStr != "" {
		val := activeStr == "true"
		isActive = &val
	}

	users, total, err := h.userRepo.ListAll(limit, offset, search, role, isActive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "LIST_ERROR", "Error al listar usuarios")
		return
	}

	// Map to response format
	usersResp := make([]map[string]interface{}, len(users))
	for i, u := range users {
		usersResp[i] = map[string]interface{}{
			"id":          u.ID,
			"cedula":      u.Cedula,
			"nombre":      u.Nombre,
			"email":       u.Email,
			"telefono":    u.Telefono,
			"role":        u.Role,
			"is_active":   u.Activo,
			"quote_limit": u.QuoteQuota,
			"quotes_used": u.QuotesUsed,
			"created_at":  u.CreatedAt,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"users": usersResp,
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cedula     string `json:"cedula"`
		Nombre     string `json:"nombre"`
		Email      string `json:"email"`
		Telefono   string `json:"telefono"`
		Password   string `json:"password"`
		Role       string `json:"role"`
		IsActive   bool   `json:"is_active"`
		QuoteLimit int    `json:"quote_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Cedula == "" || req.Nombre == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Cédula y nombre son requeridos")
		return
	}

	// Check if user exists
	existing, _ := h.userRepo.FindByCedula(req.Cedula)
	if existing != nil {
		respondError(w, http.StatusConflict, "USER_EXISTS", "Ya existe un usuario con esta cédula")
		return
	}

	user := &models.User{
		Cedula:     req.Cedula,
		Nombre:     req.Nombre,
		Role:       req.Role,
		Activo:     req.IsActive,
		QuoteQuota: req.QuoteLimit,
	}
	if user.Role == "" {
		user.Role = "user"
	}
	if user.QuoteQuota == 0 {
		user.QuoteQuota = 3
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Telefono != "" {
		user.Telefono = &req.Telefono
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "HASH_ERROR", "Error al procesar contraseña")
			return
		}
		user.PasswordHash = &hash
	}

	if err := h.userRepo.Create(user); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear usuario")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":     user.ID,
			"cedula": user.Cedula,
			"nombre": user.Nombre,
		},
	})
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
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
		Nombre     string `json:"nombre"`
		Email      string `json:"email"`
		Telefono   string `json:"telefono"`
		Password   string `json:"password"`
		Role       string `json:"role"`
		IsActive   *bool  `json:"is_active"`
		QuoteLimit *int   `json:"quote_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Nombre != "" {
		user.Nombre = req.Nombre
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Telefono != "" {
		user.Telefono = &req.Telefono
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.IsActive != nil {
		user.Activo = *req.IsActive
	}
	if req.QuoteLimit != nil {
		user.QuoteQuota = *req.QuoteLimit
	}
	if req.Password != "" {
		hash, err := utils.HashPassword(req.Password)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "HASH_ERROR", "Error al procesar contraseña")
			return
		}
		user.PasswordHash = &hash
	}

	if err := h.userRepo.Update(user); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar usuario")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":     user.ID,
			"cedula": user.Cedula,
			"nombre": user.Nombre,
		},
	})
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	if err := h.userRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar usuario")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Usuario eliminado",
	})
}

// ==================== QUOTES (Admin) ====================

func (h *AdminHandler) GetQuotes(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 15
	}
	offset := (page - 1) * limit

	status := r.URL.Query().Get("status")
	sortOrder := r.URL.Query().Get("sort")
	if sortOrder == "" {
		sortOrder = "desc"
	}

	quotes, total, err := h.quoteRepo.ListAllAdmin(limit, offset, status, sortOrder)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "LIST_ERROR", "Error al listar cotizaciones")
		return
	}

	// Map to response format
	quotesResp := make([]map[string]interface{}, len(quotes))
	for i, q := range quotes {
		resp := map[string]interface{}{
			"id":          q.ID,
			"status":      q.Status,
			"total_price": q.PriceFinal,
			"quantity":    q.Quantity,
			"created_at":  q.CreatedAt,
		}
		if q.User != nil && q.User.ID > 0 {
			resp["user_name"] = q.User.Nombre
			resp["cedula"] = q.User.Cedula
		}
		if q.Technology != nil && q.Technology.ID > 0 {
			resp["technology_name"] = q.Technology.Name
		}
		if q.Material != nil && q.Material.ID > 0 {
			resp["material_name"] = q.Material.Name
		}
		if q.SVGAnalysis != nil {
			resp["filename"] = q.SVGAnalysis.Filename
		}
		quotesResp[i] = resp
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"quotes": quotesResp,
			"total":  total,
			"page":   page,
			"limit":  limit,
		},
	})
}

func (h *AdminHandler) GetQuote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	quote, err := h.quoteRepo.FindByIDWithRelations(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Cotización no encontrada")
		return
	}

	resp := map[string]interface{}{
		"id":               quote.ID,
		"status":           quote.Status,
		"quantity":         quote.Quantity,
		"cut_price":        quote.CostCut,
		"engrave_price":    quote.CostEngrave,
		"subtotal":         quote.CostBase,
		"discount_percent": quote.DiscountVolumePct,
		"discount_amount":  0, // Calculate if needed
		"total_price":      quote.PriceFinal,
		"admin_notes":      quote.ReviewNotes,
		"created_at":       quote.CreatedAt,
		"updated_at":       quote.UpdatedAt,
	}

	if quote.User != nil && quote.User.ID > 0 {
		resp["user_name"] = quote.User.Nombre
		resp["cedula"] = quote.User.Cedula
		resp["user_email"] = quote.User.Email
		resp["user_phone"] = quote.User.Telefono
	}
	if quote.Technology != nil && quote.Technology.ID > 0 {
		resp["technology_name"] = quote.Technology.Name
	}
	if quote.Material != nil && quote.Material.ID > 0 {
		resp["material_name"] = quote.Material.Name
	}
	if quote.EngraveType != nil && quote.EngraveType.ID > 0 {
		resp["engrave_type_name"] = quote.EngraveType.Name
	}
	if quote.SVGAnalysis != nil {
		resp["filename"] = quote.SVGAnalysis.Filename
		resp["svg_analysis"] = map[string]interface{}{
			"width_mm":         quote.SVGAnalysis.Width,
			"height_mm":        quote.SVGAnalysis.Height,
			"cut_length_mm":    quote.SVGAnalysis.CutLengthMM,
			"vector_length_mm": quote.SVGAnalysis.VectorLengthMM,
			"raster_area_mm2":  quote.SVGAnalysis.RasterAreaMM2,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    resp,
	})
}

func (h *AdminHandler) UpdateQuote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	quote, err := h.quoteRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Cotización no encontrada")
		return
	}

	var req struct {
		Status     string `json:"status"`
		AdminNotes string `json:"admin_notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Status != "" {
		quote.Status = models.QuoteStatus(req.Status)
	}
	if req.AdminNotes != "" {
		quote.ReviewNotes = &req.AdminNotes
	}

	if err := h.quoteRepo.Update(quote); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar cotización")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":     quote.ID,
			"status": quote.Status,
		},
	})
}

// ==================== TECH RATES (Admin) ====================

func (h *AdminHandler) GetTechRates(w http.ResponseWriter, r *http.Request) {
	rates, err := h.rateRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "LIST_ERROR", "Error al listar tarifas")
		return
	}

	ratesResp := make([]map[string]interface{}, len(rates))
	for i, rate := range rates {
		ratesResp[i] = map[string]interface{}{
			"id":                   rate.ID,
			"technology_id":        rate.TechnologyID,
			"technology_name":      rate.Technology.Name,
			"engrave_rate_hour":    rate.EngraveRateHour,
			"cut_rate_hour":        rate.CutRateHour,
			"design_rate_hour":     rate.DesignRateHour,
			"overhead_rate_hour":   rate.OverheadRateHour,
			"setup_fee":            rate.SetupFee,
			"cost_per_min_engrave": rate.CostPerMinEngrave,
			"cost_per_min_cut":     rate.CostPerMinCut,
			"margin_percent":       rate.MarginPercent,
			"is_active":            rate.IsActive,
			// Costos fijos por máquina
			"electricidad_mes":   rate.ElectricidadMes,
			"mantenimiento_mes":  rate.MantenimientoMes,
			"depreciacion_mes":   rate.DepreciacionMes,
			"seguro_mes":         rate.SeguroMes,
			"consumibles_mes":    rate.ConsumiblesMes,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    ratesResp,
	})
}

func (h *AdminHandler) CreateTechRate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TechnologyID     uint    `json:"technology_id"`
		EngraveRateHour  float64 `json:"engrave_rate_hour"`
		CutRateHour      float64 `json:"cut_rate_hour"`
		DesignRateHour   float64 `json:"design_rate_hour"`
		OverheadRateHour float64 `json:"overhead_rate_hour"`
		SetupFee         float64 `json:"setup_fee"`
		MarginPercent    float64 `json:"margin_percent"`
		ElectricidadMes  float64 `json:"electricidad_mes"`
		MantenimientoMes float64 `json:"mantenimiento_mes"`
		DepreciacionMes  float64 `json:"depreciacion_mes"`
		SeguroMes        float64 `json:"seguro_mes"`
		ConsumiblesMes   float64 `json:"consumibles_mes"`
		IsActive         bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.TechnologyID == 0 {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Technology es requerido")
		return
	}

	rate := &models.TechRate{
		TechnologyID:      req.TechnologyID,
		EngraveRateHour:   req.EngraveRateHour,
		CutRateHour:       req.CutRateHour,
		DesignRateHour:    req.DesignRateHour,
		OverheadRateHour:  req.OverheadRateHour,
		SetupFee:          req.SetupFee,
		MarginPercent:     req.MarginPercent,
		CostPerMinEngrave: (req.EngraveRateHour + req.OverheadRateHour) / 60,
		CostPerMinCut:     (req.CutRateHour + req.OverheadRateHour) / 60,
		ElectricidadMes:   req.ElectricidadMes,
		MantenimientoMes:  req.MantenimientoMes,
		DepreciacionMes:   req.DepreciacionMes,
		SeguroMes:         req.SeguroMes,
		ConsumiblesMes:    req.ConsumiblesMes,
		IsActive:          req.IsActive,
	}

	if err := h.rateRepo.Create(rate); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear tarifa")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id": rate.ID,
		},
	})
}

func (h *AdminHandler) DeleteTechRate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}

	if err := h.rateRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar tarifa")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Tarifa eliminada",
	})
}

// GetDashboardStats returns aggregated stats for the admin dashboard
func (h *AdminHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	db := database.Get()

	// Get total quoted this month (sum of price_final for approved quotes)
	var totalQuotedMonth float64
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	err := db.Model(&models.Quote{}).
		Where("created_at >= ?", startOfMonth).
		Where("status IN ?", []string{"auto_approved", "approved", "converted"}).
		Select("COALESCE(SUM(price_final), 0)").
		Scan(&totalQuotedMonth).Error

	if err != nil {
		log.Printf("Error getting dashboard stats: %v", err)
		totalQuotedMonth = 0
	}

	// Get total quotes this month
	var totalQuotesMonth int64
	db.Model(&models.Quote{}).
		Where("created_at >= ?", startOfMonth).
		Count(&totalQuotesMonth)

	// Get new users this month
	var newUsersMonth int64
	db.Model(&models.User{}).
		Where("created_at >= ?", startOfMonth).
		Count(&newUsersMonth)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"total_quoted_month": totalQuotedMonth,
			"total_quotes_month": totalQuotesMonth,
			"new_users_month":    newUsersMonth,
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
