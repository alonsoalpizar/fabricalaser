package config

import (
	"encoding/json"
	"net/http"

	"github.com/alonsoalpizar/fabricalaser/internal/repository"
)

type ConfigHandler struct {
	techRepo      *repository.TechnologyRepository
	materialRepo  *repository.MaterialRepository
	engraveRepo   *repository.EngraveTypeRepository
	rateRepo      *repository.TechRateRepository
	discountRepo  *repository.VolumeDiscountRepository
	priceRefRepo  *repository.PriceReferenceRepository
}

func NewConfigHandler() *ConfigHandler {
	return &ConfigHandler{
		techRepo:      repository.NewTechnologyRepository(),
		materialRepo:  repository.NewMaterialRepository(),
		engraveRepo:   repository.NewEngraveTypeRepository(),
		rateRepo:      repository.NewTechRateRepository(),
		discountRepo:  repository.NewVolumeDiscountRepository(),
		priceRefRepo:  repository.NewPriceReferenceRepository(),
	}
}

// GetTechnologies returns all active technologies
func (h *ConfigHandler) GetTechnologies(w http.ResponseWriter, r *http.Request) {
	technologies, err := h.techRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener tecnologías")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    technologies,
	})
}

// GetMaterials returns all active materials
func (h *ConfigHandler) GetMaterials(w http.ResponseWriter, r *http.Request) {
	materials, err := h.materialRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener materiales")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    materials,
	})
}

// GetEngraveTypes returns all active engrave types
func (h *ConfigHandler) GetEngraveTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.engraveRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener tipos de grabado")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    types,
	})
}

// GetTechRates returns all active tech rates with technology info
func (h *ConfigHandler) GetTechRates(w http.ResponseWriter, r *http.Request) {
	rates, err := h.rateRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener tarifas")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    rates,
	})
}

// GetVolumeDiscounts returns all active volume discounts
func (h *ConfigHandler) GetVolumeDiscounts(w http.ResponseWriter, r *http.Request) {
	discounts, err := h.discountRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener descuentos")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    discounts,
	})
}

// GetPriceReferences returns all active price references
func (h *ConfigHandler) GetPriceReferences(w http.ResponseWriter, r *http.Request) {
	refs, err := h.priceRefRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener referencias de precio")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    refs,
	})
}

// GetAll returns all configuration data in a single response (for initial load)
func (h *ConfigHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	technologies, err := h.techRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener configuración")
		return
	}

	materials, err := h.materialRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener configuración")
		return
	}

	engraveTypes, err := h.engraveRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener configuración")
		return
	}

	rates, err := h.rateRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener configuración")
		return
	}

	discounts, err := h.discountRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener configuración")
		return
	}

	priceRefs, err := h.priceRefRepo.FindAll()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "FETCH_ERROR", "Error al obtener configuración")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"technologies":     technologies,
			"materials":        materials,
			"engrave_types":    engraveTypes,
			"tech_rates":       rates,
			"volume_discounts": discounts,
			"price_references": priceRefs,
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
