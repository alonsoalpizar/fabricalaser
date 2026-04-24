package admin

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/go-chi/chi/v5"
)

// BlankHandler gestiona el CRUD admin de blanks y el endpoint público
// de consulta usado por el agente de WhatsApp.
type BlankHandler struct {
	repo *repository.BlankRepository
}

func NewBlankHandler() *BlankHandler {
	return &BlankHandler{repo: repository.NewBlankRepository()}
}

// GetAll retorna todos los blanks (activos e inactivos) para el panel admin.
func (h *BlankHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	blanks, err := h.repo.FindAllAdmin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"blanks": blanks, "total": len(blanks)})
}

// Create crea un nuevo blank (admin).
func (h *BlankHandler) Create(w http.ResponseWriter, r *http.Request) {
	var blank models.Blank
	if err := json.NewDecoder(r.Body).Decode(&blank); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if blank.Name == "" || blank.Category == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "name y category son obligatorios")
		return
	}
	if blank.PriceBreaks == nil {
		blank.PriceBreaks = []byte("[]")
	}
	if blank.Accessories == nil {
		blank.Accessories = []byte("[]")
	}
	if blank.Aliases == nil {
		blank.Aliases = []byte("[]")
	}
	blank.IsActive = true
	if err := h.repo.Create(&blank); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"blank": blank})
}

// Update actualiza un blank existente (admin).
func (h *BlankHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}
	existing, err := h.repo.FindByID(uint(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Blank no encontrado")
		return
	}

	var updates models.Blank
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	existing.Name = updates.Name
	existing.Category = updates.Category
	existing.Description = updates.Description
	existing.Dimensions = updates.Dimensions
	existing.CostPrice = updates.CostPrice
	existing.BasePrice = updates.BasePrice
	existing.MinQty = updates.MinQty
	existing.StockQty = updates.StockQty
	existing.StockAlert = updates.StockAlert
	existing.IsActive = updates.IsActive
	if updates.PriceBreaks != nil {
		existing.PriceBreaks = updates.PriceBreaks
	}
	if updates.Accessories != nil {
		existing.Accessories = updates.Accessories
	}
	if updates.Aliases != nil {
		existing.Aliases = updates.Aliases
	}

	if err := h.repo.Update(existing); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"blank": existing})
}

// Delete realiza soft-delete de un blank (admin).
func (h *BlankHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}
	if err := h.repo.Delete(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// UpdateStock ajusta el stock de un blank (admin).
// Body: {"qty": 50, "operation": "add"} — "add" suma, "set" reemplaza.
func (h *BlankHandler) UpdateStock(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}
	var body struct {
		Qty       int    `json:"qty"`
		Operation string `json:"operation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if body.Operation != "add" && body.Operation != "set" {
		respondError(w, http.StatusBadRequest, "INVALID_OPERATION", "operation debe ser 'add' o 'set'")
		return
	}
	if err := h.repo.UpdateStock(uint(id), body.Qty, body.Operation); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	blank, _ := h.repo.FindByID(uint(id))
	respondJSON(w, http.StatusOK, map[string]any{"stock_qty": blank.StockQty})
}

// ToggleFeatured invierte is_featured de un blank (admin).
func (h *BlankHandler) ToggleFeatured(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_ID", "ID inválido")
		return
	}
	if err := h.repo.ToggleFeatured(uint(id)); err != nil {
		respondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	blank, _ := h.repo.FindByID(uint(id))
	respondJSON(w, http.StatusOK, map[string]any{"is_featured": blank.IsFeatured})
}

// ─── Endpoint público para el agente de WhatsApp ─────────────────────────────

// ConsultarBlank es llamado por el tool consultar_blank del agente Gemini.
// No requiere auth (es un endpoint interno, protegido por INTERNAL_API_TOKEN en el caller).
func (h *BlankHandler) ConsultarBlank(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Categoria string `json:"categoria"`
		Cantidad  int    `json:"cantidad"`
		BlankID   int    `json:"blank_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.Categoria == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "categoria es obligatorio")
		return
	}

	// Búsqueda por ID específico
	if req.BlankID > 0 {
		blank, err := h.repo.FindByID(uint(req.BlankID))
		if err != nil || !blank.IsActive {
			respondJSON(w, http.StatusOK, map[string]any{
				"encontrado": false,
				"mensaje":    "Blank no encontrado",
			})
			return
		}
		respondJSON(w, http.StatusOK, h.buildBlankResult(blank, req.Cantidad))
		go h.repo.IncrementQuoteCount(blank.ID)
		return
	}

	// Búsqueda por categoría
	blanks, err := h.repo.FindByCategory(req.Categoria)
	if err != nil || len(blanks) == 0 {
		respondJSON(w, http.StatusOK, map[string]any{
			"encontrado": false,
			"mensaje":    "No hay blanks disponibles en esa categoría",
		})
		return
	}

	// Múltiples opciones → devolver lista para que el agente pregunte
	if req.BlankID == 0 && len(blanks) > 1 {
		type opcion struct {
			ID          uint   `json:"id"`
			Nombre      string `json:"nombre"`
			Dimensiones string `json:"dimensiones"`
			PrecioBase  int    `json:"precio_base"`
			MinQty      int    `json:"min_qty"`
		}
		opciones := make([]opcion, 0, len(blanks))
		for _, b := range blanks {
			dim := ""
			if b.Dimensions != nil {
				dim = *b.Dimensions
			}
			opciones = append(opciones, opcion{
				ID:          b.ID,
				Nombre:      b.Name,
				Dimensiones: dim,
				PrecioBase:  b.BasePrice,
				MinQty:      b.MinQty,
			})
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"encontrado":         true,
			"multiples_opciones": true,
			"mensaje":            "Hay varias opciones disponibles en esa categoría",
			"opciones":           opciones,
		})
		return
	}

	// Un solo resultado
	blank := blanks[0]
	respondJSON(w, http.StatusOK, h.buildBlankResult(&blank, req.Cantidad))
	go h.repo.IncrementQuoteCount(blank.ID)
}

// buildBlankResult construye la respuesta de consulta para un blank específico,
// calculando el precio correcto según la cantidad solicitada.
func (h *BlankHandler) buildBlankResult(b *models.Blank, qty int) map[string]any {
	unitPrice := computePriceForQty(b, qty)
	totalPrice := unitPrice * qty

	dim := ""
	if b.Dimensions != nil {
		dim = *b.Dimensions
	}

	var accessories []map[string]any
	_ = json.Unmarshal(b.Accessories, &accessories)

	result := map[string]any{
		"encontrado":            true,
		"id":                    b.ID,
		"nombre":                b.Name,
		"categoria":             b.Category,
		"descripcion":           b.Description,
		"dimensiones":           dim,
		"min_qty":               b.MinQty,
		"precio_unitario":       unitPrice,
		"precio_total":          totalPrice,
		"cantidad_solicitada":   qty,
		"accesorios_opcionales": accessories,
	}

	// Avisos de stock
	if b.StockQty == 0 {
		result["sin_stock"] = true
		result["mensaje_stock"] = "Producto sin stock disponible — consultar disponibilidad con el asesor"
	} else if b.StockQty <= b.StockAlert {
		result["stock_bajo"] = true
		result["mensaje_stock"] = "Stock limitado — confirmar disponibilidad con el asesor"
	}

	// Advertencia si cantidad < mínimo
	if qty < b.MinQty {
		result["bajo_minimo"] = true
		result["mensaje_minimo"] = "La cantidad solicitada está por debajo del mínimo de " + strconv.Itoa(b.MinQty) + " unidades"
	}

	return result
}

// computePriceForQty retorna el precio unitario correcto para la cantidad dada,
// usando la tabla price_breaks del blank. Si qty no alcanza ningún tier, devuelve base_price.
func computePriceForQty(b *models.Blank, qty int) int {
	var breaks []struct {
		Qty       int `json:"qty"`
		UnitPrice int `json:"unit_price"`
	}
	if err := json.Unmarshal(b.PriceBreaks, &breaks); err != nil || len(breaks) == 0 {
		return b.BasePrice
	}
	// Ordenar de mayor a menor para encontrar el tier más alto que aplica
	sort.Slice(breaks, func(i, j int) bool { return breaks[i].Qty > breaks[j].Qty })
	for _, br := range breaks {
		if qty >= br.Qty {
			return br.UnitPrice
		}
	}
	return b.BasePrice
}
