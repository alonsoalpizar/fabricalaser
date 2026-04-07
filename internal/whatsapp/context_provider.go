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
	contextCacheTTL    = 5 * time.Minute
	defaultAsesorPhone = "+50686091954"
	defaultMaxMsgs     = 20
)

// WAContextProvider carga contexto dinámico desde BD (tecnologías, materiales, TelAsesor)
// con cache de 5 minutos para evitar hits innecesarios a la DB.
type WAContextProvider struct {
	techRepo      *repository.TechnologyRepository
	matRepo       *repository.MaterialRepository
	sysConfigRepo *repository.SystemConfigRepository
	blankRepo     *repository.BlankRepository
	mu                   sync.RWMutex
	cachedContext        string
	asesorPhone          string
	asesorTelegramChatID int64
	maxMensajes          int
	fetchedAt            time.Time
}

// NewWAContextProvider crea un provider con repositorios conectados a la BD.
func NewWAContextProvider() *WAContextProvider {
	return &WAContextProvider{
		techRepo:      repository.NewTechnologyRepository(),
		matRepo:       repository.NewMaterialRepository(),
		sysConfigRepo: repository.NewSystemConfigRepository(),
		blankRepo:     repository.NewBlankRepository(),
	}
}

// GetDynamicContext retorna el bloque de contexto dinámico para inyectar al system prompt.
// Incluye IDs exactos de tecnologías y materiales para que Gemini pueda usarlos en tools.
// El resultado se cachea por 5 minutos.
func (p *WAContextProvider) GetDynamicContext() string {
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

// GetMaxMensajesDia retorna el límite diario de mensajes por número desde system_config.
// Fallback: 20. Si no está en cache, lee de la DB directamente.
func (p *WAContextProvider) GetMaxMensajesDia() int {
	p.mu.RLock()
	v := p.maxMensajes
	p.mu.RUnlock()
	if v > 0 {
		return v
	}

	// Si no está en cache, leerlo ahora de la DB
	maxMensajes := defaultMaxMsgs
	if cfg, err := p.sysConfigRepo.FindByKey("wa_max_mensajes_dia"); err == nil && cfg.ConfigValue != "" {
		if n, err := fmt.Sscanf(cfg.ConfigValue, "%d", &maxMensajes); err != nil || n != 1 {
			maxMensajes = defaultMaxMsgs
		}
	}

	p.mu.Lock()
	p.maxMensajes = maxMensajes
	p.mu.Unlock()

	return maxMensajes
}

// GetAsesorPhone retorna el teléfono del asesor desde system_config (TelAsesor).
// Fallback: +50686091954. El valor se cachea junto con el contexto dinámico.
func (p *WAContextProvider) GetAsesorPhone() string {
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

// GetAsesorTelegramChatID retorna el chat ID de Telegram del asesor desde system_config.
// Retorna 0 si no está configurado. El valor se cachea junto con el contexto dinámico.
func (p *WAContextProvider) GetAsesorTelegramChatID() int64 {
	p.mu.RLock()
	chatID := p.asesorTelegramChatID
	p.mu.RUnlock()

	if chatID != 0 {
		return chatID
	}

	// Si no está en cache, leerlo ahora
	if cfg, err := p.sysConfigRepo.FindByKey("TelegramAsesorChatID"); err == nil && cfg.ConfigValue != "" {
		if _, err := fmt.Sscanf(cfg.ConfigValue, "%d", &chatID); err == nil {
			p.mu.Lock()
			p.asesorTelegramChatID = chatID
			p.mu.Unlock()
		}
	}

	return chatID
}

func (p *WAContextProvider) buildContext() string {
	var b strings.Builder

	techs, err := p.techRepo.FindAll()
	if err != nil {
		slog.Error("WAContextProvider: error cargando tecnologías", "error", err)
	} else {
		b.WriteString("\n\n## Tecnologías disponibles (IDs exactos para calcular_cotizacion):\n")
		for _, t := range techs {
			b.WriteString(fmt.Sprintf("- technology_id=%d, code=%s, nombre=\"%s\"\n", t.ID, t.Code, t.Name))
		}
	}

	mats, err := p.matRepo.FindAll()
	if err != nil {
		slog.Error("WAContextProvider: error cargando materiales", "error", err)
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

	// Límite diario de mensajes
	maxMensajes := defaultMaxMsgs
	if cfg, err := p.sysConfigRepo.FindByKey("wa_max_mensajes_dia"); err == nil && cfg.ConfigValue != "" {
		if v, err := fmt.Sscanf(cfg.ConfigValue, "%d", &maxMensajes); err != nil || v != 1 {
			maxMensajes = defaultMaxMsgs
		}
	}

	// Chat ID de Telegram del asesor
	var asesorTgChatID int64
	if cfg, err := p.sysConfigRepo.FindByKey("TelegramAsesorChatID"); err == nil && cfg.ConfigValue != "" {
		if v, err := fmt.Sscanf(cfg.ConfigValue, "%d", &asesorTgChatID); err != nil || v != 1 {
			asesorTgChatID = 0
		}
	}

	// Actualizar cache del teléfono, Telegram chat ID y límite al mismo tiempo que el contexto
	p.mu.Lock()
	p.asesorPhone = asesorPhone
	p.asesorTelegramChatID = asesorTgChatID
	p.maxMensajes = maxMensajes
	p.mu.Unlock()

	b.WriteString(fmt.Sprintf("\n## Configuración operativa:\n- Teléfono asesor para escalado: %s\n- Costo de vectorización: ₡%s\n", asesorPhone, costoVectorizacion))

	// Catálogo de blanks (resumen para que el agente sepa qué categorías existen)
	blanks, err := p.blankRepo.FindAll()
	if err != nil {
		slog.Error("WAContextProvider: error cargando blanks", "error", err)
	} else if len(blanks) > 0 {
		b.WriteString("\n## Catálogo de blanks disponibles (usar consultar_blank para precios en tiempo real):\n")
		for _, blank := range blanks {
			dim := ""
			if blank.Dimensions != nil {
				dim = " (" + *blank.Dimensions + ")"
			}
			b.WriteString(fmt.Sprintf("- blank_id=%d, categoria=%s, nombre=\"%s\"%s, min_qty=%d\n",
				blank.ID, blank.Category, blank.Name, dim, blank.MinQty))
		}
	}

	return b.String()
}
