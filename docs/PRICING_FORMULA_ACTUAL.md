# Formula Real del Cotizador FabricaLaser

Documento generado por auditoria de codigo - 2026-02-26

## Fuentes del Codigo

- `internal/services/pricing/calculator.go` - Motor principal
- `internal/services/pricing/time_estimator.go` - Estimador de tiempos
- `internal/services/pricing/config_loader.go` - Carga de configuracion desde BD

---

## 1. ESTIMACION DE TIEMPO (TimeEstimator.Estimate)

### Variables de Entrada (del analisis SVG)
```
analysis.RasterAreaMM2   - Area de grabado raster (negro) en mm²
analysis.VectorLengthMM  - Longitud de lineas de grabado vector (azul) en mm
analysis.CutLengthMM     - Longitud de corte (rojo) en mm
```

### Parametros de Configuracion
```
speedMult = EngraveType.SpeedMultiplier     (ej: Vectorial=1.0, Rasterizado=0.5)
materialFactor = Material.Factor            (ej: MDF=1.0, Acrilico=1.2)
```

### Lookup de Velocidades (tech_material_speeds)

El sistema busca velocidad especifica por `(tech_id, material_id, thickness)`:

```go
specificSpeed = config.GetMaterialSpeed(techID, materialID, thickness)
// Si no encuentra exacto, intenta con thickness=0
// Si no encuentra nada, Found=false → usa fallback
```

### Calculo de Tiempo de Grabado Raster
```
SI specificSpeed.Found Y specificSpeed.EngraveSpeedMmMin > 0:
    effectiveRasterSpeed = specificSpeed.EngraveSpeedMmMin * speedMult
SINO:
    effectiveRasterSpeed = baseEngraveAreaSpeed * speedMult / materialFactor
    // baseEngraveAreaSpeed = 500 (de system_config)

rasterTime = RasterAreaMM2 / effectiveRasterSpeed
EngraveMins += rasterTime
```

### Calculo de Tiempo de Grabado Vector
```
SI specificSpeed.Found Y specificSpeed.EngraveSpeedMmMin > 0:
    effectiveVectorSpeed = specificSpeed.EngraveSpeedMmMin * speedMult
SINO:
    effectiveVectorSpeed = baseEngraveLineSpeed * speedMult / materialFactor
    // baseEngraveLineSpeed = 100 (de system_config)

vectorTime = VectorLengthMM / effectiveVectorSpeed
EngraveMins += vectorTime
```

### Calculo de Tiempo de Corte
```
SI specificSpeed.Found Y specificSpeed.CutSpeedMmMin > 0:
    effectiveCutSpeed = specificSpeed.CutSpeedMmMin
    // NOTA: NO aplica speedMult al corte (bug o feature?)
SINO:
    effectiveCutSpeed = baseCutSpeed / materialFactor
    // baseCutSpeed = 20 (de system_config)

CutMins = CutLengthMM / effectiveCutSpeed
```

### Tiempo Total
```
SetupMins = setup_time_minutes  // 5 minutos (de system_config)
perUnitTime = EngraveMins + CutMins
TotalMins = SetupMins + (perUnitTime * quantity)

// LUEGO ajusta engrave y cut para reflejar totales:
EngraveMins *= quantity
CutMins *= quantity
```

---

## 2. CALCULO DE COSTOS BASE (Calculator.Calculate)

### Costos de Tiempo (Maquina)
```
CostEngrave = TimeEngraveMins * CostPerMinEngrave[techID]
CostCut = TimeCutMins * CostPerMinCut[techID]
CostSetup = SetupFee[techID]  // NOTA: Actualmente 0 en BD
CostBase = CostEngrave + CostCut  // SIN setup
```

### Costo de Material (si material_included=true)
```
AreaConsumedMM2 = analysis.Width * analysis.Height

SI materialIncluded:
    matCost = config.GetMaterialCost(materialID, thickness)
    SI matCost.Found Y matCost.CostPerMm2 > 0:
        WastePct = matCost.WastePct  // 0.15 (15%)
        CostMaterialRaw = AreaConsumedMM2 * matCost.CostPerMm2
        CostMaterialWithWaste = CostMaterialRaw * (1 + WastePct)
    SINO:
        WastePct = default_waste_pct  // 0.15
        CostMaterialRaw = 0
        CostMaterialWithWaste = 0
SINO:
    WastePct = 0
    CostMaterialRaw = 0
    CostMaterialWithWaste = 0
```

---

## 3. MODELO HIBRIDO DE PRECIOS (Principal)

### Factores
```
FactorMaterial = Material.Factor        (1.0 - 1.8)
FactorEngrave = EngraveType.Factor      (1.0 - 3.0)
FactorUVPremium = Technology.UVPremiumFactor  (0.0 o 0.2)
FactorMargin = TechRate.MarginPercent   (0.40 o 0.45)
DiscountVolumePct = config.GetVolumeDiscount(quantity)  (0.0 - 0.20)
```

### Precio Unitario Hibrido
```
perUnitMachineCost = CostBase / quantity
perUnitMaterialCost = CostMaterialWithWaste  // Material es POR PIEZA

perUnitCostBase = perUnitMachineCost + perUnitMaterialCost

hybridUnit = perUnitCostBase
hybridUnit *= (1 + FactorMargin)         // Margen (40-45%)
hybridUnit *= FactorEngrave              // Factor grabado
hybridUnit *= (1 + FactorUVPremium)      // Premium UV (+20% si UV)

PriceHybridUnit = round(hybridUnit * 100) / 100
```

### Precio Total Hibrido
```
hybridTotal = PriceHybridUnit * quantity
hybridTotal *= (1 - DiscountVolumePct)   // Descuento volumen
hybridTotal += CostSetup                 // Setup UNA vez (actualmente 0)

PriceHybridTotal = round(hybridTotal * 100) / 100
```

---

## 4. MODELO BASADO EN VALOR (Alternativo)

```
totalArea = analysis.TotalArea()  // Width * Height
SI totalArea < min_area_mm2:
    totalArea = min_area_mm2  // 100 mm²

valueBase = max(min_value_base, totalArea * price_per_mm2)
// min_value_base = 3000 CRC
// price_per_mm2 = 0.515 CRC

valueUnit = valueBase
valueUnit *= FactorMaterial   // NOTA: Hibrido NO usa FactorMaterial aqui
valueUnit *= FactorEngrave
valueUnit *= (1 + FactorUVPremium)

PriceValueUnit = round(valueUnit * 100) / 100

valueTotal = PriceValueUnit * quantity
valueTotal *= (1 - DiscountVolumePct)
valueTotal += CostSetup

PriceValueTotal = round(valueTotal * 100) / 100
```

---

## 5. CLASIFICACION DE COMPLEJIDAD

```
complexityFactor = analysis.ComplexityFactor()
// Definido en models/svg_analysis.go

SI complexityFactor <= complexity_auto_approve:  // 6.0
    Status = "auto_approved"
SINO SI complexityFactor <= complexity_needs_review:  // 12.0
    Status = "needs_review"
SINO:
    Status = "rejected"
```

---

## 6. VALORES ACTUALES EN BASE DE DATOS

### system_config
| Key | Value |
|-----|-------|
| base_engrave_area_speed | 500 mm²/min |
| base_engrave_line_speed | 100 mm/min |
| base_cut_speed | 20 mm/min |
| setup_time_minutes | 5 min |
| complexity_auto_approve | 6.0 |
| complexity_needs_review | 12.0 |
| quote_validity_days | 7 |
| min_value_base | 3000 CRC |
| price_per_mm2 | 0.515 CRC |
| min_area_mm2 | 100 mm² |

### tech_rates (Tarifas por Tecnologia)
| Tech ID | Tech | Cost/Min Engrave | Cost/Min Cut | Setup Fee | Margin |
|---------|------|------------------|--------------|-----------|--------|
| 1 | CO2 | 118.28 | 135.45 | **0.00** | 40% |
| 2 | UV | 135.45 | 152.62 | **0.00** | 40% |
| 3 | FIBRA | 197.42 | 214.58 | **0.00** | 45% |
| 4 | MOPA | 214.58 | 214.58 | **0.00** | 45% |

### materials (Factores de Material)
| ID | Material | Factor |
|----|----------|--------|
| 1 | Madera/MDF | 1.00 |
| 2 | Acrilico | 1.20 |
| 3 | Plastico ABS/PC | 1.25 |
| 4 | Cuero/Piel | 1.30 |
| 6 | Ceramica | 1.60 |
| 7 | Metal con coating | 1.80 |
| 12 | Vidrio/Cristal | 1.50 |

### engrave_types (Tipos de Grabado)
| ID | Tipo | Factor Precio | Speed Mult |
|----|------|---------------|------------|
| 1 | Vectorial | 1.00 | 1.00 |
| 2 | Rasterizado | 1.50 | 0.50 |
| 3 | Fotograbado | 2.50 | 0.20 |
| 4 | 3D/Relieve | 3.00 | 0.15 |

### volume_discounts
| Min Qty | Max Qty | Descuento |
|---------|---------|-----------|
| 1 | 9 | 0% |
| 10 | 24 | 5% |
| 25 | 49 | 10% |
| 50 | 99 | 15% |
| 100 | - | 20% |

### tech_material_speeds (Ejemplo CO2 + MDF)
| Grosor | Cut Speed | Engrave Speed |
|--------|-----------|---------------|
| 3mm | **2200** mm/min | 4000 mm/min |
| 5mm | 1600 mm/min | 4000 mm/min |
| 6mm | 1200 mm/min | 4000 mm/min |
| 10mm | 700 mm/min | 4000 mm/min |

---

## 7. OBSERVACIONES CRITICAS

### OBS-1: Setup Fee es 0
Todas las tecnologias tienen `setup_fee = 0` en `tech_rates`.
El codigo suma `CostSetup` pero siempre es 0.

### OBS-2: FactorMaterial NO aplica en modelo hibrido
En `calculator.go:165-168`, el modelo hibrido aplica:
- FactorMargin
- FactorEngrave
- FactorUVPremium

Pero **NO** aplica `FactorMaterial` al precio (solo al tiempo via speed).
El modelo de valor SI lo aplica (linea 198).

### OBS-3: SpeedMult NO aplica al corte
En `time_estimator.go:92-94`, la velocidad de corte usa:
```go
effectiveCutSpeed = *specificSpeed.CutSpeedMmMin
```
Sin multiplicar por `speedMult`. Esto es intencional (corte no varia por tipo de grabado).

### OBS-4: Material cost es por unidad, no compartido
El costo de material (`CostMaterialWithWaste`) se suma por unidad, no se divide.
Esto significa que si pides 10 unidades, pagas 10x el costo de material.

### OBS-5: Velocidad de corte MDF 3mm es 2200, no 1800
La matriz tiene 2200 mm/min para CO2+MDF+3mm, no 1800.

---

## 8. ORDEN DE APLICACION DE FACTORES (Modelo Hibrido)

```
1. Calcular tiempo (con speedMult y materialFactor en velocidades)
2. Calcular costo base (tiempo * tarifa)
3. Calcular costo material (area * precio_mm2 * (1+waste))
4. Sumar: perUnitCost = (costoBase/qty) + costoMaterial
5. Aplicar margen: *= (1 + marginPct)
6. Aplicar factor grabado: *= factorEngrave
7. Aplicar premium UV: *= (1 + uvPremium)
8. Multiplicar por cantidad: * qty
9. Aplicar descuento volumen: *= (1 - discountPct)
10. Sumar setup (0 actualmente)
```
