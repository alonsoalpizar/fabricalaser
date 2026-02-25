# Phase 1 Frontend: UI Cotizador - Plan en Micro-Fases

## Relación con APIs Backend

### APIs a Consumir
| API | Cuándo | Datos |
|-----|--------|-------|
| `GET /api/v1/config` | Al cargar página | technologies, materials, engrave_types |
| `GET /api/v1/auth/me` | Al cargar (si tiene token) | usuario, quote_quota, quotes_used |
| `POST /api/v1/quotes/analyze` | Paso 1: Upload SVG | analysis_id, geometría, warnings |
| `POST /api/v1/quotes/calculate` | Paso 2: Calcular | quote con precios y desglose |
| `GET /api/v1/quotes/my` | Historial | lista de cotizaciones previas |

### Headers Requeridos
```javascript
// Para endpoints protegidos
headers: {
  'Authorization': `Bearer ${localStorage.getItem('fabricalaser_token')}`,
  'Content-Type': 'application/json'  // o multipart/form-data para upload
}
```

### Flujo de Datos
```
1. Usuario llega a /cotizar
   ├─ Sin token → Modal auth → Redirect aquí
   └─ Con token → Verificar cuota
       ├─ Cuota agotada → Mensaje "contacte admin"
       └─ Cuota OK → Mostrar wizard

2. Wizard Paso 1: Upload SVG
   ├─ POST /quotes/analyze (multipart/form-data)
   ├─ Respuesta: SVGAnalysis
   │   ├─ warnings[] → Mostrar alertas
   │   ├─ cut_length_mm, vector_length_mm, raster_area_mm2
   │   └─ status: "analyzed" → Continuar
   └─ Guardar analysis_id para paso 2

3. Wizard Paso 2: Seleccionar Opciones
   ├─ Cargar opciones de /api/v1/config (o cache)
   ├─ Usuario selecciona:
   │   ├─ technology_id (CO2, UV, Fibra, MOPA)
   │   ├─ material_id (Madera, Acrílico, etc.)
   │   ├─ engrave_type_id (Vectorial, Raster, etc.)
   │   ├─ quantity (1+)
   │   └─ thickness (opcional)
   └─ POST /quotes/calculate → Quote

4. Wizard Paso 3: Resultado
   ├─ Mostrar desglose de tiempos
   ├─ Mostrar desglose de costos
   ├─ Mostrar precio final
   ├─ Status: auto_approved|needs_review
   └─ Opción: "Nueva cotización" | "Ver historial"
```

---

## Micro-Fases de Implementación

### 1F.1: Estructura Base Cotizador
**Archivo:** `web/cotizar/index.html`

- HTML base con estructura wizard
- CSS para pasos y transiciones
- JS base para navegación entre pasos
- Verificación de auth al cargar
- Redirect a landing si no autenticado

**Dependencias:** Token en localStorage (de landing/auth)

### 1F.2: Paso 1 - Upload SVG
**Sección en:** `web/cotizar/index.html`

- Dropzone para arrastrar/soltar SVG
- Input file como fallback
- Validación cliente: .svg, max 5MB
- Preview del SVG (opcional)
- Llamada POST /quotes/analyze
- Mostrar loading durante upload
- Manejar errores (formato, tamaño)
- Mostrar warnings del análisis
- Mostrar resumen geometría:
  - Dimensiones (ancho × alto)
  - Longitud corte (rojo)
  - Longitud vector (azul)
  - Área raster (negro)
- Botón "Continuar" cuando análisis OK

### 1F.3: Paso 2 - Selección de Opciones
**Sección en:** `web/cotizar/index.html`

- Cargar config desde API (o cache en sessionStorage)
- Select: Tecnología (con descripción)
- Select: Material (con factor visible)
- Select: Tipo de grabado (con descripción)
- Input: Cantidad (número, min 1)
- Input: Espesor (opcional, dropdown con valores del material)
- Preview de descuento por volumen
- Botón "Calcular Precio"
- Llamada POST /quotes/calculate
- Loading mientras calcula

### 1F.4: Paso 3 - Resultado
**Sección en:** `web/cotizar/index.html`

- Desglose de tiempos:
  - Tiempo grabado: X min
  - Tiempo corte: X min
  - Setup: X min
  - Total: X min
- Desglose de factores aplicados:
  - Factor material: 1.2×
  - Factor grabado: 1.0×
  - Premium UV: +20%
  - Margen: 40%
  - Descuento volumen: -5%
- Precios:
  - Precio unitario: $X.XX
  - Precio total: $X.XX
- Status badge:
  - Verde "Aprobado" (auto_approved)
  - Amarillo "En revisión" (needs_review)
- Validez: "Válido hasta: fecha"
- Botones:
  - "Nueva cotización" → Reset wizard
  - "Ver mis cotizaciones" → /mi-cuenta o sección historial

### 1F.5: Historial de Cotizaciones
**Opción A:** Sección en `/mi-cuenta`
**Opción B:** Tab en `/cotizar`

- Llamada GET /quotes/my
- Lista con:
  - Fecha
  - Archivo SVG
  - Cantidad
  - Precio
  - Status
- Click para ver detalle
- Paginación si hay muchas

### 1F.6: UX Refinamientos
- Animaciones de transición entre pasos
- Feedback visual en cada acción
- Mensajes de error user-friendly
- Tooltips en opciones complicadas
- Mobile responsive
- Indicador de paso actual (1 de 3)
- Opción de volver al paso anterior

---

## Mapeo API → UI

### Config Options (GET /api/v1/config)
```javascript
// Response
{
  "technologies": [
    {"id": 1, "code": "CO2", "name": "Láser CO2", "description": "...", "uv_premium_factor": 0}
  ],
  "materials": [
    {"id": 1, "name": "Madera / MDF", "factor": 1.0, "thicknesses": [3,5,6,9,12,15,18]}
  ],
  "engrave_types": [
    {"id": 1, "name": "Vectorial", "factor": 1.0, "description": "..."}
  ],
  "volume_discounts": [
    {"min_qty": 10, "max_qty": 24, "discount_pct": 0.05}
  ]
}

// UI Mapping
technologies → Select "Tecnología"
materials → Select "Material" + Sub-select "Espesor"
engrave_types → Select "Tipo de grabado"
volume_discounts → Info de descuento según cantidad
```

### Analysis Response → Step 1 Display
```javascript
// Response
{
  "data": {
    "width_mm": 100,
    "height_mm": 50,
    "cut_length_mm": 245.5,
    "vector_length_mm": 120.3,
    "raster_area_mm2": 1500.0,
    "warnings": ["No width specified"]
  }
}

// UI Display
"Dimensiones: 100 × 50 mm"
"Corte (rojo): 245.5 mm"
"Vector (azul): 120.3 mm"
"Raster (negro): 1500 mm²"
⚠️ "No width specified"
```

### Quote Response → Step 3 Display
```javascript
// Response
{
  "data": {
    "time_breakdown": {...},
    "cost_breakdown": {...},
    "factors": {...},
    "pricing": {
      "hybrid_unit": 9.61,
      "hybrid_total": 91.29,
      "final": 91.29
    },
    "status": "auto_approved",
    "valid_until": "2026-03-04T..."
  }
}

// UI Display
"⏱️ Tiempo total: 25.7 min"
"💰 Precio unitario: $9.61"
"💰 Precio total (10 pcs): $91.29"
"✅ Cotización aprobada"
"📅 Válida hasta: 4 Mar 2026"
```

---

## Estado Global (JavaScript)
```javascript
const cotizadorState = {
  // Auth
  token: localStorage.getItem('fabricalaser_token'),
  user: null,

  // Config (cached)
  config: null,

  // Wizard state
  currentStep: 1,

  // Step 1 result
  analysis: null,

  // Step 2 selections
  options: {
    technologyId: null,
    materialId: null,
    engraveTypeId: null,
    quantity: 1,
    thickness: null
  },

  // Step 3 result
  quote: null
};
```

---

## Orden de Implementación
```
1F.1 Estructura Base ──► 1F.2 Upload SVG ──► 1F.3 Opciones ──► 1F.4 Resultado
                                                                     │
                                                                     ▼
                                                              1F.5 Historial
                                                                     │
                                                                     ▼
                                                              1F.6 UX Polish
```

---

## Criterios de Completitud
- [ ] Usuario puede subir SVG y ver análisis
- [ ] Usuario puede seleccionar opciones y ver precio
- [ ] Quote se guarda en DB y aparece en historial
- [ ] Mensajes de error claros para cada caso
- [ ] Mobile responsive funcional
- [ ] Flujo completo sin errores de consola
