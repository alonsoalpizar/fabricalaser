package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/go-chi/chi/v5"
)

type TechMaterialSpeedHandler struct {
	speedRepo *repository.TechMaterialSpeedRepository
}

func NewTechMaterialSpeedHandler() *TechMaterialSpeedHandler {
	return &TechMaterialSpeedHandler{
		speedRepo: repository.NewTechMaterialSpeedRepository(),
	}
}

// GetTechMaterialSpeeds returns all tech material speeds with optional filters
func (h *TechMaterialSpeedHandler) GetTechMaterialSpeeds(w http.ResponseWriter, r *http.Request) {
	// Parse query params for filtering
	var techID, materialID uint
	if tid := r.URL.Query().Get("technology_id"); tid != "" {
		if id, err := strconv.ParseUint(tid, 10, 32); err == nil {
			techID = uint(id)
		}
	}
	if mid := r.URL.Query().Get("material_id"); mid != "" {
		if id, err := strconv.ParseUint(mid, 10, 32); err == nil {
			materialID = uint(id)
		}
	}

	var speeds []models.TechMaterialSpeed
	var err error

	if techID > 0 || materialID > 0 {
		speeds, err = h.speedRepo.FindByTechAndMaterial(techID, materialID)
	} else {
		speeds, err = h.speedRepo.FindAll()
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "LIST_ERROR", "Error al listar velocidades")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    speeds,
	})
}

// GetTechMaterialSpeed returns a single tech material speed by ID
func (h *TechMaterialSpeedHandler) GetTechMaterialSpeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	speed, err := h.speedRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Configuracion de velocidad no encontrada")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    speed,
	})
}

// CreateTechMaterialSpeed creates a new tech material speed
func (h *TechMaterialSpeedHandler) CreateTechMaterialSpeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TechnologyID      uint     `json:"technology_id"`
		MaterialID        uint     `json:"material_id"`
		Thickness         float64  `json:"thickness"`
		CutSpeedMmMin     *float64 `json:"cut_speed_mm_min"`
		EngraveSpeedMmMin *float64 `json:"engrave_speed_mm_min"`
		IsCompatible      *bool    `json:"is_compatible"`
		Notes             string   `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON invalido")
		return
	}

	if req.TechnologyID == 0 || req.MaterialID == 0 {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "technology_id y material_id son requeridos")
		return
	}

	speed := &models.TechMaterialSpeed{
		TechnologyID:      req.TechnologyID,
		MaterialID:        req.MaterialID,
		Thickness:         req.Thickness,
		CutSpeedMmMin:     req.CutSpeedMmMin,
		EngraveSpeedMmMin: req.EngraveSpeedMmMin,
		IsCompatible:      true, // default
		IsActive:          true,
	}
	if req.IsCompatible != nil {
		speed.IsCompatible = *req.IsCompatible
	}
	if req.Notes != "" {
		speed.Notes = &req.Notes
	}

	if err := h.speedRepo.Create(speed); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear configuracion de velocidad")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id": speed.ID,
		},
	})
}

// UpdateTechMaterialSpeed updates an existing tech material speed
func (h *TechMaterialSpeedHandler) UpdateTechMaterialSpeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	speed, err := h.speedRepo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Configuracion de velocidad no encontrada")
		return
	}

	var req struct {
		CutSpeedMmMin     *float64 `json:"cut_speed_mm_min"`
		EngraveSpeedMmMin *float64 `json:"engrave_speed_mm_min"`
		IsCompatible      *bool    `json:"is_compatible"`
		Notes             *string  `json:"notes"`
		IsActive          *bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON invalido")
		return
	}

	// Update fields if provided
	if req.CutSpeedMmMin != nil {
		speed.CutSpeedMmMin = req.CutSpeedMmMin
	}
	if req.EngraveSpeedMmMin != nil {
		speed.EngraveSpeedMmMin = req.EngraveSpeedMmMin
	}
	if req.IsCompatible != nil {
		speed.IsCompatible = *req.IsCompatible
	}
	if req.Notes != nil {
		speed.Notes = req.Notes
	}
	if req.IsActive != nil {
		speed.IsActive = *req.IsActive
	}

	if err := h.speedRepo.Update(speed); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar configuracion de velocidad")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    speed,
	})
}

// DeleteTechMaterialSpeed soft-deletes a tech material speed
func (h *TechMaterialSpeedHandler) DeleteTechMaterialSpeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	if err := h.speedRepo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar configuracion de velocidad")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Configuracion de velocidad eliminada",
	})
}

// BulkCreateTechMaterialSpeeds creates multiple tech material speeds
func (h *TechMaterialSpeedHandler) BulkCreateTechMaterialSpeeds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Speeds []struct {
			TechnologyID      uint     `json:"technology_id"`
			MaterialID        uint     `json:"material_id"`
			Thickness         float64  `json:"thickness"`
			CutSpeedMmMin     *float64 `json:"cut_speed_mm_min"`
			EngraveSpeedMmMin *float64 `json:"engrave_speed_mm_min"`
			IsCompatible      *bool    `json:"is_compatible"`
			Notes             string   `json:"notes"`
		} `json:"speeds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON invalido")
		return
	}

	if len(req.Speeds) == 0 {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "Se requiere al menos una configuracion de velocidad")
		return
	}

	speeds := make([]models.TechMaterialSpeed, len(req.Speeds))
	for i, s := range req.Speeds {
		if s.TechnologyID == 0 || s.MaterialID == 0 {
			respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "technology_id y material_id son requeridos en cada registro")
			return
		}
		speeds[i] = models.TechMaterialSpeed{
			TechnologyID:      s.TechnologyID,
			MaterialID:        s.MaterialID,
			Thickness:         s.Thickness,
			CutSpeedMmMin:     s.CutSpeedMmMin,
			EngraveSpeedMmMin: s.EngraveSpeedMmMin,
			IsCompatible:      true,
			IsActive:          true,
		}
		if s.IsCompatible != nil {
			speeds[i].IsCompatible = *s.IsCompatible
		}
		if s.Notes != "" {
			speeds[i].Notes = &s.Notes
		}
	}

	if err := h.speedRepo.BulkCreate(speeds); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear configuraciones de velocidad")
		return
	}

	// Collect IDs of created records
	ids := make([]uint, len(speeds))
	for i, s := range speeds {
		ids[i] = s.ID
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"created": len(speeds),
			"ids":     ids,
		},
	})
}
