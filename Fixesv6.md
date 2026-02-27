# fixes-v6.md — Value-Based por Perímetro para Solo Corte + Bug Costo Máquina

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## NO usar subagentes. Secuencial.

---

## PARTE A: Investigar Bug — Costo Máquina = ₡0

### Síntoma
Al cotizar 100 círculos 50mm, Acrílico 3mm, CO2, solo corte:
- Tiempo corte = 8.7 min (CORRECTO)
- Costo máquina (tiempo) = ₡0 (BUG — debería ser ~₡1,195)
- El Hybrid parece no estar calculando correctamente

### Investigar

1. Verificar que `GetCostPerMinEngrave(techID)` y `GetCostPerMinCut(techID)` retornan valores > 0:

```go
// En calculator.go, buscar dónde se obtienen estos valores
costPerMinEngrave := config.GetCostPerMinEngrave(techID)
costPerMinCut := config.GetCostPerMinCut(techID)
```

Agregar log temporal si es necesario:
```go
log.Printf("DEBUG PRICING: techID=%d, costPerMinEngrave=%.2f, costPerMinCut=%.2f", techID, costPerMinEngrave, costPerMinCut)
```

2. Verificar que `techID` llega correctamente al calculator (no es 0 o nil)

3. Verificar que `TechRates[techID]` tiene datos cargados en el config_loader:
```go
log.Printf("DEBUG PRICING: TechRates keys: %v", reflect.ValueOf(config.TechRates).MapKeys())
```

4. Verificar que `EngraveRateHour` y `CutRateHour` tienen valores en el struct TechRate:
```go
rate := config.TechRates[techID]
if rate != nil {
    log.Printf("DEBUG PRICING: EngraveRateHour=%.2f, CutRateHour=%.2f", rate.EngraveRateHour, rate.CutRateHour)
}
```

5. Verificar que las columnas nuevas (electricidad_mes, etc.) no rompieron el GORM autoload del struct TechRate. Si GORM falla silenciosamente al cargar un struct con columnas que no existen en BD, podría devolver struct vacío.

```bash
# Verificar que la migración corrió
psql -c "SELECT column_name FROM information_schema.columns WHERE table_name='tech_rates' ORDER BY ordinal_position;"
```

### Posibles causas (en orden de probabilidad)

1. **GetCostPerMinEngrave ahora requiere techID pero se llama sin él** — La firma cambió en fixes-v5 pero calculator.go no se actualizó
2. **TechRates no se carga por cambio en el struct** — Columnas nuevas causan error silencioso en GORM
3. **EngraveRateHour se llama diferente** — El campo en Go no mapea a la columna de BD correctamente
4. **El overhead global retorna 0** — Los items de system_config no se insertaron correctamente

### Fix

Una vez identificada la causa, corregir. Después de corregir, verificar con el mismo caso:
```
100 círculos 50mm, Acrílico 3mm, CO2, solo corte, qty=100
Esperado: CostCut > ₡1,000 (8.7 min × ~₡137/min)
```

---

## PARTE B: Value-Based por Perímetro para Solo Corte

### Problema

El Value-Based usa `área × price_per_mm2` para TODO. Esto funciona bien cuando hay grabado (raster/vector) porque el área refleja la complejidad. Pero para trabajos de **solo corte**, el valor está en el perímetro, no en el área. Un círculo de 50mm tiene 2,500mm² de área pero solo 157mm de corte.

Ejemplo del bug: 100 círculos cortados → Value = ₡127,600 (absurdo) vs Hybrid ~₡7,000 (razonable).

### Solución

Agregar `price_per_mm_cut` a system_config y usar perímetro cuando es solo corte.

### Paso 1: Nuevo config

```sql
INSERT INTO system_config (config_key, config_value, value_type, category, description) VALUES
('price_per_mm_cut', '0.25', 'number', 'pricing', 'Tarifa por mm lineal de corte para Value-Based en trabajos de solo corte (₡/mm)')
ON CONFLICT (config_key) DO UPDATE SET config_value = EXCLUDED.config_value, description = EXCLUDED.description;
```

**NOTA:** El valor ₡0.25/mm es estimado (₡500/min mercado ÷ 2000mm/min velocidad promedio). Puede ajustarse después de validar con costos reales de máquina (depende de resolver el bug de Parte A).

### Paso 2: config_loader.go

```go
func (c *PricingConfig) GetPricePerMmCut() float64 {
    return c.GetSystemConfigFloat("price_per_mm_cut", 0.25)
}
```

### Paso 3: calculator.go — Modificar sección Value-Based

Detectar si es trabajo de solo corte y usar perímetro:

```go
// =============================================================
// VALUE-BASED PRICING — adaptativo por tipo de trabajo
// Solo corte → usa perímetro × price_per_mm_cut
// Con grabado → usa área × price_per_mm2 (como antes)
// =============================================================

var valueBase float64

hasEngrave := scaledRasterArea > 0 || scaledVectorLength > 0
isOnlyCut := !hasEngrave && scaledCutLength > 0

if isOnlyCut {
    // Solo corte: valor por perímetro
    pricePerMmCut := config.GetPricePerMmCut()
    valueBase = math.Max(minValueBase, scaledCutLength * pricePerMmCut)
} else {
    // Con grabado: valor por área (comportamiento original)
    totalArea := scaledMaterialArea
    if totalArea < minAreaMM2 {
        totalArea = minAreaMM2
    }
    valueBase = math.Max(minValueBase, totalArea * pricePerMM2)
}

// Aplicar factores (igual para ambos casos)
valueBase *= result.FactorMaterial
valueBase *= result.FactorEngrave
valueBase *= (1 + result.FactorUVPremium)

valueTotal := valueBase
valueTotal *= (1 - result.DiscountVolumePct)
valueTotal += result.CostSetup

result.PriceValueTotal = math.Round(valueTotal*100) / 100
result.PriceValueUnit = math.Round((valueTotal/float64(quantity))*100) / 100
```

### Paso 4: Actualizar frontend — badge del modelo

El badge "Precio por area" debe decir "Precio por perímetro" cuando es solo corte. Agregar al JSON de respuesta:

En el calculator, agregar al result:
```go
if result.PriceModel == "value" {
    if isOnlyCut {
        result.PriceModelDetail = "perimeter"  // "Precio por perímetro de corte"
    } else {
        result.PriceModelDetail = "area"  // "Precio por área"
    }
}
```

En el modelo Quote, agregar campo:
```go
PriceModelDetail string `json:"price_model_detail,omitempty"`
```

En el frontend (index.html), actualizar el JS de displayResult:
```javascript
const modelLabels = {
    'hybrid': { text: 'Precio basado en tiempo de producción', class: 'hybrid' },
    'value':  { text: 'Precio basado en valor del servicio', class: 'value' }
};

// Detalle del modelo value
if (model === 'value') {
    const detail = pricing.price_model_detail || 'area';
    if (detail === 'perimeter') {
        modelLabels['value'].text = 'Precio por perímetro de corte';
    } else {
        modelLabels['value'].text = 'Precio por área';
    }
}
```

---

## Verificación

```bash
# 1. Compilar
go build ./...

# 2. Verificar price_per_mm_cut en config
grep -n "price_per_mm_cut\|PricePerMmCut" internal/services/pricing/

# 3. Verificar lógica isOnlyCut
grep -n "isOnlyCut\|hasEngrave" internal/services/pricing/calculator.go

# 4. Verificar PriceModelDetail
grep -n "PriceModelDetail\|price_model_detail" internal/models/ internal/services/pricing/
```

## Validación — Caso de prueba

100 círculos 50mm, Acrílico 3mm, CO2, solo corte:

```
ANTES (bug):
  Value = 250,000mm² × 0.515 × 1.2 × 0.80 + 4000 = ₡127,600
  → Absurdo para solo corte

DESPUÉS (fix):
  Value = 15,700mm × 0.25 × 1.2 × 0.80 + 4000 = ₡7,768
  → Razonable, ligeramente arriba del Hybrid (~₡7,000)
  
  Unitario: ₡78 c/u ← precio justo para cortar un círculo de acrílico
```

## Orden de ejecución

1. **PRIMERO resolver Parte A** (bug costo máquina = ₡0)
2. **DESPUÉS implementar Parte B** (Value por perímetro)
3. Validar con el mismo caso de prueba