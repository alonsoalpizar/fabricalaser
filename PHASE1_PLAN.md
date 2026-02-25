# Phase 1: Motor SVG + Cotizador - Plan en Mini-Etapas

## Confirmado: Sin Hardcode

Todas las configuraciones de pricing vienen de la base de datos:

```
tech_rates       → cost_per_min_engrave, cost_per_min_cut, margin_percent
materials        → factor (1.0 - 1.8)
engrave_types    → factor (1.0 - 3.0), speed_multiplier
volume_discounts → discount_pct (0% - 20%)
technologies     → uv_premium_factor
```

---

## Phase 1A: Motor SVG (svgengine)

### 1A.1: Modelos SVG
**Archivos:**
- `internal/models/svg_analysis.go`
- `internal/models/svg_element.go`

**Contenido:**
```go
type SVGAnalysis struct {
    ID            uint
    UserID        uint
    Filename      string
    OriginalSVG   string      // SVG completo
    Width, Height float64     // Dimensiones
    TotalArea     float64     // mm²
    CutLength     float64     // mm (rojo)
    EngraveArea   float64     // mm² (negro)
    VectorLength  float64     // mm (azul)
    Elements      []SVGElement
    Status        string      // pending, analyzed, error
}

type SVGElement struct {
    Type      string  // path, rect, circle, line, polygon
    Color     string  // hex
    Category  string  // cut, engrave, vector
    Area      float64
    Perimeter float64
    PathData  string  // d attribute original
}
```

### 1A.2: Migración svg_analyses
**Archivo:** `migrations/009_svg_analyses.sql`

Tabla para almacenar análisis SVG con geometría calculada.

### 1A.3: SVG Parser
**Archivo:** `internal/services/svgengine/parser.go`

- Parsear XML con `encoding/xml`
- Extraer elementos: `<path>`, `<rect>`, `<circle>`, `<line>`, `<polygon>`, `<ellipse>`
- Extraer atributos: `d`, `fill`, `stroke`, `width`, `height`, `x`, `y`, `r`, `cx`, `cy`
- Manejar viewBox y transformaciones básicas

### 1A.4: Color Classifier
**Archivo:** `internal/services/svgengine/classifier.go`

Convenciones de color (del roadmap):
```
Rojo #FF0000 (stroke) → CORTE
Azul #0000FF (stroke) → VECTOR (grabado línea)
Negro #000000 (fill)  → RASTER (grabado área)
```

Tolerancia: ±10% en cada canal RGB.

### 1A.5: Geometry Calculator
**Archivo:** `internal/services/svgengine/geometry.go`

- **Rectángulos:** área = w×h, perímetro = 2(w+h)
- **Círculos:** área = πr², perímetro = 2πr
- **Elipses:** área = πab, perímetro ≈ π(3(a+b) - √((3a+b)(a+3b)))
- **Paths/Bézier:** Algoritmo de subdivisión adaptativa
  - Subdividir hasta segmentos < 0.1mm
  - Sumar longitudes de segmentos
  - Área por Shoelace formula

### 1A.6: SVG Analyzer Service
**Archivo:** `internal/services/svgengine/analyzer.go`

Orquestador que:
1. Recibe SVG string
2. Parsea XML
3. Clasifica elementos por color
4. Calcula geometría de cada elemento
5. Agrega totales (cut_length, engrave_area, vector_length)
6. Retorna SVGAnalysis completo

---

## Phase 1B: Motor Pricing

### 1B.1: Modelos Quote
**Archivos:**
- `internal/models/quote.go`
- `internal/models/quote_item.go`

```go
type Quote struct {
    ID              uint
    UserID          uint
    SVGAnalysisID   uint
    TechnologyID    uint
    MaterialID      uint
    EngraveTypeID   uint
    Quantity        int

    // Calculados
    TimeEngrave     float64  // minutos
    TimeCut         float64  // minutos
    CostBase        float64  // antes de margen
    CostFinal       float64  // con margen y descuentos
    DiscountApplied float64  // porcentaje

    Status          string   // draft, auto_approved, needs_review, rejected
    ReviewNotes     string
    ExpiresAt       time.Time
}
```

### 1B.2: Migración quotes
**Archivo:** `migrations/010_quotes.sql`

Tabla quotes con FK a svg_analyses, users, technologies, materials, engrave_types.

### 1B.3: Config Loader
**Archivo:** `internal/services/pricing/config_loader.go`

Carga de DB (NO hardcode):
```go
type PricingConfig struct {
    TechRates       map[uint]TechRate       // tech_id → rates
    Materials       map[uint]Material       // material_id → factor
    EngraveTypes    map[uint]EngraveType    // type_id → factor, speed
    VolumeDiscounts []VolumeDiscount        // rangos ordenados
}

func LoadPricingConfig(db *gorm.DB) (*PricingConfig, error)
```

Cache en memoria con TTL de 5 minutos.

### 1B.4: Time Estimator
**Archivo:** `internal/services/pricing/time_estimator.go`

```go
// Tiempo grabado = área / (velocidad_base × speed_multiplier)
// Tiempo corte = longitud / velocidad_corte
// Velocidad base: 100 mm²/min grabado, 20 mm/min corte (configurable)

func EstimateTime(analysis SVGAnalysis, engraveType EngraveType) (engraveMins, cutMins float64)
```

### 1B.5: Pricing Calculator
**Archivo:** `internal/services/pricing/calculator.go`

**Modelo Híbrido (del roadmap):**
```
Costo_Base = (tiempo_grabado × rate_engrave) + (tiempo_corte × rate_cut)
Costo_Final = Costo_Base × (1 + margin) × factor_material × factor_grabado × (1 + uv_premium)
```

**Descuento por Volumen:**
```
if cantidad >= 100: descuento = 20%
if cantidad >= 50:  descuento = 15%
if cantidad >= 25:  descuento = 10%
if cantidad >= 10:  descuento = 5%
```

**Clasificación automática:**
```
factor_complejidad = (cut_length + vector_length) / sqrt(area)

if factor ≤ 1.5:  AUTO_APPROVED
if factor ≤ 2.5:  NEEDS_REVIEW
else:             REJECTED (con mensaje)
```

---

## Phase 1C: Quote API

### 1C.1: QuotaMiddleware
**Archivo:** `internal/middleware/quota.go`

```go
func QuotaMiddleware(next http.Handler) http.Handler {
    // 1. Extraer user_id del JWT
    // 2. Consultar quote_quota del usuario
    // 3. Si quota == -1: unlimited (admin)
    // 4. Si quota == 0: rechazar con 403
    // 5. Contar cotizaciones del mes actual
    // 6. Si usadas >= quota: rechazar con 429
    // 7. Continuar
}
```

### 1C.2: Quote Handler
**Archivo:** `internal/handlers/quote/handler.go`

Endpoints:
```
POST /api/v1/quotes/analyze    → Subir SVG, analizar, retornar SVGAnalysis
POST /api/v1/quotes/calculate  → Recibir opciones, calcular precio
GET  /api/v1/quotes/:id        → Ver cotización específica
GET  /api/v1/quotes/my         → Listar mis cotizaciones
```

### 1C.3: Quote Repository
**Archivo:** `internal/repositories/quote_repository.go`

CRUD estándar + queries específicas:
- `GetByUserID(userID uint, limit, offset int)`
- `CountByUserThisMonth(userID uint) int`
- `GetPendingReview() []Quote`

### 1C.4: Integración y Tests
- Agregar rutas al router principal
- Middleware de quota en rutas de quotes
- Tests unitarios para geometry calculator
- Tests de integración para flujo completo

---

## Orden de Ejecución

```
┌─────────────────────────────────────────────────────────────┐
│  1A.1 Modelos SVG  ──►  1A.2 Migración  ──►  1A.3 Parser   │
│                                              │              │
│                                              ▼              │
│                                         1A.4 Classifier     │
│                                              │              │
│                                              ▼              │
│                                         1A.5 Geometry       │
│                                              │              │
│                                              ▼              │
│                                         1A.6 Analyzer       │
└──────────────────────────────────────────────┼──────────────┘
                                               │
┌──────────────────────────────────────────────▼──────────────┐
│  1B.1 Modelos Quote  ──►  1B.2 Migración  ──►  1B.3 Loader │
│                                                │             │
│                                                ▼             │
│                                           1B.4 Time Est     │
│                                                │             │
│                                                ▼             │
│                                           1B.5 Calculator   │
└────────────────────────────────────────────────┼────────────┘
                                                 │
┌────────────────────────────────────────────────▼────────────┐
│  1C.1 QuotaMiddleware  ──►  1C.2 Handler  ──►  1C.3 Repo   │
│                                                │             │
│                                                ▼             │
│                                           1C.4 Tests        │
└─────────────────────────────────────────────────────────────┘
```

---

## Principios

1. **NO HARDCODE**: Todo parámetro de pricing viene de DB
2. **Modular**: Cada servicio es independiente y testeable
3. **Failsafe**: Si DB falla, usar valores por defecto seguros (rechazar)
4. **Logging**: Registrar cada cálculo para auditoría
5. **Cache**: Config loader con cache para evitar queries repetidas

---

## Criterios de Completitud

- [ ] SVG de prueba analizado correctamente
- [ ] Cotización calculada con valores de DB
- [ ] QuotaMiddleware bloqueando usuarios sin cuota
- [ ] Endpoint /quotes/my retornando historial
- [ ] Tests pasando para geometry calculator
