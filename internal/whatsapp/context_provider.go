package whatsapp

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/repository"
)

const (
	contextCacheTTL       = 5 * time.Minute
	defaultAsesorPhone    = "+50686091954"
)

// waContextProvider carga contexto dinámico desde BD (tecnologías, materiales, TelAsesor)
// con cache de 5 minutos para evitar hits innecesarios a la DB.
type waContextProvider struct {
	techRepo      *repository.TechnologyRepository
	matRepo       *repository.MaterialRepository
	sysConfigRepo *repository.SystemConfigRepository
	mu            sync.RWMutex
	cachedContext string
	asesorPhone   string
	fetchedAt     time.Time
}

// NewWAContextProvider crea un provider con repositorios conectados a la BD.
func NewWAContextProvider() *waContextProvider {
	return &waContextProvider{
		techRepo:      repository.NewTechnologyRepository(),
		matRepo:       repository.NewMaterialRepository(),
		sysConfigRepo: repository.NewSystemConfigRepository(),
	}
}

// GetDynamicContext retorna el bloque de contexto dinámico para inyectar al system prompt.
// Incluye IDs exactos de tecnologías y materiales para que Gemini pueda usarlos en tools.
// El resultado se cachea por 5 minutos.
func (p *waContextProvider) GetDynamicContext() string {
	p.mu.RLock()
	if p.cachedContext != "" && time.Since(p.fetchedAt) < contextCacheTTL {
		ctx := p.cachedContext
		p.mu.RUnlock()
		return ctx
	}
	p.mu.RUnlock()

	content := p.buildContext()

	p.mu.Lock()
	p.cachedContext = content
	p.fetchedAt = time.Now()
	p.mu.Unlock()

	return content
}

// GetAsesorPhone retorna el teléfono del asesor desde system_config (TelAsesor).
// Fallback: +50686091954. El valor se cachea junto con el contexto dinámico.
func (p *waContextProvider) GetAsesorPhone() string {
	p.mu.RLock()
	phone := p.asesorPhone
	p.mu.RUnlock()

	if phone != "" {
		return phone
	}

	// Si no está en cache, leerlo ahora
	if cfg, err := p.sysConfigRepo.FindByKey("TelAsesor"); err == nil && cfg.ConfigValue != "" {
		phone = cfg.ConfigValue
	} else {
		phone = defaultAsesorPhone
	}

	p.mu.Lock()
	p.asesorPhone = phone
	p.mu.Unlock()

	return phone
}

func (p *waContextProvider) buildContext() string {
	var b strings.Builder

	techs, err := p.techRepo.FindAll()
	if err != nil {
		slog.Error("waContextProvider: error cargando tecnologías", "error", err)
	} else {
		b.WriteString("\n\n## Tecnologías disponibles (IDs exactos para calcular_cotizacion):\n")
		for _, t := range techs {
			b.WriteString(fmt.Sprintf("- technology_id=%d, code=%s, nombre=\"%s\"\n", t.ID, t.Code, t.Name))
		}
	}

	mats, err := p.matRepo.FindAll()
	if err != nil {
		slog.Error("waContextProvider: error cargando materiales", "error", err)
	} else {
		b.WriteString("\n## Materiales disponibles (IDs exactos para calcular_cotizacion):\n")
		for _, m := range mats {
			b.WriteString(fmt.Sprintf("- material_id=%d, nombre=\"%s\", categoría=%s\n", m.ID, m.Name, m.Category))
		}
	}

	// Teléfono del asesor
	asesorPhone := defaultAsesorPhone
	if cfg, err := p.sysConfigRepo.FindByKey("TelAsesor"); err == nil && cfg.ConfigValue != "" {
		asesorPhone = cfg.ConfigValue
	}

	// Costo de vectorización
	costoVectorizacion := "10000"
	if cfg, err := p.sysConfigRepo.FindByKey("CostoVectorizacion"); err == nil && cfg.ConfigValue != "" {
		costoVectorizacion = cfg.ConfigValue
	}

	// Actualizar cache del teléfono al mismo tiempo que el contexto
	p.mu.Lock()
	p.asesorPhone = asesorPhone
	p.mu.Unlock()

	b.WriteString(fmt.Sprintf("\n## Configuración operativa:\n- Teléfono asesor para escalado: %s\n- Costo de vectorización: ₡%s\n", asesorPhone, costoVectorizacion))

	return b.String()
}
