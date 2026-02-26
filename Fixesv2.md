# fixes-v2.md — Corrección de Velocidad Raster con Spot Size

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD del plan. Si la sesión se compacta o perdés contexto: **RELEER este archivo completo antes de continuar.** No asumir de memoria.

## Contexto

`fixes.md` (ya ejecutado) separó `engrave_speed_mm_min` en dos columnas: `raster_speed_mm2_min` y `vector_speed_mm_min` en la tabla `tech_material_speeds`. Esta solución requiere llenar 74 valores manualmente y no escala bien.

**Solución superior:** Revertir a UNA sola columna de velocidad de cabezal (mm/min) y agregar `spot_size_mm` a la tabla `technologies`. El sistema convierte automáticamente:
- Vector = velocidad cabezal directa (mm/min)
- Raster = velocidad cabezal × spot_size (mm²/min)

**Por qué:** La velocidad de grabado en la máquina láser es siempre la velocidad lineal del cabezal (mm/min). Para raster, el cabezal barre líneas horizontales cubriendo un ancho igual al spot size por pasada. La velocidad de cobertura de área es: `velocidad_cabezal × spot_size_mm`.

**Impacto:** Con spot_size=0.1mm (CO2), los tiempos de raster serán ~10x mayores que antes. Esto es CORRECTO — el sistema anterior subcobraba raster significativamente.

---

## Archivos a Modificar

```
internal/models/technology.go              — Agregar campo SpotSizeMM
internal/models/tech_material_speed.go     — Revertir a engrave_speed_mm_min (1 columna)
internal/services/pricing/config_loader.go — Revertir TechMaterialSpeedResult + exponer spot_size
internal/services/pricing/time_estimator.go — Conversión automática raster = speed × spot_size
migrations/                                — Migración para revertir columnas + agregar spot_size
seeds/                                     — Seed con spot_size por tecnología
```

---

## Tarea Única: Spot Size Automático

**NO usar subagentes. Una sola tarea, un solo flujo, secuencial.**

### Paso 1: Migración SQL

Crear UNA migración nueva que haga todo en orden:

```sql
-- 1. Revertir la separación de columnas en tech_material_speeds
-- Si fixes.md creó raster_speed_mm2_min y vector_speed_mm_min:
--   - Copiar datos de raster_speed_mm2_min de vuelta a engrave_speed_mm_min
--     (o recrear la columna si fue eliminada)
--   - Eliminar raster_speed_mm2_min y vector_speed_mm_min

-- 2. Agregar spot_size_mm a technologies
ALTER TABLE technologies
  ADD COLUMN IF NOT EXISTS spot_size_mm FLOAT NOT NULL DEFAULT 0.1;

COMMENT ON COLUMN technologies.spot_size_mm IS 'Diámetro del punto láser en mm. Usado para convertir velocidad cabezal (mm/min) a velocidad raster (mm²/min). Fórmula: raster_speed = engrave_speed × spot_size_mm';
```

**IMPORTANTE:** Revisar primero qué hizo exactamente `fixes.md` en la tabla `tech_material_speeds` antes de escribir la migración de reversión. Usar `\d tech_material_speeds` o revisar las migraciones creadas.

### Paso 2: Seed de Spot Sizes

```sql
-- Spot sizes por tecnología (valores reales del taller)
UPDATE technologies SET spot_size_mm = 0.10 WHERE code = 'CO2';
UPDATE technologies SET spot_size_mm = 0.03 WHERE code = 'FIBRA';
UPDATE technologies SET spot_size_mm = 0.04 WHERE code = 'MOPA';
UPDATE technologies SET spot_size_mm = 0.02 WHERE code = 'UV';
```

### Paso 3: Modelo Go — Technology

En `internal/models/technology.go`, agregar campo:

```go
SpotSizeMM float64 `gorm:"type:float;not null;default:0.1" json:"spot_size_mm"`
```

### Paso 4: Modelo Go — TechMaterialSpeed

Revertir a UNA sola columna de velocidad de grabado. Si `fixes.md` renombró o separó columnas, restaurar a:

```go
EngraveSpeedMmMin *float64 `gorm:"column:engrave_speed_mm_min" json:"engrave_speed_mm_min"`
```

Eliminar cualquier campo `RasterSpeedMm2Min` o `VectorSpeedMmMin` que haya agregado `fixes.md`.

### Paso 5: Config Loader

En `config_loader.go`:

**5a.** Revertir `TechMaterialSpeedResult` a UNA velocidad de grabado:

```go
type TechMaterialSpeedResult struct {
    CutSpeedMmMin     *float64
    EngraveSpeedMmMin *float64  // Velocidad cabezal (mm/min), NO mm²/min
    Found             bool
}
```

Eliminar `RasterSpeedMm2Min` y `VectorSpeedMmMin` si existen.

**5b.** Agregar método para obtener spot size:

```go
func (c *PricingConfig) GetSpotSize(techID uint) float64 {
    if tech := c.Technologies[techID]; tech != nil && tech.SpotSizeMM > 0 {
        return tech.SpotSizeMM
    }
    return 0.1 // Default CO2 spot size
}
```

### Paso 6: Time Estimator — La conversión clave

En `time_estimator.go`, método `Estimate`:

**6a.** Obtener spot size al inicio:

```go
spotSize := e.config.GetSpotSize(techID)
```

**6b.** Sección RASTER — convertir velocidad cabezal a velocidad de área:

```go
if analysis.RasterAreaMM2 > 0 {
    var engraveSpeedMmMin float64
    if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
        engraveSpeedMmMin = *specificSpeed.EngraveSpeedMmMin * speedMult
    } else {
        engraveSpeedMmMin = baseEngraveLineSpeed * speedMult / materialFactor
    }
    // Convertir velocidad cabezal (mm/min) a velocidad área (mm²/min)
    // Cada pasada del cabezal cubre un ancho = spot_size_mm
    effectiveRasterSpeed := engraveSpeedMmMin * spotSize
    rasterTime := analysis.RasterAreaMM2 / effectiveRasterSpeed
    estimate.EngraveMins += rasterTime
}
```

**6c.** Sección VECTOR — usar velocidad cabezal directa:

```go
if analysis.VectorLengthMM > 0 {
    var effectiveVectorSpeed float64
    if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
        effectiveVectorSpeed = *specificSpeed.EngraveSpeedMmMin * speedMult
    } else {
        effectiveVectorSpeed = baseEngraveLineSpeed * speedMult / materialFactor
    }
    // Vector usa velocidad cabezal directamente (mm/min)
    vectorTime := analysis.VectorLengthMM / effectiveVectorSpeed
    estimate.EngraveMins += vectorTime
}
```

**6d.** Actualizar `SpeedInfo` struct y `GetSpeedInfo` para incluir spot_size y las velocidades derivadas:

```go
type SpeedInfo struct {
    // ... campos existentes ...
    SpotSizeMM            float64  // Del technology
    EffectiveRasterSpeed  float64  // engraveSpeed × spotSize (mm²/min)
    EffectiveVectorSpeed  float64  // engraveSpeed directa (mm/min)
}
```

### Paso 7: Verificación

```bash
# 1. Compilar
go build ./...

# 2. Verificar que NO existen columnas separadas
grep -rn "RasterSpeedMm2Min\|VectorSpeedMmMin\|raster_speed_mm2_min\|vector_speed_mm_min" internal/
# Debe dar 0 resultados

# 3. Verificar que spot_size se usa en time_estimator
grep -n "spotSize\|SpotSize\|spot_size" internal/services/pricing/time_estimator.go
# Debe mostrar la conversión

# 4. Verificar que EngraveSpeedMmMin es una sola columna
grep -n "EngraveSpeedMmMin" internal/models/tech_material_speed.go
# Debe ser exactamente 1 campo
```

---

## Lo que NO se toca

Las Tareas 2-6 de `fixes.md` quedan intactas:
- ✅ Tarea 2: Validación compatibilidad tech×material en handler
- ✅ Tarea 3: PriceFinal = MAX(Hybrid, Value)
- ✅ Tarea 4: Simulación FactorMaterial
- ✅ Tarea 5: Cleanup quantity + margin fallback
- ✅ Tarea 6: Bugs persistencia (*bool) + fallback warning

---

## Impacto en Precios

Con spot_size = 0.1mm (CO2), los tiempos de raster aumentan ~10x.
Ejemplo CO2 + MDF 3mm, raster 900mm²:
- ANTES: 900 / 4000 = 0.225 min (13 seg) ← INCORRECTO
- DESPUÉS: 900 / (4000 × 0.1) = 2.25 min (2:15) ← CORRECTO

Esto hará que los precios de trabajos con mucho raster (fotograbado, rellenos) suban significativamente. Es lo correcto — antes estaban subvalorados.