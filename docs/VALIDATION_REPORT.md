# Reporte de Validacion del Cotizador FabricaLaser

**Fecha:** 2026-02-26
**Auditor:** Claude (Auditor automatizado)
**Version del codigo:** Actual en produccion

---

## 1. Formula Real Documentada

Ver documento completo: [PRICING_FORMULA_ACTUAL.md](./PRICING_FORMULA_ACTUAL.md)

### Resumen de la Formula

```
TIEMPO:
  tiempo_grabado = (raster_area / velocidad_raster) + (vector_length / velocidad_vector)
  tiempo_corte = cut_length / velocidad_corte
  tiempo_total = setup + (tiempo_grabado + tiempo_corte) * cantidad

PRECIO HIBRIDO (Principal):
  costo_maquina = (tiempo_grabado * tarifa_grabado) + (tiempo_corte * tarifa_corte)
  costo_material = area_consumida * precio_mm2 * (1 + merma)

  precio_unitario = (costo_maquina/cantidad + costo_material)
  precio_unitario *= (1 + margen)
  precio_unitario *= factor_grabado
  precio_unitario *= (1 + premium_uv)

  precio_total = precio_unitario * cantidad
  precio_total *= (1 - descuento_volumen)
  precio_total += setup_fee
```

---

## 2. Resultados de Casos de Prueba

### CASO 1: Base Simple (Solo Corte)
| Campo | Valor |
|-------|-------|
| Input | SVG rectangulo rojo, CO2, MDF 3mm, Vectorial, Qty=1 |
| cut_length_mm | 220 |
| time_cut_mins | 0.1 |
| Velocidad efectiva | 2200 mm/min |
| Velocidad matriz | 2200 mm/min |
| price_final | 32.49 CRC |
| **Resultado** | **PASS** - Velocidad de matriz correcta |

### CASO 2: Grabado Raster
| Campo | Valor |
|-------|-------|
| Input | SVG relleno negro, CO2, MDF 3mm, Rasterizado, Qty=1 |
| raster_area_mm2 | 900 |
| time_engrave_mins | 0.45 |
| Velocidad efectiva | 2000 mm²/min |
| Calculo | 4000 * 0.5 (speed_mult) = 2000 |
| factor_engrave | 1.5 |
| price_final | 121.92 CRC |
| **Resultado** | **PASS** - Factor grabado aplicado |

### CASO 3: Mixto Corte + Grabado
| Campo | Valor |
|-------|-------|
| Input | SVG mixto, CO2, Acrilico 5mm, Vectorial, Qty=1 |
| cut=240mm, vector=60mm, raster=400mm² | - |
| time_engrave_mins | 0.1533 |
| time_cut_mins | 0.2 |
| Velocidad corte acrilico 5mm | 1200 mm/min |
| factor_material | 1.2 |
| price_final | 120.43 CRC |
| **Resultado** | **PASS** - Tiempos calculados correctamente |

### CASO 4: Fallback (Grosor sin datos)
| Campo | Valor |
|-------|-------|
| Input | CO2, MDF 7mm (NO existe en matriz) |
| error | null (sin error) |
| time_cut_mins | 11.0 |
| Velocidad efectiva | 20 mm/min |
| Velocidad esperada | base_cut_speed = 20 mm/min |
| price_final | 2085.93 CRC |
| **Resultado** | **PASS** - Fallback funciona |

### CASO 5: Descuento Volumen
| Campo | Valor |
|-------|-------|
| Input | Mismo caso 1 con Qty=25 |
| discount_pct | 0.10 (10%) |
| price_unit | 32.49 CRC |
| price_total | 731.03 CRC |
| Calculo | 32.49 * 25 * 0.9 = 731.025 |
| **Resultado** | **PASS** - Descuento aplicado correctamente |

### CASO 6: Premium UV
| Campo | Valor |
|-------|-------|
| Input | UV, Vidrio, Vectorial, Qty=1 |
| uv_premium | 0.20 (20%) |
| price_final | 4230.63 CRC |
| **Resultado** | **PASS** - Premium UV aplicado |
| **NOTA** | Precio muy alto porque UV+Vidrio usa fallback de velocidad |

### CASO 7: Material Incluido vs No Incluido
| Campo | Con Material | Sin Material |
|-------|--------------|--------------|
| material_included (response) | null | null |
| material_cost.included | true | true |
| material_cost.with_waste | 9.66 | 0 |
| price_final | 32.49 | 18.96 |
| Diferencia | 13.53 CRC | - |
| **Resultado** | **PARTIAL PASS** |

**Problemas encontrados:**
1. `material_included` en response root es `null` (falta en ToDetailedJSON)
2. `material_cost.included` siempre muestra `true` (bug de persistencia)
3. La diferencia de precio (13.53) no es igual al costo material (9.66) porque se aplica margen

### CASO 8: Complejidad Alta
| Campo | Valor |
|-------|-------|
| Input | SVG complejo (11 elementos, 4179mm corte) |
| status | rejected |
| time_total | 6.9 min |
| price_final | 468.43 CRC |
| **Resultado** | **PASS** - Clasificacion de complejidad funciona |

---

## 3. Lista de Inconsistencias Encontradas

### 3.1 Frontend vs API

| Item | Frontend Lee | API Devuelve | Estado |
|------|--------------|--------------|--------|
| material_included | `q.material_included` | `null` (no existe a nivel root) | **INCONSISTENTE** |
| material_cost | `materialCost.with_waste` | `material_cost.with_waste` | OK |
| time_breakdown | `time.engrave_mins` | `time_breakdown.engrave_mins` | OK |
| factors | `factors.material` | `factors.material` | OK |
| pricing | `pricing.final` | `pricing.final` | OK |

### 3.2 Codigo vs Base de Datos

| Item | Codigo | BD | Estado |
|------|--------|-----|--------|
| setup_fee | Usa TechRate.SetupFee | Todos = 0.00 | **VALOR CERO** |
| material_included | Se asigna correctamente | Siempre `true` | **BUG PERSISTENCIA** |

### 3.3 Documentacion vs Realidad

| Item | Documentacion/Test | Realidad | Estado |
|------|-------------------|----------|--------|
| Velocidad MDF 3mm | 1800 mm/min | 2200 mm/min | **DISCREPANCIA** |

---

## 4. Lista de Bugs Encontrados

### BUG-1: material_included no se persiste en BD (CRITICO)
**Ubicacion:** `internal/models/quote.go` o persistencia GORM
**Sintoma:** Aunque se envie `material_included: false`, la BD siempre guarda `true`
**Causa probable:** GORM omite campos booleanos con valor `false` en INSERT porque Go trata `false` como zero value. La BD tiene `DEFAULT true`.
**Impacto:** El toggle "Material incluido" no funciona correctamente al revisar quotes existentes
**Solucion sugerida:** Cambiar `MaterialIncluded bool` a `MaterialIncluded *bool` en el modelo Quote

### BUG-2: material_included falta en response JSON a nivel root
**Ubicacion:** `internal/models/quote.go` funcion `ToDetailedJSON`
**Sintoma:** `q.material_included` es `null` en la respuesta
**Causa:** ToDetailedJSON no incluye el campo a nivel root, solo dentro de `material_cost.included`
**Impacto:** Frontend lee campo incorrecto
**Solucion sugerida:** Agregar `"material_included": q.MaterialIncluded` a ToDetailedJSON o cambiar frontend para leer `q.material_cost.included`

### BUG-3: Setup fee siempre es 0
**Ubicacion:** Datos en tabla `tech_rates`
**Sintoma:** Todas las tecnologias tienen `setup_fee = 0.00`
**Impacto:** No se cobra configuracion inicial
**Solucion sugerida:** Decidir si esto es intencional o configurar valores reales

---

## 5. Verificacion de Consistencia (Checklist)

| Item | Estado | Notas |
|------|--------|-------|
| Campos JSON API coinciden con frontend | **PARCIAL** | `material_included` falta a nivel root |
| Frontend muestra costo material separado | **SI** | Linea "Costo material" en breakdown |
| Toggle "Material incluido" funciona | **PARCIAL** | Funciona en calculo, NO persiste |
| Velocidades de matriz se leen correctamente | **SI** | Caso 1-3 prueban esto |
| Fallback a system_config funciona | **SI** | Caso 4 prueba esto |
| Factores se aplican en orden correcto | **SI** | margen → grabado → UV |
| Descuento volumen despues de factores | **SI** | Caso 5 prueba esto |
| Setup se cobra UNA vez | **SI** | Aunque es 0 actualmente |
| Costo material se multiplica por cantidad | **NO** | Es por unidad, no compartido |
| Merma se aplica al area del material | **SI** | waste_pct = 0.15 (15%) |

---

## 6. Valores Sospechosos o Irreales

### 6.1 Precios extremos con fallback
Cuando no existe combinacion en matriz (ej: UV+Vidrio para corte), se usa `base_cut_speed = 20 mm/min`. Esto genera precios muy altos porque:
- 220mm / 20mm/min = 11 minutos de corte
- vs 220mm / 2200mm/min = 0.1 minutos con matriz

**Recomendacion:** Mostrar advertencia cuando se usa fallback.

### 6.2 Setup fee = 0
Todas las tecnologias tienen setup_fee = 0. Esto puede ser intencional pero parece irreal para un taller de corte laser.

### 6.3 Costo material por unidad
El costo de material se calcula por unidad (area del SVG), no por hoja de material. Si un cliente pide 10 piezas que caben en una hoja, paga 10x el costo de material. Esto puede ser correcto (desperdicio por pieza) o incorrecto segun el modelo de negocio.

---

## 7. Recomendaciones de Ajuste

### Prioridad Alta

1. **Corregir BUG-1:** Cambiar `MaterialIncluded bool` a `MaterialIncluded *bool` para que GORM persista valores `false`

2. **Corregir BUG-2:** Sincronizar frontend y API para campo `material_included`:
   - Opcion A: Agregar campo a nivel root en ToDetailedJSON
   - Opcion B: Cambiar frontend para leer `material_cost.included`

### Prioridad Media

3. **Configurar setup_fee:** Decidir y configurar valores reales de setup por tecnologia

4. **Advertencia de fallback:** Mostrar indicador cuando el precio usa velocidades de fallback

### Prioridad Baja

5. **Documentar velocidades correctas:** La matriz tiene MDF 3mm = 2200 mm/min, no 1800 mm/min como dice el test original

6. **Revisar modelo de material:** Evaluar si el costo de material debe ser por pieza o por hoja

---

## 8. Resumen Ejecutivo

| Metrica | Valor |
|---------|-------|
| Casos de prueba ejecutados | 8 |
| Casos PASS | 6 |
| Casos PARTIAL | 1 (Caso 7) |
| Casos FAIL | 0 |
| Bugs criticos encontrados | 1 (BUG-1) |
| Bugs menores encontrados | 2 (BUG-2, BUG-3) |
| Inconsistencias | 2 |

**Conclusion:** El motor de cotizacion funciona correctamente en sus calculos principales. La formula se aplica segun lo documentado. Los bugs encontrados son de integracion (persistencia de booleanos, serializacion JSON) y no afectan la logica de calculo. Se recomienda corregir BUG-1 y BUG-2 antes de uso en produccion para que el toggle de material funcione correctamente.
