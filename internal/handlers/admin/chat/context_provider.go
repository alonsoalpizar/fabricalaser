package chat

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/repository"
)

const adminContextCacheTTL = 5 * time.Minute

// ContextProvider arma el bloque de contexto dinámico que se inyecta al system
// prompt en cada llamada al modelo. Cachea 5 minutos para evitar hits a DB
// en cada mensaje.
//
// A diferencia del WAContextProvider del paquete whatsapp, este context es
// más rico — incluye tarifas, descuentos por volumen y tipos de grabado con
// factores, porque el gestor puede pedirle al asistente que explique cómo
// llegó al precio.
type ContextProvider struct {
	techRepo       *repository.TechnologyRepository
	matRepo        *repository.MaterialRepository
	engraveRepo    *repository.EngraveTypeRepository
	rateRepo       *repository.TechRateRepository
	discountRepo   *repository.VolumeDiscountRepository
	blankRepo      *repository.BlankRepository
	sysConfigRepo  *repository.SystemConfigRepository

	mu       sync.RWMutex
	cached   string
	fetched  time.Time
}

// NewContextProvider construye el provider con todos los repos.
func NewContextProvider() *ContextProvider {
	return &ContextProvider{
		techRepo:      repository.NewTechnologyRepository(),
		matRepo:       repository.NewMaterialRepository(),
		engraveRepo:   repository.NewEngraveTypeRepository(),
		rateRepo:      repository.NewTechRateRepository(),
		discountRepo:  repository.NewVolumeDiscountRepository(),
		blankRepo:     repository.NewBlankRepository(),
		sysConfigRepo: repository.NewSystemConfigRepository(),
	}
}

// Get retorna el bloque de contexto formateado para inyectar al prompt.
func (p *ContextProvider) Get() string {
	p.mu.RLock()
	if p.cached != "" && time.Since(p.fetched) < adminContextCacheTTL {
		c := p.cached
		p.mu.RUnlock()
		return c
	}
	p.mu.RUnlock()

	content := p.build()
	p.mu.Lock()
	p.cached = content
	p.fetched = time.Now()
	p.mu.Unlock()
	return content
}

func (p *ContextProvider) build() string {
	var b strings.Builder

	b.WriteString("\n\n## DATOS DE LA BASE DE DATOS (en tiempo real, cache 5min)\n")

	// Tecnologías
	if techs, err := p.techRepo.FindAll(); err == nil {
		b.WriteString("\n### Tecnologías disponibles (IDs exactos para calcular_cotizacion):\n")
		for _, t := range techs {
			if !t.IsActive {
				continue
			}
			premiumStr := ""
			if t.UVPremiumFactor > 0 {
				premiumStr = fmt.Sprintf(", premium_uv=%.2f", t.UVPremiumFactor)
			}
			b.WriteString(fmt.Sprintf("- technology_id=%d, code=%s, nombre=\"%s\"%s\n", t.ID, t.Code, t.Name, premiumStr))
		}
	} else {
		slog.Error("admin_chat ContextProvider: error cargando tecnologías", "error", err)
	}

	// Materiales (con cuttable y factor — el agente lo necesita para explicar)
	if mats, err := p.matRepo.FindAll(); err == nil {
		b.WriteString("\n### Materiales disponibles (IDs exactos para calcular_cotizacion):\n")
		for _, m := range mats {
			if !m.IsActive {
				continue
			}
			cuttableStr := ""
			if m.IsCuttable {
				cuttableStr = ", cortable=SI"
			} else {
				cuttableStr = ", cortable=NO"
			}
			b.WriteString(fmt.Sprintf("- material_id=%d, nombre=\"%s\", categoría=%s, factor=%.2f%s\n",
				m.ID, m.Name, m.Category, m.Factor, cuttableStr))
		}
	} else {
		slog.Error("admin_chat ContextProvider: error cargando materiales", "error", err)
	}

	// Tipos de grabado con factores
	if engraves, err := p.engraveRepo.FindAll(); err == nil {
		b.WriteString("\n### Tipos de grabado disponibles:\n")
		for _, e := range engraves {
			if !e.IsActive {
				continue
			}
			b.WriteString(fmt.Sprintf("- engrave_type_id=%d, nombre=\"%s\", factor_tiempo=%.2f\n",
				e.ID, e.Name, e.Factor))
		}
	}

	// Descuentos por volumen
	if discounts, err := p.discountRepo.FindAll(); err == nil && len(discounts) > 0 {
		b.WriteString("\n### Descuentos por volumen aplicados automáticamente:\n")
		for _, d := range discounts {
			if !d.IsActive {
				continue
			}
			rng := fmt.Sprintf("%d+", d.MinQty)
			if d.MaxQty != nil {
				rng = fmt.Sprintf("%d-%d", d.MinQty, *d.MaxQty)
			}
			b.WriteString(fmt.Sprintf("- %s unidades → %.0f%% descuento\n", rng, d.DiscountPct*100))
		}
	}

	// Configuración operativa relevante
	b.WriteString("\n### Configuración operativa:\n")
	if cfg, err := p.sysConfigRepo.FindByKey("CostoVectorizacion"); err == nil && cfg.ConfigValue != "" {
		b.WriteString(fmt.Sprintf("- Costo de vectorización (sumar al total si el cliente NO trae SVG): ₡%s\n", cfg.ConfigValue))
	}
	if cfg, err := p.sysConfigRepo.FindByKey("TelAsesor"); err == nil && cfg.ConfigValue != "" {
		b.WriteString(fmt.Sprintf("- Teléfono asesor de ventas (FYI): %s\n", cfg.ConfigValue))
	}

	// Catálogo de blanks — exposición rica para que Gemini matchee bien.
	// Incluye descripción completa, precio base, price_breaks por volumen y
	// una línea de alias semánticos según categoría para que el modelo
	// asocie sinónimos comunes ("acrílicos redondos", "discos", etc.) con
	// el blank correcto.
	if blanks, err := p.blankRepo.FindAll(); err == nil && len(blanks) > 0 {
		b.WriteString("\n### Catálogo de blanks (productos pre-fabricados — SIEMPRE preferir consultar_blank antes de calcular_cotizacion cuando aplique):\n")
		for _, blank := range blanks {
			dim := ""
			if blank.Dimensions != nil {
				dim = " (" + *blank.Dimensions + ")"
			}
			b.WriteString(fmt.Sprintf("\n- **blank_id=%d** | categoria=%s | nombre=\"%s\"%s | min_qty=%d\n",
				blank.ID, blank.Category, blank.Name, dim, blank.MinQty))
			if blank.Description != "" {
				b.WriteString(fmt.Sprintf("  Descripción: %s\n", blank.Description))
			}
			b.WriteString(fmt.Sprintf("  Precio base: ₡%d/u (a min_qty=%d)\n", blank.BasePrice, blank.MinQty))
			if priceBreaks := summarizePriceBreaks(blank.PriceBreaks); priceBreaks != "" {
				b.WriteString(fmt.Sprintf("  Volumen: %s\n", priceBreaks))
			}
			if aliases := summarizeAliases(blank.Aliases); aliases != "" {
				b.WriteString(fmt.Sprintf("  Sinónimos comunes: %s\n", aliases))
			}
		}
	}

	return b.String()
}

// summarizePriceBreaks convierte el JSONB de price_breaks a una línea legible
// como "25u=₡240/u, 50u=₡220/u, 100u=₡180/u". Si el formato es inválido, vacío.
func summarizePriceBreaks(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var breaks []struct {
		Qty       int `json:"qty"`
		UnitPrice int `json:"unit_price"`
	}
	if err := json.Unmarshal(raw, &breaks); err != nil || len(breaks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(breaks))
	for _, br := range breaks {
		parts = append(parts, fmt.Sprintf("%du=₡%d/u", br.Qty, br.UnitPrice))
	}
	return strings.Join(parts, ", ")
}

// summarizeAliases convierte el JSONB aliases del blank (array de strings)
// a una línea separada por comas. Si el formato es inválido o vacío, retorna "".
//
// Reemplaza la versión hardcoded por categoría — ahora cada blank trae sus
// propios aliases editables desde la UI admin (migration 030).
func summarizeAliases(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var aliases []string
	if err := json.Unmarshal(raw, &aliases); err != nil || len(aliases) == 0 {
		return ""
	}
	return strings.Join(aliases, ", ")
}
