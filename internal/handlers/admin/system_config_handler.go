package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/go-chi/chi/v5"
)

type SystemConfigHandler struct {
	repo *repository.SystemConfigRepository
}

func NewSystemConfigHandler() *SystemConfigHandler {
	return &SystemConfigHandler{
		repo: repository.NewSystemConfigRepository(),
	}
}

// GetSystemConfigs returns all system configurations
func (h *SystemConfigHandler) GetSystemConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.repo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "LIST_ERROR", "Error al listar configuraciones")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    configs,
	})
}

// GetSystemConfig returns a single system configuration by ID
func (h *SystemConfigHandler) GetSystemConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	config, err := h.repo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Configuracion no encontrada")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    config,
	})
}

// CreateSystemConfig creates a new system configuration
func (h *SystemConfigHandler) CreateSystemConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigKey   string `json:"config_key"`
		ConfigValue string `json:"config_value"`
		ValueType   string `json:"value_type"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON invalido")
		return
	}

	if req.ConfigKey == "" || req.ConfigValue == "" || req.Category == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "config_key, config_value y category son requeridos")
		return
	}

	// Check if key already exists
	existing, _ := h.repo.FindByKey(req.ConfigKey)
	if existing != nil {
		respondError(w, http.StatusConflict, "KEY_EXISTS", "Ya existe una configuracion con esta clave")
		return
	}

	config := &models.SystemConfig{
		ConfigKey:   req.ConfigKey,
		ConfigValue: req.ConfigValue,
		ValueType:   req.ValueType,
		Category:    req.Category,
		IsActive:    true,
	}

	if config.ValueType == "" {
		config.ValueType = "string"
	}

	if req.Description != "" {
		config.Description = &req.Description
	}

	if err := h.repo.Create(config); err != nil {
		respondError(w, http.StatusInternalServerError, "CREATE_ERROR", "Error al crear configuracion")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    config,
	})
}

// UpdateSystemConfig updates an existing system configuration
func (h *SystemConfigHandler) UpdateSystemConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	config, err := h.repo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Configuracion no encontrada")
		return
	}

	var req struct {
		ConfigValue string `json:"config_value"`
		Description string `json:"description"`
		IsActive    *bool  `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON invalido")
		return
	}

	if req.ConfigValue != "" {
		config.ConfigValue = req.ConfigValue
	}
	if req.Description != "" {
		config.Description = &req.Description
	}
	if req.IsActive != nil {
		config.IsActive = *req.IsActive
	}

	if err := h.repo.Update(config); err != nil {
		respondError(w, http.StatusInternalServerError, "UPDATE_ERROR", "Error al actualizar configuracion")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    config,
	})
}

// DeleteSystemConfig soft-deletes a system configuration
func (h *SystemConfigHandler) DeleteSystemConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID invalido")
		return
	}

	if err := h.repo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DELETE_ERROR", "Error al eliminar configuracion")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Configuracion eliminada",
	})
}
