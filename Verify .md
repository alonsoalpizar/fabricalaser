# verify.md — Verificación Post-Fixes

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## Fase 1: Verificación de Código (automática)

Ejecutar TODOS estos checks en orden. Reportar cada uno con ✅ o ❌.

### 1.1 Compilación
```bash
cd /path/to/fabricalaser
go build ./...
echo "EXIT CODE: $?"
```
Esperado: EXIT CODE 0

### 1.2 No quedan referencias a columnas eliminadas
```bash
grep -rn "RasterSpeedMm2Min\|VectorSpeedMmMin\|raster_speed_mm2_min\|vector_speed_mm_min" internal/
```
Esperado: 0 resultados

### 1.3 Spot size existe en model y config_loader
```bash
grep -n "SpotSizeMM\|spot_size_mm\|GetSpotSize" internal/models/ internal/services/pricing/
```
Esperado: aparece en technology.go, config_loader.go, time_estimator.go

### 1.4 EstimateWithGeometry existe
```bash
grep -n "EstimateWithGeometry" internal/services/pricing/time_estimator.go
```
Esperado: definición del método

### 1.5 TotalArea() se usa para material (no Width × Height)
```bash
grep -n "TotalArea\|Width.*Height" internal/services/pricing/calculator.go
```
Esperado: TotalArea aparece, Width*Height NO aparece para área de material

### 1.6 MaterialIncluded es puntero (*bool)
```bash
grep -n "MaterialIncluded" internal/models/quote.go
```
Esperado: `*bool` no `bool`

### 1.7 PriceFinal usa MAX(Hybrid, Value)
```bash
grep -n "math.Max\|PriceFinal\|PriceModel" internal/services/pricing/calculator.go
```
Esperado: math.Max con hybrid y value, PriceModel asignado

### 1.8 Validación de compatibilidad en handler
```bash
grep -n "IsCompatible\|INCOMPATIBLE\|incompatible" internal/handlers/quote/
```
Esperado: validación antes de llamar Calculate

### 1.9 Fallback warning en modelo
```bash
grep -n "UsedFallbackSpeeds\|FallbackWarning\|used_fallback" internal/models/quote.go internal/services/pricing/
```
Esperado: campos existen en modelo y se asignan en calculator/time_estimator

### 1.10 Overhead dinámico en config_loader
```bash
grep -n "GetOverheadGlobalPerHourCRC\|GetOverheadMaquinaPerHourCRC\|GetHorasTrabajoMes" internal/services/pricing/config_loader.go
```
Esperado: 3 métodos de cálculo dinámico:
- GetHorasTrabajoMes() - horas trabajo estimadas
- GetOverheadGlobalPerHourCRC() - overhead global (alquiler+internet) / horas
- GetOverheadMaquinaPerHourCRC(techID) - overhead máquina / horas

### 1.11 cost_per_min se calcula dinámicamente
```bash
grep -n "GetCostPerMinEngrave\|GetCostPerMinCut" internal/services/pricing/config_loader.go
```
Esperado: usan EngraveRateHour + GetOverheadPerHourCRC, no leen campo fijo

### 1.12 Geometría escalada en calculator
```bash
grep -n "scaledCutLength\|scaledRasterArea\|scaledMaterialArea\|scaledVectorLength" internal/services/pricing/calculator.go
```
Esperado: variables de escalado presentes

### 1.13 Overhead architecture (global + per-machine)
```bash
# Global overhead en system_config
grep -n "overhead_alquiler\|overhead_internet\|horas_trabajo_mes" seeds/
# Machine overhead en tech_rates model
grep -n "ElectricidadMes\|MantenimientoMes\|DepreciacionMes\|SeguroMes\|ConsumiblesMes" internal/models/tech_rate.go
```
Esperado:
- system_config: overhead_alquiler, overhead_internet, horas_trabajo_mes (costos globales del taller)
- tech_rates: electricidad_mes, mantenimiento_mes, depreciacion_mes, seguro_mes, consumibles_mes (costos por máquina)

### 1.14 No hay hardcoded 135.45 o 152.62
```bash
grep -rn "135.45\|152.62\|135\.45\|152\.62" internal/services/pricing/
```
Esperado: 0 resultados (estos valores ahora se calculan dinámicamente)

### 1.15 Cantidad no multiplica precio (geometría escalada)
```bash
grep -n "quantity" internal/services/pricing/calculator.go
```
Verificar: quantity se usa SOLO para escalar geometría y calcular unitario (total/qty), NO para multiplicar precio

### 1.16 Admin: campos calculados readonly en formulario tarifas
```bash
grep -n "recalcularCostos\|field-calculated\|readonly.*overhead\|readonly.*cost_per_min" templates/admin/
```
Esperado: función recalcularCostos, campos con readonly y class field-calculated

### 1.17 Admin: labels renombrados
```bash
grep -n "Tarifa Operador\|Costos Fijos" templates/admin/
```
Esperado: "Tarifa Operador Grabado", "Tarifa Operador Corte", "Costos Fijos por Hora"

### 1.18 Overhead calculado (frontend o backend)
```bash
# Opción A: Frontend calcula con JS
grep -n "recalcularCostos\|GetOverheadGlobalPerHourCRC\|GetOverheadMaquinaPerHourCRC" web/admin/ internal/services/pricing/config_loader.go
# Opción B: Backend provee via API
grep -n "overhead" internal/handlers/admin/
```
Esperado: El overhead se calcula en:
- Backend: `GetOverheadGlobalPerHourCRC()` y `GetOverheadMaquinaPerHourCRC()` en config_loader.go
- Frontend admin: `recalcularCostos()` en rates.html para preview en tiempo real

---

## Fase 2: Verificación de Precios (manual, con Alonso)

Ejecutar estos 7 casos en el cotizador real. Copiar el JSON de respuesta del API y pegar en la conversación con Claude para validación.

### Caso 1 — Mínimo absoluto
```
SVG: Cuadrado pequeño 20×20mm, solo corte (líneas rojas)
Config: MDF 3mm, CO2, Vectorial, qty=1, material incluido
```
**Verificar:**
- PriceFinal ≥ ₡3,000 + setup (piso Value-Based)
- PriceModel = "value" (porque el Hybrid sería muy bajo)
- Setup fee = ₡4,000

### Caso 2 — Raster con spot_size
```
SVG: Logo relleno ~50×50mm (áreas negras = raster + contorno rojo = corte)
Config: MDF 3mm, CO2, Rasterizado, qty=1, material incluido
```
**Verificar:**
- EngraveMins > 1 minuto (si sale < 0.5 min, spot_size no se aplicó)
- Factor grabado = 1.50x (rasterizado)
- SpeedMult = 0.5 (rasterizado)

### Caso 3 — Volumen con geometría escalada
```
SVG: Círculo 50mm (solo corte rojo)
Config: MDF 3mm, CO2, Vectorial, qty=1 PRIMERO, luego qty=10
```
**Verificar:**
- qty=1: setup ₡4,000 domina el precio
- qty=10: setup sigue ₡4,000 (no ₡40,000)
- Precio unitario de qty=10 << precio de qty=1
- Descuento 5% aplicado (10 piezas)

### Caso 4 — Incompatibilidad
```
Seleccionar: Material=Madera/MDF, luego buscar Tecnología=FIBRA
```
**Verificar:**
- FIBRA no aparece como opción compatible
- Si por alguna razón se envía al API: error de incompatibilidad

### Caso 5 — Material no incluido
```
SVG: cualquiera
Config: cualquier combo válido, qty=1
Toggle: "No, cliente provee"
```
**Verificar:**
- En resultado: material_included = false
- Costo material = ₡0 (cliente provee)
- El toggle persiste correctamente (no se resetea a true)

### Caso 6 — Fallback (grosor sin datos)
```
SVG: cualquiera con corte
Config: MDF, CO2, grosor que NO esté en la matriz (ej: 7mm o 8mm)
Si el sistema no ofrece ese grosor, usar un grosor edge que pueda caer a fallback
```
**Verificar:**
- Warning de fallback aparece en resultado
- used_fallback_speeds = true
- Precio es conservador (más alto que con velocidad calibrada)

### Caso 7 — Canvas grande, diseño pequeño
```
SVG: Canvas 300×300mm con un solo círculo de 50mm
Config: MDF 3mm, CO2, Vectorial, qty=1
```
**Verificar:**
- AreaConsumedMM2 ≈ 2,500 mm² (bounding box del círculo)
- NO 90,000 mm² (canvas completo)
- Material cost coherente con 2,500mm², no con 90,000mm²

---

## Formato de Reporte

Para cada caso, reportar:

```
CASO X: [nombre]
  Input: [configuración usada]
  PriceFinal: ₡XX,XXX
  PriceModel: hybrid/value
  PriceHybridTotal: ₡XX,XXX
  PriceValueTotal: ₡XX,XXX
  TimeEngrave: X.X min
  TimeCut: X.X min
  TimeSetup: X.X min
  AreaConsumed: X,XXX mm²
  MaterialIncluded: true/false
  UsedFallback: true/false
  Setup Fee: ₡X,XXX
  Discount: X%
  ✅/❌ [observaciones]
```

---

## Criterios de Aprobación

- Fase 1: TODOS los checks ✅ (cualquier ❌ requiere fix antes de continuar)
- Fase 2: Los 7 casos dan resultados coherentes con la fórmula documentada
- NO hay precios negativos, NaN, Infinity, o ₡0 en ningún caso normal
- El modelo Value-Based siempre tiene un piso ≥ min_value_base
- Los tiempos de raster son razonables (minutos, no milisegundos)
- El setup se cobra exactamente 1 vez por cotización