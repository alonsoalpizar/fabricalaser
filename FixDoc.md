# fixes-docs.md — Correcciones a /docs/pricing

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## NO usar subagentes. Secuencial.

Buscar el archivo HTML de la documentación:
```bash
find . -path "*/docs/pricing*" -o -name "pricing.html" | head -10
```

---

## Corrección 1: Fórmula principal INCORRECTA

La fórmula dice:
```
Precio Final = MAX(Hybrid, Value) + Setup Fee
```

Pero el Setup Fee ya está DENTRO de ambos modelos. La fórmula correcta es:
```
Hybrid = costos × factores + Setup Fee
Value  = valor × factores + Setup Fee
Precio Final = MAX(Hybrid, Value)
```

Corregir el bloque de "Formula Principal" a:
```
Precio Final = MAX(Hybrid, Value)
// Setup Fee ya incluido en ambos modelos
```

---

## Corrección 2: Valores de system_config desactualizados

En la tabla "Configuración General (system_config)", corregir:

| Clave | Valor actual (incorrecto) | Valor correcto |
|---|---|---|
| `base_engrave_line_speed` | 1000 mm/min | 2500 mm/min |
| `base_cut_speed` | 500 mm/min | Verificar en BD el valor real |
| `overhead_alquiler` | ₡50,000 | ₡51,500 |
| `overhead_internet` | ₡11,800 | ₡10,300 |

Agregar fila nueva:
```
| `price_per_mm_cut` | Tarifa por mm lineal para Value-Based en solo corte | ₡0.25/mm |
```

---

## Corrección 3: Aclarar Speed Multiplier vs Factor Grabado

Estos son dos conceptos diferentes que se confunden fácilmente. En la sección "Estimación de Tiempo", cambiar la presentación de Speed multiplier a:

```
Speed multiplier (afecta TIEMPO de producción):
  Vectorial: 1.0 (velocidad normal)
  Rasterizado: 0.5 (mitad de velocidad → doble de tiempo)
  Fotograbado: 0.25 (cuarto de velocidad → 4x tiempo)
  
  Nota: Este factor ya hace que trabajos rasterizados y fotograbado
  tomen más tiempo y por tanto cuesten más en el modelo Hybrid.
```

Y en la sección "Factores y Ajustes", agregar nota al Factor Grabado:

```
Factor Grabado (ajuste adicional al PRECIO):
  Vectorial: 1.0x
  Rasterizado: 1.5x  
  Fotograbado: 2.5x

  Nota: Este factor es adicional al speed multiplier. Un trabajo
  rasterizado ya tarda el doble (speed_mult=0.5) y además se 
  multiplica ×1.5 al precio por la complejidad del resultado.
```

---

## Corrección 4: Value-Based adaptativo (perímetro vs área)

En la sección "Modelo Value-Based", reemplazar el bloque de código con:

```
// El Value-Based se adapta al tipo de trabajo:

// CON GRABADO (raster o vector): cobra por área
Area Total = MAX(area_bounding_box x cantidad, area_minima)
Valor Base = MAX(min_value_base, Area Total x precio_por_mm2)

// SOLO CORTE (sin grabado): cobra por perímetro
Perimetro Total = longitud_corte x cantidad
Valor Base = MAX(min_value_base, Perimetro Total x price_per_mm_cut)

// En ambos casos:
Precio Value = Valor Base
             x Factor Material
             x Factor Grabado
             x (1 + Premium UV%)
             x (1 - Descuento%)
             + Setup Fee
```

Agregar nota explicativa:

```
¿Por qué perímetro para solo corte?

Cuando un trabajo es solo corte, la complejidad está en el perímetro, 
no en el área encerrada. Un círculo de 50mm tiene 2,500mm² de área 
pero solo 157mm de corte. Cobrar por área inflaría el precio 
innecesariamente.

El precio por perímetro (₡0.25/mm) se deriva de la tarifa de mercado 
de ₡500/min dividido por la velocidad promedio de corte (~2,000 mm/min).
```

---

## Corrección 5: Actualizar diagrama de flujo

El diagrama actual dice:
```
├ Value: area x tarifa
```

Reemplazar con:
```
├ Value: área o perímetro × tarifa
```

---

## Corrección 6: Ejemplo de geometría escalada

El ejemplo dice:
```
Unitario baja de ~₡7,000 (qty=1) a ~₡1,623 (qty=10)
```

Recalcular con valores actuales del sistema. Si los valores ya no coinciden, 
usar esta nota en su lugar:

```
Ejemplo: 10 círculos 50mm, MDF 3mm, CO2, Vectorial

Geometría escalada: corte = 1,570mm, área = 25,000mm²
Tiempo: 1,570 / 2,200 = 0.71 min + 5 min setup = 5.71 min
Setup: ₡4,000 (una vez, no ₡40,000)

El precio unitario disminuye significativamente con la cantidad,
porque el setup se diluye y el descuento por volumen aplica.
Los valores exactos dependen de las tarifas configuradas.
```

---

## Corrección 7: Ejemplo de Costos de Máquina

Agregar nota debajo del ejemplo de CO2:

```
Nota: Los valores de este ejemplo son ilustrativos. Los valores reales 
se configuran desde el panel de administración y se reflejan 
automáticamente en los campos calculados de cada tecnología.
```

---

## Verificación

```bash
# 1. Fórmula principal NO dice "+ Setup Fee" al final
grep -n "MAX.*Setup" [archivo_docs]

# 2. Valores corregidos
grep -n "1000 mm/min\|50,000\|11,800" [archivo_docs]
# Debe dar 0 resultados

# 3. price_per_mm_cut aparece
grep -n "price_per_mm_cut" [archivo_docs]

# 4. Perímetro mencionado en Value-Based
grep -n "perimetro\|perímetro\|SOLO CORTE" [archivo_docs]

# 5. Speed multiplier y Factor Grabado tienen notas aclaratorias
grep -n "adicional\|speed_mult" [archivo_docs]
```