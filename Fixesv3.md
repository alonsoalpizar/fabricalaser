# fixes-v3.md — Bounding Box Real + Geometría Escalada por Cantidad

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## Dos cambios relacionados en un solo fix

### Cambio A: Usar bounding box real en vez de canvas
Hoy `AreaConsumedMM2 = analysis.Width × analysis.Height` usa el canvas del SVG. Si el canvas es 300×300mm pero el diseño es un círculo de 50mm, cobra material por 90,000mm² cuando debería ser 2,500mm². Usar `analysis.TotalArea()` que ya existe y calcula el bounding box real.

### Cambio B: Escalar geometría × quantity en vez de multiplicar precio
Hoy calcula precio de 1 pieza y multiplica × qty. Esto no refleja la operación real. La solución: multiplicar la geometría del SVG × qty ANTES de calcular, luego calcular todo como qty=1 con setup una vez.

```
ANTES: precio_1_pieza × qty
AHORA: geometría × qty → calcular 1 vez → setup 1 vez → precio total / qty = unitario
```

---

## Archivos a Modificar

```
internal/services/pricing/calculator.go     — Cambios A y B
internal/services/pricing/time_estimator.go — Nuevo método EstimateWithGeometry
```

---

## Paso 1: time_estimator.go — Agregar EstimateWithGeometry

Agregar método nuevo que recibe geometría directa (ya escalada por qty) en vez de un SVGAnalysis. No modificar métodos existentes, solo agregar:

```go
// EstimateWithGeometry calcula tiempo con geometría explícita (ya escalada por qty)
// Usa spot_size para convertir velocidad cabezal a velocidad raster automáticamente
func (e *TimeEstimator) EstimateWithGeometry(
    rasterAreaMM2 float64,
    vectorLengthMM float64,
    cutLengthMM float64,
    techID uint,
    materialID uint,
    engraveTypeID uint,
    thickness float64,
) TimeEstimate {
    estimate := TimeEstimate{}

    speedMult := e.config.GetEngraveTypeSpeedMultiplier(engraveTypeID)
    if speedMult <= 0 {
        speedMult = 1.0
    }

    materialFactor := e.config.GetMaterialFactor(materialID)
    if materialFactor <= 0 {
        materialFactor = 1.0
    }

    spotSize := e.config.GetSpotSize(techID)
    specificSpeed := e.config.GetMaterialSpeed(techID, materialID, thickness)

    baseEngraveLineSpeed := e.config.GetBaseEngraveLineSpeed()
    baseCutSpeed := e.config.GetBaseCutSpeed()
    setupTimeMinutes := e.config.GetSetupTimeMinutes()

    // Raster (área): convertir velocidad cabezal a mm²/min con spot_size
    if rasterAreaMM2 > 0 {
        var engraveSpeedMmMin float64
        if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
            engraveSpeedMmMin = *specificSpeed.EngraveSpeedMmMin * speedMult
        } else {
            engraveSpeedMmMin = baseEngraveLineSpeed * speedMult / materialFactor
            estimate.UsedFallback = true
        }
        effectiveRasterSpeed := engraveSpeedMmMin * spotSize
        estimate.EngraveMins += rasterAreaMM2 / effectiveRasterSpeed
    }

    // Vector (líneas): velocidad cabezal directa en mm/min
    if vectorLengthMM > 0 {
        var effectiveVectorSpeed float64
        if specificSpeed.Found && specificSpeed.EngraveSpeedMmMin != nil && *specificSpeed.EngraveSpeedMmMin > 0 {
            effectiveVectorSpeed = *specificSpeed.EngraveSpeedMmMin * speedMult
        } else {
            effectiveVectorSpeed = baseEngraveLineSpeed * speedMult / materialFactor
            estimate.UsedFallback = true
        }
        estimate.EngraveMins += vectorLengthMM / effectiveVectorSpeed
    }

    // Corte
    if cutLengthMM > 0 {
        var effectiveCutSpeed float64
        if specificSpeed.Found && specificSpeed.CutSpeedMmMin != nil && *specificSpeed.CutSpeedMmMin > 0 {
            effectiveCutSpeed = *specificSpeed.CutSpeedMmMin
        } else {
            effectiveCutSpeed = baseCutSpeed / materialFactor
            estimate.UsedFallback = true
        }
        estimate.CutMins = cutLengthMM / effectiveCutSpeed
    }

    // Setup UNA vez (no se multiplica)
    estimate.SetupMins = setupTimeMinutes
    estimate.TotalMins = estimate.SetupMins + estimate.EngraveMins + estimate.CutMins

    return estimate
}
```

**NOTA:** Este método usa `GetSpotSize()` que fue agregado en fixes-v2. Verificar que existe antes de continuar. Si no existe, revisar fixes-v2.

---

## Paso 2: calculator.go — Reemplazar lógica de cálculo

### 2a. Escalar geometría ANTES de calcular tiempos

Buscar donde se llama al TimeEstimator. Reemplazar con:

```go
// =============================================================
// ESCALAR GEOMETRÍA POR CANTIDAD
// Multiplicamos la geometría × qty ANTES de calcular.
// Esto simula "un SVG con todas las piezas" y refleja la operación real:
// un solo setup, un solo job de máquina, material sobre área total.
// =============================================================

// Geometría escalada (Cambio A: TotalArea en vez de Width×Height)
scaledCutLength := analysis.CutLengthMM * float64(quantity)
scaledVectorLength := analysis.VectorLengthMM * float64(quantity)
scaledRasterArea := analysis.RasterAreaMM2 * float64(quantity)
scaledMaterialArea := analysis.TotalArea() * float64(quantity) // Bounding box real, no canvas

// Calcular tiempo sobre geometría escalada, qty=1
timeEstimator := NewTimeEstimator(config)
timeEst := timeEstimator.EstimateWithGeometry(
    scaledRasterArea,
    scaledVectorLength,
    scaledCutLength,
    techID, materialID, engraveTypeID, thickness,
)

result.TimeEngraveMins = timeEst.EngraveMins
result.TimeCutMins = timeEst.CutMins
result.TimeSetupMins = timeEst.SetupMins
result.TimeTotalMins = timeEst.TotalMins
```

### 2b. Área de material con geometría escalada

Buscar `result.AreaConsumedMM2 = analysis.Width * analysis.Height` y reemplazar:

```go
// Material: área total escalada (bounding box real × qty)
result.AreaConsumedMM2 = scaledMaterialArea
```

### 2c. Costos base — ya NO dividir entre qty

Los costos de tiempo ya reflejan el total (geometría escalada). No dividir:

```go
result.CostEngrave = timeEst.EngraveMins * costPerMinEngrave
result.CostCut = timeEst.CutMins * costPerMinCut
result.CostSetup = setupFee
result.CostBase = result.CostEngrave + result.CostCut
```

### 2d. Precio Hybrid — sin ida y vuelta de qty

Reemplazar toda la sección de cálculo Hybrid con:

```go
// =============================================================
// HYBRID PRICING — sobre geometría escalada
// El costo base YA incluye todas las piezas
// =============================================================

machineCost := result.CostBase
materialCost := result.CostMaterialWithWaste

totalCostBase := machineCost + materialCost

hybridTotal := totalCostBase
hybridTotal *= (1 + result.FactorMargin)
hybridTotal *= result.FactorEngrave
hybridTotal *= (1 + result.FactorUVPremium)

// Descuento volumen
hybridTotal *= (1 - result.DiscountVolumePct)

// Setup UNA vez
hybridTotal += result.CostSetup

result.PriceHybridTotal = math.Round(hybridTotal*100) / 100

// Unitario es referencia: total / qty
result.PriceHybridUnit = math.Round((hybridTotal/float64(quantity))*100) / 100
```

### 2e. Precio Value-Based — sobre área escalada

Reemplazar la sección Value-Based:

```go
// =============================================================
// VALUE-BASED PRICING — sobre área escalada
// =============================================================

totalArea := scaledMaterialArea
if totalArea < minAreaMM2 {
    totalArea = minAreaMM2
}

valueBase := math.Max(minValueBase, totalArea*pricePerMM2)
valueBase *= result.FactorMaterial
valueBase *= result.FactorEngrave
valueBase *= (1 + result.FactorUVPremium)

valueTotal := valueBase
valueTotal *= (1 - result.DiscountVolumePct)
valueTotal += result.CostSetup

result.PriceValueTotal = math.Round(valueTotal*100) / 100
result.PriceValueUnit = math.Round((valueTotal/float64(quantity))*100) / 100
```

---

## Verificación

```bash
# 1. Compilar
go build ./...

# 2. No debe haber Width * Height para área
grep -n "Width \* analysis.Height\|Width\*analysis.Height" internal/services/pricing/calculator.go
# Debe dar 0 resultados

# 3. TotalArea() se usa para escalar
grep -n "TotalArea" internal/services/pricing/calculator.go
# Debe aparecer en el escalado de geometría

# 4. EstimateWithGeometry existe
grep -n "EstimateWithGeometry" internal/services/pricing/time_estimator.go
# Debe existir

# 5. No debe haber perUnitMachineCost = CostBase / quantity
grep -n "CostBase.*quantity\|quantity.*CostBase" internal/services/pricing/calculator.go
# Debe dar 0 resultados (ya no se divide)
```

## Validación mental

10 círculos 50mm, MDF 3mm, CO2, Vectorial:
```
Geometría: cut=1570mm, área=25000mm²
Tiempo: 1570/2200=0.71min + 5 setup = 5.71min
Máquina: 0.71×135.45 = ₡96.17
Material: 25000×0.00167978×1.15 = ₡48.30
Base: ₡144.47 × 1.40 = ₡202.26
- 5% descuento = ₡192.15 + ₡4000 setup = ₡4,192
Unitario: ₡419

Value: max(3000, 25000×0.515)=₡12,875 × 0.95 + ₡4000 = ₡16,231
Unitario: ₡1,623

Final = MAX(₡4,192, ₡16,231) = ₡16,231
```