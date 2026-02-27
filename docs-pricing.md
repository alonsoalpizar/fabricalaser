# docs-pricing.md — Manual del Motor de Precios FabricaLaser

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## Objetivo

Crear una documentación web accesible que explique el modelo completo de precios de FabricaLaser. Dirigida a:
- **Alonso (dueño):** Para entender, auditar y ajustar precios
- **Futuros operadores:** Para entender cómo se calcula una cotización
- **Soporte técnico:** Para diagnosticar precios que parezcan incorrectos

## Formato

Página HTML standalone, estilo documentación moderna (dark theme consistente con FabricaLaser). Puede ser una single-page con navegación lateral tipo sidebar con anchors, o múltiples páginas. Debe ser desplegable como ruta del mismo sitio (ej: `/docs/pricing`).

## NO usar subagentes. Secuencial.

---

## Estructura del Documento

### 1. Visión General
Explicar que FabricaLaser usa un **sistema de doble modelo** que protege tanto al negocio como al cliente:

```
Precio Final = MAX(Precio Hybrid, Precio Value-Based) + Setup Fee
```

- **Hybrid:** Basado en costos reales de producción (tiempo + material + margen)
- **Value-Based:** Basado en el valor de mercado del servicio (precio por mm²)
- El sistema siempre elige el mayor, garantizando que nunca se cobra por debajo del costo ni por debajo del valor de mercado

Incluir diagrama visual del flujo:
```
SVG Upload → Análisis → Geometría × Cantidad → 
  ├─ Hybrid: tiempo + material + margen
  ├─ Value: área × tarifa mercado
  └─ Final: MAX(Hybrid, Value) + Setup
```

### 2. Análisis del SVG
Explicar cómo el sistema interpreta un archivo SVG:

- **Corte (rojo):** Líneas con stroke rojo (#FF0000, #ff0000, red). Representan el perímetro a cortar. Se miden en milímetros lineales.
- **Grabado Vector (azul):** Líneas con stroke azul (#0000FF, #0000ff, blue). Representan líneas a grabar sin relleno. Se miden en milímetros lineales.
- **Grabado Raster (negro):** Áreas con fill negro (#000000, black). Representan superficies a grabar barriendo. Se miden en milímetros cuadrados.
- **Dimensiones:** Se usa el bounding box real de los elementos del SVG, NO el canvas. Un SVG de 300×300mm con un círculo de 50mm solo cuenta 50×50mm.

### 3. Modelo Hybrid (Costo de Producción)

#### 3a. Estimación de Tiempo
Fórmula por componente:

```
Tiempo Raster = área_raster_mm² / (velocidad_cabezal × spot_size × speed_mult)
Tiempo Vector = longitud_vector_mm / (velocidad_cabezal × speed_mult)
Tiempo Corte  = longitud_corte_mm / velocidad_corte
Tiempo Setup  = constante (5 minutos por defecto)
Tiempo Total  = Setup + Raster + Vector + Corte
```

Explicar:
- **Velocidad cabezal:** Viene de la matriz tech_material_speeds. Es la velocidad del cabezal láser en mm/min, calibrada por tecnología × material × grosor
- **Spot size:** Diámetro del haz láser en mm. Convierte velocidad lineal (mm/min) en velocidad de área (mm²/min). CO2=0.10mm, FIBRA=0.03mm, MOPA=0.04mm, UV=0.02mm
- **Speed multiplier:** Factor por tipo de grabado. Vectorial=1.0, Rasterizado=0.5 (más lento), Fotograbado=0.25 (mucho más lento)
- **Velocidades fallback:** Cuando no hay datos calibrados para una combinación específica, se usan velocidades conservadoras (más lentas = precio más alto). El sistema avisa con un warning

#### 3b. Costos de Máquina
```
Costo por minuto = (Tarifa Operador + Overhead Global + Overhead Máquina) / 60

Donde:
  Tarifa Operador = mano de obra por hora (varía: grabado vs corte)
  Overhead Global = (alquiler + internet) / horas_mes → compartido todas las máquinas
  Overhead Máquina = (electricidad + mantenimiento + depreciación + seguro + consumibles) / horas_mes → específico por tecnología
```

Ejemplo con CO2:
```
Tarifa Operador Grabado: ₡6,180/hora
Overhead Global: ₡515/hora (₡61,800/mes ÷ 120 horas)
Overhead Máquina CO2: ₡1,528/hora (₡183,365/mes ÷ 120 horas)
→ Costo por minuto grabado = (6180 + 515 + 1528) / 60 = ₡137.05/min
```

#### 3c. Costo de Material
```
Área Material = bounding_box_real × cantidad
Costo Base = área × costo_por_mm² del material
Merma = 15% adicional (desperdicio de corte)
Costo Material = Costo Base × 1.15
```

Si el cliente provee el material, Costo Material = ₡0.

#### 3d. Factores y Ajustes
```
Precio Hybrid = (Costo Máquina + Costo Material)
              × (1 + Margen%)        → 40% por defecto
              × Factor Grabado       → Vec=1.0, Raster=1.5, Foto=2.5
              × (1 + Premium UV%)    → 20% extra si es Láser UV
              × (1 - Descuento%)     → Por volumen
              + Setup Fee            → ₡4,000 (una vez)
```

#### 3e. Descuentos por Volumen
| Cantidad | Descuento |
|----------|-----------|
| 1-9      | 0%        |
| 10-24    | 5%        |
| 25-49    | 10%       |
| 50-99    | 15%       |
| 100+     | 20%       |

### 4. Modelo Value-Based (Valor de Mercado)

```
Área Total = MAX(área_bounding_box × cantidad, área_mínima)
Valor Base = MAX(min_value_base, Área Total × precio_por_mm²)

Precio Value = Valor Base
             × Factor Material    → MDF=1.0, Acrílico=1.2, Vidrio=1.5, Metal=1.8
             × Factor Grabado     → Vec=1.0, Raster=1.5, Foto=2.5
             × (1 + Premium UV%)
             × (1 - Descuento%)
             + Setup Fee
```

Explicar:
- **precio_por_mm² = ₡0.515:** Tarifa promedio de mercado por milímetro cuadrado, calculada como promedio ponderado entre todos los materiales considerando raster + material + margen
- **min_value_base = ₡3,000:** Precio mínimo absoluto antes de setup. Protege trabajos muy pequeños
- **Factor Material se aplica directo:** A diferencia del Hybrid donde el material ya afecta vía velocidad, aquí se multiplica explícitamente

### 5. Geometría Escalada por Cantidad

Explicar que el sistema NO calcula "precio × cantidad". En su lugar:

```
ANTES (incorrecto): calcular 1 pieza → multiplicar × qty
AHORA (correcto):   geometría × qty → calcular 1 vez

Diferencia clave:
- Setup se cobra UNA vez (no qty veces)
- Material se calcula sobre área total (no qty × área individual con qty mermas)
- Descuento aplica sobre el total
- Unitario = Total / qty (solo referencia)
```

Ejemplo: 10 círculos 50mm, MDF 3mm
```
Geometría escalada: corte = 1,570mm, área = 25,000mm²
Tiempo: 1,570 / 2,200 = 0.71 min + 5 min setup = 5.71 min
Setup: ₡4,000 (una vez, no ₡40,000)
→ Unitario baja de ~₡7,000 (qty=1) a ~₡1,623 (qty=10)
```

### 6. Compatibilidad Tecnología × Material

Tabla de compatibilidad:
| Material | CO2 | UV | FIBRA | MOPA |
|----------|-----|----|-------|------|
| MDF/Madera | ✅ Ideal | ⚠️ Posible | ❌ | ❌ |
| Acrílico | ✅ Ideal | ✅ Bueno | ❌ | ❌ |
| Cuero | ✅ Ideal | ⚠️ Posible | ❌ | ❌ |
| Vidrio | ⚠️ Posible | ✅ Ideal | ❌ | ✅ Bueno |
| Metal | ❌ | ❌ | ✅ Ideal | ✅ Ideal |
| ABS/Plástico | ✅ Bueno | ✅ Bueno | ❌ | ⚠️ Posible |
| Cerámica | ⚠️ Posible | ✅ Ideal | ❌ | ✅ Bueno |

El cotizador solo muestra tecnologías compatibles con el material seleccionado.

### 7. Configuración Administrable

Sección que explique qué valores se pueden cambiar desde el admin y dónde:

**Configuración General (system_config):**
- `price_per_mm2` — Tarifa por mm² para Value-Based
- `min_value_base` — Precio mínimo Value-Based
- `base_engrave_line_speed` — Velocidad fallback grabado
- `base_cut_speed` — Velocidad fallback corte
- `waste_factor` — Factor de merma (15%)
- `min_area_mm2` — Área mínima para cobro
- `overhead_alquiler` — Alquiler mensual del taller (₡)
- `overhead_internet` — Internet mensual (₡)
- `horas_trabajo_mes` — Horas de trabajo estimadas/mes

**Por Tecnología (tech_rates):**
- Tarifa operador grabado/corte/diseño
- Costos fijos de la máquina (electricidad, mantenimiento, depreciación, seguro, consumibles)
- Setup fee
- Margen (%)
- Spot size

**Matriz de Velocidades (tech_material_speeds):**
- Velocidad de grabado y corte por combinación tecnología × material × grosor
- 37 combinaciones calibradas

**Descuentos (volume_discounts):**
- Rangos de cantidad y porcentaje de descuento

### 8. Diagnóstico de Precios

Guía para cuando un precio "parece raro":

**Precio muy bajo:**
- ¿El modelo es Hybrid? → Trabajo pequeño/rápido. El Value-Based debería proteger como piso
- ¿min_value_base está configurado? → Verificar que sea ≥ ₡3,000
- ¿Setup fee está en 0? → Verificar tech_rates

**Precio muy alto:**
- ¿Usó fallback? → Velocidades conservadoras. Calibrar esa combinación en la matriz
- ¿Canvas muy grande? → Verificar que el SVG tenga dimensiones reales (bounding box, no canvas inflado)
- ¿Factor grabado alto? → Fotograbado = 2.5x. Confirmar que el tipo de grabado es correcto

**Precio no cambia con cantidad:**
- ¿Descuento por volumen está configurado? → Verificar volume_discounts
- ¿Setup fee es muy alto relativo al trabajo? → Setup se diluye con cantidad

### 9. Glosario

- **Bounding box:** Rectángulo mínimo que contiene todos los elementos del SVG
- **Spot size:** Diámetro del punto del haz láser. Determina cuánta área cubre por pasada
- **Raster:** Grabado por barrido de área (como una impresora). Usa spot_size para calcular velocidad
- **Vector:** Grabado siguiendo líneas/contornos. Velocidad directa del cabezal
- **Fallback:** Velocidad genérica usada cuando no hay datos calibrados para una combinación específica
- **Merma:** Porcentaje de material desperdiciado por el proceso de corte (15%)
- **Overhead:** Costos fijos del taller y la máquina prorrateados por hora de trabajo

---

## Estilo Visual

- Dark theme consistente con FabricaLaser (--bg-dark, --accent copper/terracotta)
- Fuente: DM Sans (la misma del cotizador)
- Sidebar fija con navegación por secciones
- Code blocks para fórmulas
- Tablas estilizadas para compatibilidad y descuentos
- Diagramas con SVG o Mermaid para flujos
- Responsive (funciona en móvil)
- Ruta sugerida: `/docs/pricing` o `/docs/motor-precios`

## Datos Reales a Usar

Todos los valores, fórmulas y ejemplos en este documento son los valores REALES del sistema en producción. No son placeholders — usar tal cual.