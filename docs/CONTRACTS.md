# CONTRACTS.md - Especificaciones Exactas para Refactorización

**Fecha:** 2026-02-26
**Versión:** 1.1 (actualizado con correcciones aprobadas)

> **REGLA OBLIGATORIA PARA SUBAGENTES:**
> NO inventar nombres de campos, NO cambiar tipos, NO agregar campos que no estén en este contrato.
> Si necesita algo que no está aquí, REPORTAR para actualizar el contrato primero.

---

## Convenciones del Proyecto (Referencia)

### Modelos Go
- Package: `models`
- ID: `uint` con `gorm:"primaryKey" json:"id"`
- Campos en CamelCase, JSON tags en snake_case
- Timestamps: `CreatedAt time.Time`, `UpdatedAt time.Time`
- Campos opcionales: punteros `*string` con `json:"campo,omitempty"`
- Booleanos: `IsActive bool` con `gorm:"default:true" json:"is_active"`
- Método `TableName() string` retorna nombre en snake_case plural

### Migraciones SQL
- Archivos: `NNN_nombre.sql` (siguiente: `011_system_config.sql`)
- `CREATE TABLE IF NOT EXISTS`
- `SERIAL PRIMARY KEY` para IDs
- `DECIMAL(10,4)` para montos, `DECIMAL(5,4)` para factores
- `TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `COMMENT ON TABLE` y `COMMENT ON COLUMN`

### Repositories
- Package: `repository`
- Struct: `NombreRepository` con `db *gorm.DB`
- Constructor: `NewNombreRepository() *NombreRepository`
- Métodos estándar: `FindAll()`, `FindByID(id uint)`, `Create()`, `Update()`, `Delete(id uint)`

### Handlers
- Request structs inline en cada método
- Response: `map[string]interface{}{"success": true, "data": ...}`
- Errores: `respondError(w, status, "CODE", "mensaje")`

### Rutas
- Admin: `/api/v1/admin/`
- Config (público): `/api/v1/config/`

---

## TABLA 1: system_config

### 1.1 Migración SQL

**Archivo:** `migrations/011_system_config.sql`

```sql
-- Migration 011: System Configuration table
-- FabricaLaser - Configuración general del sistema

CREATE TABLE IF NOT EXISTS system_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(100) UNIQUE NOT NULL,
    config_value TEXT NOT NULL,
    value_type VARCHAR(20) NOT NULL DEFAULT 'string',
    category VARCHAR(50) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_config_key ON system_config(config_key);
CREATE INDEX IF NOT EXISTS idx_system_config_category ON system_config(category);
CREATE INDEX IF NOT EXISTS idx_system_config_active ON system_config(is_active) WHERE is_active = true;

COMMENT ON TABLE system_config IS 'Configuración general del sistema (valores hardcodeados movidos a BD)';
COMMENT ON COLUMN system_config.config_key IS 'Clave única de configuración';
COMMENT ON COLUMN system_config.config_value IS 'Valor como texto (parsear según value_type)';
COMMENT ON COLUMN system_config.value_type IS 'Tipo: string, number, boolean, json';
COMMENT ON COLUMN system_config.category IS 'Categoría: speeds, times, complexity, pricing, quotes';
```

### 1.2 Modelo Go

**Archivo:** `internal/models/system_config.go`

```go
package models

import (
	"time"
)

type SystemConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ConfigKey   string    `gorm:"column:config_key;type:varchar(100);uniqueIndex;not null" json:"config_key"`
	ConfigValue string    `gorm:"column:config_value;type:text;not null" json:"config_value"`
	ValueType   string    `gorm:"column:value_type;type:varchar(20);not null;default:'string'" json:"value_type"`
	Category    string    `gorm:"column:category;type:varchar(50);not null" json:"category"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SystemConfig) TableName() string {
	return "system_config"
}
```

### 1.3 Repository

**Archivo:** `internal/repository/system_config_repository.go`

```go
package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrSystemConfigNotFound = errors.New("configuración no encontrada")

type SystemConfigRepository struct {
	db *gorm.DB
}

func NewSystemConfigRepository() *SystemConfigRepository {
	return &SystemConfigRepository{
		db: database.Get(),
	}
}

func (r *SystemConfigRepository) FindAll() ([]models.SystemConfig, error)
func (r *SystemConfigRepository) FindByID(id uint) (*models.SystemConfig, error)
func (r *SystemConfigRepository) FindByKey(key string) (*models.SystemConfig, error)
func (r *SystemConfigRepository) FindByCategory(category string) ([]models.SystemConfig, error)
func (r *SystemConfigRepository) Create(config *models.SystemConfig) error
func (r *SystemConfigRepository) Update(config *models.SystemConfig) error
func (r *SystemConfigRepository) Delete(id uint) error
```

### 1.4 Endpoints API

#### GET /api/v1/admin/system-config
Lista todas las configuraciones.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "config_key": "base_cut_speed",
      "config_value": "20",
      "value_type": "number",
      "category": "speeds",
      "description": "Velocidad base de corte (mm/min)",
      "is_active": true
    }
  ]
}
```

#### GET /api/v1/admin/system-config/{id}
Obtiene una configuración por ID.

#### PUT /api/v1/admin/system-config/{id}
Actualiza una configuración.

**Request:**
```json
{
  "config_value": "25",
  "description": "Velocidad actualizada"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "config_key": "base_cut_speed",
    "config_value": "25"
  }
}
```

#### POST /api/v1/admin/system-config
Crea una nueva configuración.

**Request:**
```json
{
  "config_key": "new_setting",
  "config_value": "100",
  "value_type": "number",
  "category": "pricing",
  "description": "Nueva configuración"
}
```

### 1.5 Seed Data

**Archivo:** `seeds/002_system_config.sql`

```sql
-- Seed 002: System Configuration
-- Valores hardcodeados movidos a BD

INSERT INTO system_config (config_key, config_value, value_type, category, description) VALUES
-- Velocidades base (de time_estimator.go)
('base_engrave_area_speed', '500', 'number', 'speeds', 'Velocidad base grabado área (mm²/min)'),
('base_engrave_line_speed', '100', 'number', 'speeds', 'Velocidad base grabado línea (mm/min)'),
('base_cut_speed', '20', 'number', 'speeds', 'Velocidad base corte (mm/min)'),

-- Tiempos (de time_estimator.go)
('setup_time_minutes', '5', 'number', 'times', 'Tiempo setup por trabajo (min)'),

-- Complejidad (de calculator.go)
('complexity_auto_approve', '6.0', 'number', 'complexity', 'Factor máximo para auto-aprobación'),
('complexity_needs_review', '12.0', 'number', 'complexity', 'Factor máximo para revisión manual'),

-- Cotizaciones (de calculator.go)
('quote_validity_days', '7', 'number', 'quotes', 'Días de validez de cotización'),

-- Pricing value-based (de calculator.go)
('min_value_base', '2575', 'number', 'pricing', 'Precio mínimo base (CRC)'),
('price_per_mm2', '0.515', 'number', 'pricing', 'Precio por mm² (CRC)'),
('min_area_mm2', '100', 'number', 'pricing', 'Área mínima para cobrar (mm²)')
ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    description = EXCLUDED.description;
```

---

## TABLA 2: tech_material_speeds

### 2.1 Migración SQL

**Archivo:** `migrations/012_tech_material_speeds.sql`

```sql
-- Migration 012: Technology-Material-Thickness Speeds Matrix
-- FabricaLaser - Velocidades por combinación tecnología/material/grosor

CREATE TABLE IF NOT EXISTS tech_material_speeds (
    id SERIAL PRIMARY KEY,
    technology_id INTEGER NOT NULL REFERENCES technologies(id) ON DELETE CASCADE,
    material_id INTEGER NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    thickness DECIMAL(5,2) NOT NULL,
    cut_speed_mm_min DECIMAL(10,2),
    engrave_speed_mm_min DECIMAL(10,2),
    is_compatible BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(technology_id, material_id, thickness)
);

CREATE INDEX IF NOT EXISTS idx_tms_technology ON tech_material_speeds(technology_id);
CREATE INDEX IF NOT EXISTS idx_tms_material ON tech_material_speeds(material_id);
CREATE INDEX IF NOT EXISTS idx_tms_thickness ON tech_material_speeds(thickness);
CREATE INDEX IF NOT EXISTS idx_tms_compatible ON tech_material_speeds(is_compatible) WHERE is_compatible = true;
CREATE INDEX IF NOT EXISTS idx_tms_active ON tech_material_speeds(is_active) WHERE is_active = true;

COMMENT ON TABLE tech_material_speeds IS 'Matriz de velocidades por combinación tecnología/material/grosor';
COMMENT ON COLUMN tech_material_speeds.cut_speed_mm_min IS 'Velocidad de corte mm/min (NULL si no corta)';
COMMENT ON COLUMN tech_material_speeds.engrave_speed_mm_min IS 'Velocidad de grabado mm/min (NULL si no graba)';
COMMENT ON COLUMN tech_material_speeds.is_compatible IS 'Si esta combinación es posible';
```

### 2.2 Modelo Go

**Archivo:** `internal/models/tech_material_speed.go`

```go
package models

import (
	"time"
)

type TechMaterialSpeed struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	TechnologyID     uint      `gorm:"not null;index" json:"technology_id"`
	MaterialID       uint      `gorm:"not null;index" json:"material_id"`
	Thickness        float64   `gorm:"type:decimal(5,2);not null" json:"thickness"`
	CutSpeedMmMin    *float64  `gorm:"column:cut_speed_mm_min;type:decimal(10,2)" json:"cut_speed_mm_min"`
	EngraveSpeedMmMin *float64 `gorm:"column:engrave_speed_mm_min;type:decimal(10,2)" json:"engrave_speed_mm_min"`
	IsCompatible     bool      `gorm:"default:true" json:"is_compatible"`
	Notes            *string   `gorm:"type:text" json:"notes,omitempty"`
	IsActive         bool      `gorm:"default:true" json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Relations
	Technology Technology `gorm:"foreignKey:TechnologyID" json:"technology,omitempty"`
	Material   Material   `gorm:"foreignKey:MaterialID" json:"material,omitempty"`
}

func (TechMaterialSpeed) TableName() string {
	return "tech_material_speeds"
}
```

### 2.3 Repository

**Archivo:** `internal/repository/tech_material_speed_repository.go`

```go
package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrTechMaterialSpeedNotFound = errors.New("configuración de velocidad no encontrada")

type TechMaterialSpeedRepository struct {
	db *gorm.DB
}

func NewTechMaterialSpeedRepository() *TechMaterialSpeedRepository {
	return &TechMaterialSpeedRepository{
		db: database.Get(),
	}
}

func (r *TechMaterialSpeedRepository) FindAll() ([]models.TechMaterialSpeed, error)
func (r *TechMaterialSpeedRepository) FindByID(id uint) (*models.TechMaterialSpeed, error)
func (r *TechMaterialSpeedRepository) FindByTechAndMaterial(techID, materialID uint) ([]models.TechMaterialSpeed, error)
func (r *TechMaterialSpeedRepository) FindByTechMaterialThickness(techID, materialID uint, thickness float64) (*models.TechMaterialSpeed, error)
func (r *TechMaterialSpeedRepository) FindCompatibleTechnologies(materialID uint, thickness float64) ([]models.TechMaterialSpeed, error)
func (r *TechMaterialSpeedRepository) Create(speed *models.TechMaterialSpeed) error
func (r *TechMaterialSpeedRepository) Update(speed *models.TechMaterialSpeed) error
func (r *TechMaterialSpeedRepository) Delete(id uint) error
func (r *TechMaterialSpeedRepository) BulkCreate(speeds []models.TechMaterialSpeed) error
```

### 2.4 Endpoints API

#### GET /api/v1/admin/tech-material-speeds
Lista todas las configuraciones de velocidad.

**Query params:**
- `technology_id` (opcional): Filtrar por tecnología
- `material_id` (opcional): Filtrar por material

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "technology_id": 1,
      "technology": {"id": 1, "name": "Láser CO2", "code": "CO2"},
      "material_id": 2,
      "material": {"id": 2, "name": "Acrílico transparente"},
      "thickness": 3.0,
      "cut_speed_mm_min": 25.0,
      "engrave_speed_mm_min": 500.0,
      "is_compatible": true,
      "is_active": true
    }
  ]
}
```

#### GET /api/v1/admin/tech-material-speeds/{id}
Obtiene una configuración por ID.

#### POST /api/v1/admin/tech-material-speeds
Crea una nueva configuración de velocidad.

**Request:**
```json
{
  "technology_id": 1,
  "material_id": 2,
  "thickness": 5.0,
  "cut_speed_mm_min": 15.0,
  "engrave_speed_mm_min": 500.0,
  "is_compatible": true,
  "notes": "Velocidad reducida para 5mm"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": 5
  }
}
```

#### PUT /api/v1/admin/tech-material-speeds/{id}
Actualiza una configuración.

**Request:**
```json
{
  "cut_speed_mm_min": 18.0,
  "notes": "Ajustado después de pruebas"
}
```

#### DELETE /api/v1/admin/tech-material-speeds/{id}
Elimina (soft delete) una configuración.

#### POST /api/v1/admin/tech-material-speeds/bulk
Crea múltiples configuraciones en lote.

**Request:**
```json
{
  "speeds": [
    {"technology_id": 1, "material_id": 2, "thickness": 3.0, "cut_speed_mm_min": 25.0, "engrave_speed_mm_min": 500.0},
    {"technology_id": 1, "material_id": 2, "thickness": 5.0, "cut_speed_mm_min": 15.0, "engrave_speed_mm_min": 500.0},
    {"technology_id": 1, "material_id": 2, "thickness": 10.0, "cut_speed_mm_min": 8.0, "engrave_speed_mm_min": 500.0}
  ]
}
```

#### GET /api/v1/config/compatible-options
**Endpoint PÚBLICO** para el cotizador - obtiene opciones compatibles.

**Query params:**
- `material_id` (requerido): ID del material seleccionado
- `thickness` (opcional): Grosor seleccionado

**Response:**
```json
{
  "success": true,
  "data": {
    "technologies": [
      {
        "id": 1,
        "name": "Láser CO2",
        "code": "CO2",
        "thicknesses": [3.0, 5.0, 6.0, 10.0],
        "can_cut": true,
        "can_engrave": true
      }
    ]
  }
}
```

### 2.5 Seed Data

**Archivo:** `seeds/003_tech_material_speeds.sql`

> **NOTA:** Estos son valores PLACEHOLDER. Las velocidades reales deben calibrarse
> con pruebas en el taller. El admin puede agregar más combinaciones desde la UI.

```sql
-- Seed 003: Tech Material Speeds Matrix
-- PLACEHOLDER: Velocidades de ejemplo, calibrar con pruebas reales
-- El admin puede agregar más combinaciones desde /admin/config/speeds.html

-- =====================================================
-- EJEMPLO MÍNIMO: CO2 + Madera/MDF 3mm
-- (único registro de ejemplo para validar que el sistema funciona)
-- =====================================================
INSERT INTO tech_material_speeds (technology_id, material_id, thickness, cut_speed_mm_min, engrave_speed_mm_min, is_compatible, notes)
SELECT t.id, m.id, 3.0, 30.0, 600.0, true, 'PLACEHOLDER - calibrar con pruebas reales'
FROM technologies t, materials m
WHERE t.code = 'CO2' AND m.name = 'Madera / MDF'
ON CONFLICT (technology_id, material_id, thickness) DO UPDATE SET
    cut_speed_mm_min = EXCLUDED.cut_speed_mm_min,
    engrave_speed_mm_min = EXCLUDED.engrave_speed_mm_min,
    notes = EXCLUDED.notes;

-- =====================================================
-- EJEMPLO: Material sin grosor (thickness=0)
-- Para materiales como Metal, Cuero que no tienen grosores estándar
-- =====================================================
INSERT INTO tech_material_speeds (technology_id, material_id, thickness, cut_speed_mm_min, engrave_speed_mm_min, is_compatible, notes)
SELECT t.id, m.id, 0, NULL, 600.0, true, 'PLACEHOLDER - solo grabado, sin corte'
FROM technologies t, materials m
WHERE t.code = 'FIBRA' AND m.name = 'Metal con coating'
ON CONFLICT (technology_id, material_id, thickness) DO UPDATE SET
    cut_speed_mm_min = EXCLUDED.cut_speed_mm_min,
    engrave_speed_mm_min = EXCLUDED.engrave_speed_mm_min,
    notes = EXCLUDED.notes;

-- =====================================================
-- EJEMPLO: Combinación incompatible
-- CO2 no puede trabajar con Metal
-- =====================================================
INSERT INTO tech_material_speeds (technology_id, material_id, thickness, cut_speed_mm_min, engrave_speed_mm_min, is_compatible, notes)
SELECT t.id, m.id, 0, NULL, NULL, false, 'CO2 no trabaja con metal - usar FIBRA o MOPA'
FROM technologies t, materials m
WHERE t.code = 'CO2' AND m.name = 'Metal con coating'
ON CONFLICT (technology_id, material_id, thickness) DO UPDATE SET
    is_compatible = EXCLUDED.is_compatible,
    notes = EXCLUDED.notes;
```

> **Materiales sin grosor:** Usar `thickness = 0` para materiales que no tienen
> grosores estándar definidos (Metal con coating, Cuero / Piel, Cerámica).

---

## Rutas Admin Actualizadas

**Archivo:** `internal/handlers/router.go` - Agregar en sección admin:

```go
// System Config CRUD
r.Get("/system-config", adminHandler.GetSystemConfigs)
r.Get("/system-config/{id}", adminHandler.GetSystemConfig)
r.Post("/system-config", adminHandler.CreateSystemConfig)
r.Put("/system-config/{id}", adminHandler.UpdateSystemConfig)
r.Delete("/system-config/{id}", adminHandler.DeleteSystemConfig)

// Tech Material Speeds CRUD
r.Get("/tech-material-speeds", adminHandler.GetTechMaterialSpeeds)
r.Get("/tech-material-speeds/{id}", adminHandler.GetTechMaterialSpeed)
r.Post("/tech-material-speeds", adminHandler.CreateTechMaterialSpeed)
r.Post("/tech-material-speeds/bulk", adminHandler.BulkCreateTechMaterialSpeeds)
r.Put("/tech-material-speeds/{id}", adminHandler.UpdateTechMaterialSpeed)
r.Delete("/tech-material-speeds/{id}", adminHandler.DeleteTechMaterialSpeed)
```

**Rutas config públicas:**

```go
r.Get("/compatible-options", configHandler.GetCompatibleOptions)
```

---

## Frontend Admin

### general.html (System Config)
- Página en: `/web/admin/config/general.html`
- Endpoint: `/api/v1/admin/system-config`
- Agrupar por categoría (speeds, times, complexity, pricing, quotes)
- Formulario de edición inline o modal

### speeds.html (Tech Material Speeds)
- Página en: `/web/admin/config/speeds.html`
- Endpoint: `/api/v1/admin/tech-material-speeds`
- Filtros: Tecnología, Material
- Tabla con columnas: Tecnología, Material, Grosor, Corte mm/min, Grabado mm/min, Compatible, Acciones
- Modal para crear/editar
- Botón bulk para agregar múltiples grosores

---

## Checklist de Verificación

Después de cada implementación, verificar:

- [ ] Nombres de columnas SQL coinciden exactamente
- [ ] JSON tags en Go coinciden con columnas SQL (snake_case)
- [ ] Tipos de datos coinciden (DECIMAL vs float64, etc.)
- [ ] Responses tienen estructura `{"success": true, "data": ...}`
- [ ] Errores tienen estructura `{"success": false, "error": {"code": "X", "message": "Y"}}`
- [ ] Rutas siguen patrón `/api/v1/admin/recurso` y `/api/v1/config/recurso`
- [ ] Repository usa `database.Get()` para obtener conexión
- [ ] Soft delete usa `is_active = false`, no DELETE físico
