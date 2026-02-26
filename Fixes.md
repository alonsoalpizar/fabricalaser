# Instrucción: Corrección del Motor de Cotización FabricaLaser

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD del plan. Si la sesión se compacta, si perdés contexto, si hay cualquier duda sobre qué tarea va, qué archivo es de quién, o qué gate sigue: **RELEER este archivo completo antes de continuar.** No asumir de memoria. El archivo siempre tiene la versión correcta.

## Contexto

Se realizó una auditoría del motor de cotización (`internal/services/pricing/`) y se identificaron 5 hallazgos que requieren corrección. Este documento es la instrucción completa para ejecutar las correcciones.

**Proyecto:** FabricaLaser (Go + Chi + GORM + PostgreSQL)
**Archivos involucrados:**

```
internal/services/pricing/calculator.go      - Motor principal de precios
internal/services/pricing/time_estimator.go  - Estimador de tiempos
internal/services/pricing/config_loader.go   - Carga de config desde BD
internal/handlers/quote/handler.go           - Handler HTTP
internal/models/quote.go                     - Modelo Quote
internal/models/svg_analysis.go              - Modelo SVGAnalysis
seeds/003_tech_material_speeds.sql           - Velocidades por combinación
```

**Regla general:** Cero hardcode. Todo valor configurable viene de BD (system_config, tech_rates, materials, etc.). Si necesitás un fallback, documentalo con comentario explicando por qué.

---

## Plan de Ejecución (5 Tareas)

Ejecutar en orden. Cada tarea es independiente pero se construye sobre la anterior. Usar subagentes donde se indica, con scope acotado y validación entre pasos.

---

### TAREA 1: Separar velocidades raster/vector en tech_material_speeds
**Severidad:** Alta
**Archivos:** migración SQL nueva, `config_loader.go`, `time_estimator.go`, seed `003`

**Problema:** La tabla `tech_material_speeds` tiene una sola columna `engrave_speed_mm_min` que se usa tanto para raster (mm²/min) como para vector (mm/min). Son unidades diferentes y producen tiempos incorrectos en diseños mixtos.

**Solución:**

1. **Crear migración SQL** (nueva, NO modificar seeds existentes):
   ```sql
   -- Renombrar columna existente y agregar nueva
   ALTER TABLE tech_material_speeds 
     RENAME COLUMN engrave_speed_mm_min TO raster_speed_mm2_min;
   
   ALTER TABLE tech_material_speeds 
     ADD COLUMN vector_speed_mm_min FLOAT NULL;
   
   -- Comentarios para claridad
   COMMENT ON COLUMN tech_material_speeds.raster_speed_mm2_min IS 'Velocidad grabado raster en mm²/min';
   COMMENT ON COLUMN tech_material_speeds.vector_speed_mm_min IS 'Velocidad grabado vector en mm/min (NULL = usar base speed)';
   ```

2. **Actualizar modelo Go** `TechMaterialSpeed`:
   - Renombrar campo `EngraveSpeedMmMin` → `RasterSpeedMm2Min`
   - Agregar campo `VectorSpeedMmMin *float64`
   - Actualizar tags gorm correspondientes

3. **Actualizar `config_loader.go`**:
   - `TechMaterialSpeedResult` debe tener dos campos separados:
     ```go
     type TechMaterialSpeedResult struct {
         CutSpeedMmMin      *float64
         RasterSpeedMm2Min  *float64  // era EngraveSpeedMmMin
         VectorSpeedMmMin   *float64  // NUEVO
         Found              bool
     }
     ```
   - `GetMaterialSpeed()` mapea ambos campos

4. **Actualizar `time_estimator.go`**:
   - Sección raster usa `specificSpeed.RasterSpeedMm2Min`
   - Sección vector usa `specificSpeed.VectorSpeedMmMin` (con fallback independiente a `baseEngraveLineSpeed`)
   - `SpeedInfo` struct actualizar con ambos campos específicos

5. **Actualizar seed 003** con datos coherentes:
   ```sql
   -- Ejemplo CO2 + MDF 3mm
   raster_speed_mm2_min = 600.0,  -- mm²/min para área
   vector_speed_mm_min = 120.0,   -- mm/min para líneas (puede ser NULL si no aplica)
   ```

**Validación:** Compilar. Verificar que `GetSpeedInfo()` retorna ambas velocidades correctamente. No debe haber ninguna referencia a `EngraveSpeedMmMin` en el código después de esta tarea.

---

### TAREA 2: Validar compatibilidad tech×material en Handler
**Severidad:** Media
**Archivos:** `handler.go`, `config_loader.go`

**Problema:** Si un usuario pide CO2 + Metal (incompatible según `tech_material_speeds.is_compatible=false`), el sistema no valida y calcula con velocidades fallback, dando un precio para algo que no se puede hacer.

**Solución:**

1. **Agregar método en `config_loader.go`**:
   ```go
   // IsCompatible checks if a technology can work with a material
   // Returns: compatible bool, reason string
   func (c *PricingConfig) IsCompatible(techID, materialID uint, thickness float64) (bool, string) {
       // Buscar en TechMaterialSpeeds si existe registro con is_compatible=false
       for _, s := range c.TechMaterialSpeeds {
           if s.TechnologyID == techID && s.MaterialID == materialID {
               if s.Thickness == thickness || s.Thickness == 0 {
                   if !s.IsCompatible {
                       return false, s.Notes // ej: "CO2 no trabaja con metal - usar FIBRA o MOPA"
                   }
                   return true, ""
               }
           }
       }
       // Si no hay registro, asumimos compatible (usa velocidades base)
       return true, ""
   }
   ```

   **NOTA:** Cargar también los registros con `is_compatible=false` en el refresh(). Actualmente el query filtra `WHERE is_active = ? AND is_compatible = ?`, cambiar a solo `WHERE is_active = ?` para poder hacer el check de incompatibilidad.

2. **Validar en `handler.go` CalculatePrice**, ANTES de llamar a `calculator.Calculate()`:
   ```go
   // Validate tech×material compatibility
   config, err := h.configLoader.Load()
   if err != nil {
       respondError(w, http.StatusInternalServerError, "CONFIG_ERROR", "Error loading configuration")
       return
   }
   
   compatible, reason := config.IsCompatible(req.TechnologyID, req.MaterialID, req.Thickness)
   if !compatible {
       respondError(w, http.StatusBadRequest, "INCOMPATIBLE", reason)
       return
   }
   ```

**Validación:** Verificar que el query en refresh() ahora carga TODOS los registros activos (con y sin compatibilidad). Verificar que CO2+Metal retorna error 400 con mensaje descriptivo.

---

### TAREA 3: PriceFinal = MAX(Hybrid, Value) — Protección de piso
**Severidad:** Media
**Archivos:** `calculator.go`

**Problema:** `PriceFinal` siempre se asigna a `PriceHybridTotal`. El modelo Value-Based se calcula pero nunca se usa. Decisión: usar el MAYOR de los dos como protección de piso de precio.

**Solución:**

1. **En `calculator.go`, después de calcular ambos modelos**, reemplazar la asignación directa:

   ```go
   // ANTES (línea actual):
   // PriceFinal: result.PriceHybridTotal, // Default to hybrid
   
   // DESPUÉS:
   // PriceFinal = MAX(Hybrid, Value) — protección de piso
   // Garantiza que nunca cobramos por debajo del valor de mercado
   // ni por debajo del costo real de producción
   ```

2. **En `ToQuoteModel()`**, cambiar la asignación de PriceFinal:
   ```go
   PriceFinal: math.Max(result.PriceHybridTotal, result.PriceValueTotal),
   ```

3. **En `PriceResult`**, agregar campo para indicar qué modelo ganó:
   ```go
   PriceModel string // "hybrid" o "value" — indica cuál modelo determinó el precio final
   ```

4. **En el cálculo**, asignar el modelo ganador:
   ```go
   if result.PriceHybridTotal >= result.PriceValueTotal {
       result.PriceModel = "hybrid"
   } else {
       result.PriceModel = "value"
   }
   ```

5. **En `quote.go` modelo**, agregar campo:
   ```go
   PriceModel string `gorm:"type:varchar(10);default:'hybrid'" json:"price_model"` // which model won
   ```

6. **En `ToDetailedJSON()`**, incluir `price_model` dentro del bloque `"pricing"`.

**Validación:** Para un diseño grande pero simple (mucha área, pocas líneas), el Value debería ganar. Para un diseño pequeño pero complejo (poca área, muchas líneas finas), el Hybrid debería ganar. Verificar ambos escenarios mentalmente con los datos del seed.

---

### TAREA 4: Simulación FactorMaterial en Hybrid
**Severidad:** Media (análisis)
**Archivos:** `calculator.go` (solo agregar logging/simulación, NO cambiar la fórmula aún)

**Problema:** El FactorMaterial (1.0-1.8) solo afecta la velocidad (tiempo) en el modelo Hybrid, pero NO se aplica como multiplicador al precio. En el modelo Value SÍ se aplica. Necesitamos datos para decidir si debe aplicarse también en Hybrid.

**Solución:**

1. **En `PriceResult`**, agregar campos de simulación:
   ```go
   // Simulation fields — for analysis, not used in final price
   SimHybridWithMaterialFactor float64 // What hybrid would be WITH material factor
   SimDifferencePct            float64 // Percentage difference
   ```

2. **En `calculator.go`**, después de calcular `PriceHybridUnit`, agregar simulación:
   ```go
   // === SIMULACIÓN: ¿Qué pasaría si aplicamos FactorMaterial al Hybrid? ===
   simHybridUnit := perUnitCostBase
   simHybridUnit *= (1 + result.FactorMargin)
   simHybridUnit *= result.FactorEngrave
   simHybridUnit *= (1 + result.FactorUVPremium)
   simHybridUnit *= result.FactorMaterial  // <-- ESTE ES EL CAMBIO SIMULADO
   
   simHybridTotal := math.Round(simHybridUnit*100) / 100 * float64(quantity)
   simHybridTotal *= (1 - result.DiscountVolumePct)
   simHybridTotal += result.CostSetup
   
   result.SimHybridWithMaterialFactor = math.Round(simHybridTotal*100) / 100
   if result.PriceHybridTotal > 0 {
       result.SimDifferencePct = (result.SimHybridWithMaterialFactor - result.PriceHybridTotal) / result.PriceHybridTotal * 100
   }
   ```

3. **En `ToQuoteModel()`** y `ToDetailedJSON()`**, exponer estos campos para que podamos ver la diferencia en las respuestas API y tomar la decisión con datos reales.

**IMPORTANTE:** Esta tarea NO cambia la fórmula de precios. Solo agrega campos informativos. La decisión de si aplicar o no FactorMaterial al Hybrid se toma DESPUÉS de ver los datos en producción.

**Validación:** Para MDF (factor 1.0), SimDifferencePct debe ser 0%. Para Metal (factor 1.8), debe ser ~80% más caro. Verificar que PriceFinal NO se ve afectado por la simulación.

---

### TAREA 5: Cleanup — ida-y-vuelta de quantity + fallback de margin
**Severidad:** Baja
**Archivos:** `calculator.go`, `config_loader.go`

**Problema menor 1:** TimeEstimator multiplica tiempos por quantity, luego Calculator divide CostBase entre quantity para obtener el unitario. Es un ida-y-vuelta innecesario.

**Solución 1:** En `calculator.go`, usar `timeEstimator.EstimatePerUnit()` (ya existe) para obtener tiempos por unidad, y manejar quantity solo en el calculator:

```go
// ANTES:
timeEst := timeEstimator.Estimate(analysis, techID, materialID, engraveTypeID, thickness, quantity)
// ... luego divide CostBase / quantity

// DESPUÉS:
timeEstPerUnit := timeEstimator.EstimatePerUnit(analysis, techID, materialID, engraveTypeID, thickness)
// Costos son por unidad directamente, sin ida-y-vuelta
result.CostEngrave = timeEstPerUnit.EngraveMins * costPerMinEngrave
result.CostCut = timeEstPerUnit.CutMins * costPerMinCut
// ...
perUnitMachineCost = result.CostEngrave + result.CostCut  // Ya es por unidad
```

Ajustar los campos de tiempo en el resultado para reflejar totales (multiplicar por quantity al final para el quote model).

**Problema menor 2:** `GetMarginPercent()` tiene fallback 0.40 hardcodeado. Mover a system_config.

**Solución 2:** Agregar en seed 002:
```sql
INSERT INTO system_config (config_key, config_value, value_type, category, description) VALUES
('default_margin_percent', '0.40', 'number', 'pricing', 'Margen por defecto si no hay tech_rate configurado');
```

Y en `config_loader.go`:
```go
func (c *PricingConfig) GetMarginPercent(techID uint) float64 {
    if rate := c.TechRates[techID]; rate != nil {
        return rate.MarginPercent
    }
    return c.GetSystemConfigFloat("default_margin_percent", 0.40)
}
```

**Validación:** Los precios finales NO deben cambiar (misma matemática, diferente flujo). Verificar con un caso de prueba que el resultado sea idéntico antes y después de la refactorización.

---

### TAREA 6: Bugs de Persistencia y Respuesta API (del Reporte de Validación)
**Severidad:** Alta (BUG-1), Media (BUG-2), Media (fallback warning)
**Archivos:** `internal/models/quote.go`, `calculator.go`, `handler.go`

**Origen:** VALIDATION_REPORT.md — Casos de prueba reales detectaron estos problemas.

#### BUG-1 (CRÍTICO): MaterialIncluded no persiste `false` en BD

**Problema:** GORM omite campos booleanos con valor `false` en INSERT porque Go trata `false` como zero value. La BD tiene `DEFAULT true`, así que `material_included` SIEMPRE queda `true` aunque el usuario envíe `false`.

**Solución en `internal/models/quote.go`:**
```go
// ANTES:
MaterialIncluded bool `gorm:"default:true" json:"material_included"`

// DESPUÉS:
MaterialIncluded *bool `gorm:"default:true" json:"material_included"`
```

**Ajustar en `calculator.go` → `ToQuoteModel()`:**
```go
// Crear el puntero correctamente
materialIncl := result.MaterialIncluded
// ...
MaterialIncluded: &materialIncl,
```

**Ajustar en cualquier lugar que lea `quote.MaterialIncluded`** — ahora es puntero, hay que desreferenciar:
```go
// ANTES:
if quote.MaterialIncluded { ... }

// DESPUÉS:
if quote.MaterialIncluded != nil && *quote.MaterialIncluded { ... }
```

**Buscar TODOS los usos:** `grep -rn "MaterialIncluded" internal/` y actualizar cada referencia.

#### BUG-2 (MEDIO): material_included falta a nivel root en ToDetailedJSON

**Problema:** El frontend lee `q.material_included` a nivel root, pero `ToDetailedJSON()` solo lo pone dentro de `material_cost.included`. El campo root devuelve `null`.

**Solución en `quote.go` → `ToDetailedJSON()`:**
```go
// Agregar a nivel root del resultado (junto a quantity, thickness, etc.):
"material_included": q.MaterialIncluded,
```

Esto mantiene backward compatibility: `material_cost.included` sigue existiendo, y ahora también existe a nivel root.

#### NUEVO: Advertencia cuando se usa velocidad fallback

**Problema:** Cuando no existe velocidad específica en `tech_material_speeds` para una combinación, el sistema usa `base_cut_speed = 20 mm/min` como fallback. Esto genera precios hasta 100x mayores (Caso 6 del reporte: ₡4,230 vs ₡32). El usuario no tiene forma de saber que el precio es impreciso.

**Solución:**

1. En `PriceResult`, agregar:
```go
UsedFallbackSpeeds bool   // true si se usaron velocidades base en vez de matriz
FallbackWarning    string // mensaje descriptivo si aplica
```

2. En `time_estimator.go`, el método `Estimate` ya sabe si usó fallback (`specificSpeed.Found`). Propagarlo al resultado. Agregar campo en `TimeEstimate`:
```go
UsedFallback bool
```

3. En `calculator.go`, después de obtener `timeEst`:
```go
if timeEst.UsedFallback {
    result.UsedFallbackSpeeds = true
    result.FallbackWarning = "Precio estimado con velocidades base. No hay calibración específica para esta combinación tech/material/grosor. El precio real puede variar significativamente."
}
```

4. En `quote.go` modelo, agregar campos:
```go
UsedFallbackSpeeds bool    `gorm:"default:false" json:"used_fallback_speeds"`
FallbackWarning    *string `gorm:"type:text" json:"fallback_warning,omitempty"`
```

5. En `ToDetailedJSON()`, incluir ambos campos.

6. En `ToQuoteModel()`, mapear los campos desde PriceResult.

**Validación:** Caso CO2+MDF 3mm (tiene matriz) → `used_fallback_speeds: false`. Caso UV+Vidrio corte (sin matriz) → `used_fallback_speeds: true` con warning.

---

## Orden de Ejecución y Subagentes (ACTUALIZADO)

```
FASE 1 (paralelo):
  Tarea 1 → Subagente 1: separar velocidades raster/vector
  Tarea 2 → Subagente 2: validar compatibilidad tech×material

FASE 2 (secuencial, Subagente 1):
  Tarea 3 → PriceFinal = MAX(Hybrid, Value)
  Tarea 4 → Simulación FactorMaterial

FASE 3 (Subagente 2):
  Tarea 5 → Cleanup quantity + margin fallback
  Tarea 6 → Bugs persistencia + fallback warning
```

**Coordinación entre subagentes:**
- Tarea 1 DEBE completarse antes de Tarea 3 y 4 (cambia structs que usan)
- Tarea 2 es independiente, puede ejecutarse en paralelo con Tarea 1
- Tarea 5 va al final porque refactoriza flujo que Tarea 3 y 4 modifican

## CONTRATO DE COORDINACIÓN ENTRE SUBAGENTES

### Propiedad de Archivos (File Ownership)

Cada archivo tiene UN solo dueño en cada momento. Si un subagente no es dueño del archivo, NO lo toca.

```
FASE 1 (Tareas 1 y 2 en paralelo):

  Subagente 1 ("pricing-speeds") es DUEÑO de:
    - internal/models/tech_material_speed.go  (modelo TechMaterialSpeed)
    - internal/services/pricing/time_estimator.go
    - seeds/003_tech_material_speeds.sql (lectura, NO modificar)
    - migrations/XXX_split_engrave_speeds.sql (CREAR NUEVO)
    - seeds/005_update_split_speeds.sql (CREAR NUEVO)

  Subagente 2 ("pricing-compat") es DUEÑO de:
    - internal/handlers/quote/handler.go

  COMPARTIDO (requiere coordinación):
    - internal/services/pricing/config_loader.go
      → Subagente 1 modifica: TechMaterialSpeedResult struct + GetMaterialSpeed()
      → Subagente 2 modifica: agrega IsCompatible() + cambia query en refresh()
      → REGLA: Subagente 1 va PRIMERO en config_loader.go, Subagente 2 espera.
```

```
FASE 2 (Tareas 3 y 4, secuenciales, solo Subagente 1):

  Subagente 1 es DUEÑO de:
    - internal/services/pricing/calculator.go
    - internal/models/quote.go

  Subagente 2: INACTIVO en esta fase.
```

```
FASE 3 (Tareas 5 y 6, solo Subagente 2):

  Subagente 2 es DUEÑO de:
    - internal/services/pricing/calculator.go (cleanup de quantity + fallback warning)
    - internal/services/pricing/config_loader.go (mover fallback margin)
    - internal/services/pricing/time_estimator.go (agregar UsedFallback a TimeEstimate)
    - internal/models/quote.go (BUG-1: *bool, BUG-2: ToDetailedJSON, campos fallback)
    - seeds/002_system_config.sql (lectura, NO modificar)
    - seeds/006_default_margin.sql (CREAR NUEVO)

  Subagente 1: INACTIVO en esta fase.
```

### Puntos de Sincronización (Sync Gates)

```
GATE 0: Antes de empezar
  → Ambos subagentes leen PRICING_FORMULA_ACTUAL.md como referencia
  → Ambos verifican que el proyecto compila: `go build ./...`

GATE 1: Después de FASE 1
  → Subagente 1 TERMINA cambios en config_loader.go PRIMERO
  → Subagente 1 confirma: "config_loader.go listo, TechMaterialSpeedResult actualizado"
  → SOLO ENTONCES Subagente 2 toca config_loader.go para agregar IsCompatible()
  → Ambos verifican compilación: `go build ./...`
  → Si no compila, PARAR y resolver conflicto antes de continuar

GATE 2: Después de FASE 2
  → Subagente 1 confirma: "calculator.go y quote.go actualizados con PriceFinal=MAX y simulación"
  → Verificar compilación: `go build ./...`
  → Subagente 1 entrega ownership de calculator.go

GATE 3: Después de FASE 3
  → Subagente 2 confirma: "cleanup completo"
  → Compilación final: `go build ./...`
  → Verificar que NO hay referencias a EngraveSpeedMmMin en todo el proyecto:
    `grep -r "EngraveSpeedMmMin" internal/`  → debe dar 0 resultados
```

### Reglas Anti-Descoordinación

1. **UN subagente por archivo a la vez.** Si necesitás tocar un archivo que no es tuyo, ESPERÁ al gate de sincronización.
2. **No asumir el estado de un archivo compartido.** Antes de editarlo, LEERLO primero para ver los cambios del otro subagente.
3. **Si hay conflicto de merge o compilación falla después de un gate, PARAR.** No intentar resolverlo solo — el otro subagente puede tener contexto que falta.
4. **Cada subagente documenta qué cambió en cada archivo** al pasar un gate. Formato:
   ```
   [GATE N] Subagente X completó:
   - archivo1.go: agregó struct X, modificó función Y
   - archivo2.go: nuevo método Z
   - Compila: ✅
   ```

### Reglas Generales (se mantienen)

5. **No modificar seeds existentes (001-004).** Crear archivos nuevos para migraciones y seeds adicionales.
6. **Cada tarea compila al finalizar.** Si no compila, no avanzar.
7. **Cero hardcode.** Todo valor nuevo va a system_config o a la tabla correspondiente.
8. **Los campos nuevos en modelos deben tener valores default** para no romper registros existentes en BD.
9. **Mantener backward compatibility** en la API: los campos existentes en ToDetailedJSON() no cambian de nombre ni se eliminan, solo se agregan nuevos.
10. **Documentar cada cambio** con comentario inline explicando el POR QUÉ, no solo el QUÉ.

## Documento de Referencia

El archivo `PRICING_FORMULA_ACTUAL.md` contiene la fórmula completa documentada con todos los valores actuales. Usarlo como referencia para validar que los cambios no rompen la lógica existente (excepto donde explícitamente se indica que debe cambiar, como PriceFinal en Tarea 3).