package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/go-chi/chi/v5"
)

type MaterialCostHandler struct {
	repo *repository.MaterialCostRepository
}

func NewMaterialCostHandler() *MaterialCostHandler {
	return &MaterialCostHandler{
		repo: repository.NewMaterialCostRepository(),
	}
}

// GetMaterialCosts returns all material costs
func (h *MaterialCostHandler) GetMaterialCosts(w http.ResponseWriter, r *http.Request) {
	// Optional filter by material_id
	materialIDStr := r.URL.Query().Get("material_id")

	var costs []models.MaterialCost
	var err error

	if materialIDStr != "" {
		materialID, parseErr := strconv.ParseUint(materialIDStr, 10, 32)
		if parseErr != nil {
			respondError(w, http.StatusBadRequest, "INVALID_PARAM", "material_id invalido")
			return
		}
		costs, err = h.repo.FindByMaterial(uint(materialID))
	} else {
		costs, err = h.repo.FindAll()
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "LIST_ERROR", "Error al listar costos de material")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    costs,
	})
}

// GetMaterialCost returns a single material cost by ID
func (h *MaterialCostHandler) GetMaterialCost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	cost, err := h.repo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Costo de material no encontrado")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    cost,
	})
}

// CreateMaterialCost creates a new material cost
func (h *MaterialCostHandler) CreateMaterialCost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaterialID    uint     `json:"material_id"`
		Thickness     float64  `json:"thickness"`
		CostPerMm2    *float64 `json:"cost_per_mm2"`
		WastePct      *float64 `json:"waste_pct"`
		SheetCost     *float64 `json:"sheet_cost"`
		SheetWidthMm  *float64 `json:"sheet_width_mm"`
		SheetHeightMm *float64 `json:"sheet_height_mm"`
		Notes         string   `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON invalido")
		return
	}

	if req.MaterialID == 0 {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "material_id es requerido")
		return
	}

	cost := &models.MaterialCost{
		MaterialID:    req.MaterialID,
		Thickness:     req.Thickness,
		WastePct:      0.15, // Default
		SheetCost:     req.SheetCost,
		SheetWidthMm:  req.SheetWidthMm,
		SheetHeightMm: req.SheetHeightMm,
		IsActive:      true,
	}

	if req.WastePct != nil {
		cost.WastePct = *req.WastePct
	}

	if req.Notes != "" {
		cost.Notes = &req.Notes
	}

	// Calculate cost_per_mm2 if sheet dimensions provided
	if req.SheetCost != nil && req.SheetWidthMm != nil && req.SheetHeightMm != nil {
		if *req.SheetWidthMm > 0 && *req.SheetHeightMm > 0 {
			area := *req.SheetWidthMm * *req.SheetHeightMm
			cost.CostPerMm2 = *req.SheetCost / area
		}
	} else if req.CostPerMm2 != nil {
		cost.CostPerMm2 = *req.CostPerMm2
	} else {
		respondError(w, http.StatusBadRequest, "MISSING_COST", "Debe proveer cost_per_mm2 o (sheet_cost + sheet_width_mm + sheet_height_mm)")
		return
	}

	if err := h.repo.Create(cost); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear costo de material")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":           cost.ID,
			"cost_per_mm2": cost.CostPerMm2,
		},
	})
}

// UpdateMaterialCost updates an existing material cost
func (h *MaterialCostHandler) UpdateMaterialCost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	cost, err := h.repo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Costo de material no encontrado")
		return
	}

	var req struct {
		CostPerMm2    *float64 `json:"cost_per_mm2"`
		WastePct      *float64 `json:"waste_pct"`
		SheetCost     *float64 `json:"sheet_cost"`
		SheetWidthMm  *float64 `json:"sheet_width_mm"`
		SheetHeightMm *float64 `json:"sheet_height_mm"`
		Notes         *string  `json:"notes"`
		IsActive      *bool    `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON invalido")
		return
	}

	if req.CostPerMm2 != nil {
		cost.CostPerMm2 = *req.CostPerMm2
	}
	if req.WastePct != nil {
		cost.WastePct = *req.WastePct
	}
	if req.SheetCost != nil {
		cost.SheetCost = req.SheetCost
	}
	if req.SheetWidthMm != nil {
		cost.SheetWidthMm = req.SheetWidthMm
	}
	if req.SheetHeightMm != nil {
		cost.SheetHeightMm = req.SheetHeightMm
	}
	if req.Notes != nil {
		cost.Notes = req.Notes
	}
	if req.IsActive != nil {
		cost.IsActive = *req.IsActive
	}

	if err := h.repo.Update(cost); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar costo de material")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    cost,
	})
}

// DeleteMaterialCost soft-deletes a material cost
func (h *MaterialCostHandler) DeleteMaterialCost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	if err := h.repo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar costo de material")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Costo de material eliminado",
	})
}

// RecalculateMaterialCost recalculates cost_per_mm2 from sheet dimensions
func (h *MaterialCostHandler) RecalculateMaterialCost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	cost, err := h.repo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Costo de material no encontrado")
		return
	}

	if cost.SheetCost == nil || cost.SheetWidthMm == nil || cost.SheetHeightMm == nil {
		respondError(w, http.StatusBadRequest, "MISSING_SHEET_DATA", "Faltan datos de lamina (sheet_cost, sheet_width_mm, sheet_height_mm)")
		return
	}

	if *cost.SheetWidthMm <= 0 || *cost.SheetHeightMm <= 0 {
		respondError(w, http.StatusBadRequest, "INVALID_DIMENSIONS", "Dimensiones de lamina invalidas")
		return
	}

	// Calculate new cost_per_mm2
	area := *cost.SheetWidthMm * *cost.SheetHeightMm
	newCostPerMm2 := *cost.SheetCost / area
	cost.CostPerMm2 = newCostPerMm2

	if err := h.repo.Update(cost); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar costo")
		return
	}

	message := fmt.Sprintf("Costo recalculado: ₡%.2f / (%.0f × %.0f) = ₡%.8f/mm²",
		*cost.SheetCost, *cost.SheetWidthMm, *cost.SheetHeightMm, newCostPerMm2)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":           cost.ID,
			"cost_per_mm2": newCostPerMm2,
			"message":      message,
		},
	})
}
