# fixes-v5b.md — Overhead y Cost/Min como Campos Calculados en Admin

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## Problema

El formulario "Editar Tarifa" muestra "Overhead (₡/hora)" como campo editable con valor fijo (1947). Ahora que los costos fijos se manejan como items individuales (electricidad, mantenimiento, etc.), el overhead debe ser calculado automáticamente y mostrarse como solo lectura.

## Cambios Requeridos

### En el formulario de Editar Tarifa (admin):

**Campo: Overhead (₡/hora)**
- Cambiar label a: "Costos Fijos por Hora (₡)"
- Hacerlo **solo lectura** (readonly, fondo gris)
- Valor = (electricidad_mes + mantenimiento_mes + depreciacion_mes + seguro_mes + consumibles_mes) / horas_trabajo_mes + overhead_global_hora
- Agregar hint: "Calculado automáticamente desde costos fijos globales + costos de esta máquina"
- Recalcular en tiempo real cuando se editen los campos de costos por máquina

**Campos nuevos a mostrar como calculados (solo lectura):**

```
Costo por Minuto Grabado (₡) = (tarifa_grabado + costos_fijos_hora) / 60
Costo por Minuto Corte (₡)   = (tarifa_corte + costos_fijos_hora) / 60
```

- Solo lectura, fondo gris
- Hint: "Calculado: (Tarifa Operador + Costos Fijos) ÷ 60"
- Se recalculan en tiempo real cuando cambian tarifa_grabado, tarifa_corte, o cualquier costo fijo

### Implementación Frontend (JavaScript en el formulario admin)

```javascript
// Recalcular campos derivados en tiempo real
function recalcularCostos() {
  // Leer costos por máquina
  const electricidad = parseFloat(document.getElementById('electricidad_mes').value) || 0;
  const mantenimiento = parseFloat(document.getElementById('mantenimiento_mes').value) || 0;
  const depreciacion = parseFloat(document.getElementById('depreciacion_mes').value) || 0;
  const seguro = parseFloat(document.getElementById('seguro_mes').value) || 0;
  const consumibles = parseFloat(document.getElementById('consumibles_mes').value) || 0;

  // Overhead global viene del backend (inyectado como variable)
  // Es la suma de system_config overhead_alquiler + overhead_internet / horas_mes
  const overheadGlobalHora = parseFloat('{{.OverheadGlobalHora}}') || 0;
  const horasMes = parseFloat('{{.HorasTrabajoMes}}') || 120;

  // Overhead máquina por hora
  const totalMaquinaMes = electricidad + mantenimiento + depreciacion + seguro + consumibles;
  const overheadMaquinaHora = totalMaquinaMes / horasMes;

  // Total costos fijos por hora
  const costosFijosHora = overheadGlobalHora + overheadMaquinaHora;

  // Actualizar campo Costos Fijos por Hora
  document.getElementById('overhead_rate_hour').value = costosFijosHora.toFixed(2);

  // Leer tarifas operador
  const tarifaGrabado = parseFloat(document.getElementById('engrave_rate_hour').value) || 0;
  const tarifaCorte = parseFloat(document.getElementById('cut_rate_hour').value) || 0;

  // Calcular cost per minute
  const costPerMinEngrave = (tarifaGrabado + costosFijosHora) / 60;
  const costPerMinCut = (tarifaCorte + costosFijosHora) / 60;

  // Actualizar campos calculados
  document.getElementById('cost_per_min_engrave').value = costPerMinEngrave.toFixed(2);
  document.getElementById('cost_per_min_cut').value = costPerMinCut.toFixed(2);
}

// Escuchar cambios en TODOS los campos que afectan el cálculo
const camposQueAfectan = [
  'electricidad_mes', 'mantenimiento_mes', 'depreciacion_mes',
  'seguro_mes', 'consumibles_mes',
  'engrave_rate_hour', 'cut_rate_hour'
];

camposQueAfectan.forEach(id => {
  const el = document.getElementById(id);
  if (el) {
    el.addEventListener('input', recalcularCostos);
  }
});

// Calcular al cargar
recalcularCostos();
```

### Estilos para campos calculados

```css
.field-calculated {
  background: var(--bg-warm) !important;
  color: var(--text-muted) !important;
  cursor: not-allowed;
  border-style: dashed !important;
}

.field-calculated-hint {
  font-size: 0.6875rem;
  color: var(--text-muted);
  margin-top: 0.25rem;
  font-style: italic;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.field-calculated-hint::before {
  content: '⚡';
}
```

### HTML de los campos calculados

```html
<!-- Costos Fijos por Hora (calculado) -->
<div class="form-group">
  <label>Costos Fijos por Hora (₡)</label>
  <input type="number" id="overhead_rate_hour" class="field-calculated" readonly
         value="0" step="0.01">
  <span class="field-calculated-hint">
    Calculado: costos globales + costos de esta máquina
  </span>
</div>

<!-- Cost per min engrave (calculado) -->
<div class="form-group">
  <label>Costo por Minuto Grabado (₡)</label>
  <input type="number" id="cost_per_min_engrave" class="field-calculated" readonly
         value="0" step="0.01">
  <span class="field-calculated-hint">
    Calculado: (Tarifa Operador Grabado + Costos Fijos) ÷ 60
  </span>
</div>

<!-- Cost per min cut (calculado) -->
<div class="form-group">
  <label>Costo por Minuto Corte (₡)</label>
  <input type="number" id="cost_per_min_cut" class="field-calculated" readonly
         value="0" step="0.01">
  <span class="field-calculated-hint">
    Calculado: (Tarifa Operador Corte + Costos Fijos) ÷ 60
  </span>
</div>
```

### Backend: Inyectar overhead global al template

El handler que renderiza el formulario de tech_rates debe calcular y pasar el overhead global:

```go
// En el handler de editar tarifa
overheadAlquiler := config.GetSystemConfigFloat("overhead_alquiler", 0)
overheadInternet := config.GetSystemConfigFloat("overhead_internet", 0)
horasMes := config.GetSystemConfigFloat("horas_trabajo_mes", 120)

overheadGlobalHora := (overheadAlquiler + overheadInternet) / horasMes

data := map[string]interface{}{
    "TechRate":           techRate,
    "OverheadGlobalHora": overheadGlobalHora,
    "HorasTrabajoMes":    horasMes,
    // ... otros datos
}
```

### Renombrar labels existentes

| Campo | Label actual | Label nuevo |
|---|---|---|
| engrave_rate_hour | Tarifa Grabado (₡/hora) | Tarifa Operador Grabado (₡/hora) |
| cut_rate_hour | Tarifa Corte (₡/hora) | Tarifa Operador Corte (₡/hora) |
| design_rate_hour | Tarifa Diseño (₡/hora) | Tarifa Operador Diseño (₡/hora) |
| overhead_rate_hour | Overhead (₡/hora) | Costos Fijos por Hora (₡) |

Agregar hint debajo de Tarifa Diseño:
```html
<span class="field-calculated-hint" style="font-style: normal;">
  ℹ️ Informativo — no afecta el cálculo actual
</span>
```

---

## Verificación

```bash
# 1. Verificar que overhead_rate_hour tiene readonly en el template
grep -n "overhead_rate_hour.*readonly\|readonly.*overhead" templates/admin/

# 2. Verificar que el handler pasa OverheadGlobalHora
grep -n "OverheadGlobalHora" internal/handlers/admin/

# 3. Verificar labels renombrados
grep -n "Tarifa Operador\|Costos Fijos por Hora" templates/admin/

# 4. Verificar recalcularCostos existe
grep -n "recalcularCostos" templates/admin/
```

## Resultado Visual Esperado

El formulario de Editar Tarifa se verá:

```
Tecnología: [Láser UV ▼]

Tarifa Operador Grabado (₡/hora) *    Tarifa Operador Corte (₡/hora) *
[  6180  ]                              [  7210  ]

Tarifa Operador Diseño (₡/hora)        
[  7725  ]
ℹ️ Informativo — no afecta el cálculo actual

--- Costos Fijos de Esta Máquina (₡/mes) ---
Electricidad     Mantenimiento     Depreciación
[ 15450 ]        [ 25750 ]         [ 96990 ]

Seguro           Consumibles
[ 12875 ]        [ 20600 ]

--- Valores Calculados ---
Costos Fijos por Hora (₡)              
[ 1945.54 ] (readonly, fondo gris)
⚡ Calculado: costos globales + costos de esta máquina

Costo por Min Grabado (₡)    Costo por Min Corte (₡)
[ 135.43 ] (readonly)        [ 152.59 ] (readonly)
⚡ (Tarifa + Costos Fijos) ÷ 60

Setup Fee (₡)               Margen (%)
[  4000  ]                  [  40  ]

Estado: [Activo ▼]

                        [Cancelar] [Guardar]
```