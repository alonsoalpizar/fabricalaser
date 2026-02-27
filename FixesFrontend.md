# fixes-frontend.md — Cambios Frontend Cotizador Público

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## Contexto

Después de corregir el motor de cálculo (fixes.md, v2, v3, v5), el API devuelve campos nuevos que el cotizador público debe mostrar. Los cambios del admin ya fueron implementados en fixes-v5b.md.

## Archivo a Modificar

```
Cotizador público: el HTML que contiene el wizard de cotización (Step 3 - Resultado)
```

Buscar con:
```bash
grep -rl "resultPrice\|displayResult" templates/ static/ public/ web/
```

---

## Cambio 1: Mostrar modelo de precio (price_model)

El API devuelve `pricing.model` = "hybrid" o "value". El cliente debe saber qué modelo determinó su precio.

**En el HTML, agregar DEBAJO del div `result-price-label`:**

```html
<!-- Modelo de precio usado -->
<div class="result-price-model" id="resultPriceModel" style="display: none;">
  <span class="price-model-badge" id="priceModelBadge"></span>
</div>
```

**CSS a agregar:**

```css
/* PRICE MODEL INDICATOR */
.result-price-model {
  margin-top: 0.5rem;
}

.price-model-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  padding: 0.375rem 0.75rem;
  border-radius: 6px;
  font-size: 0.75rem;
  font-weight: 500;
}

.price-model-badge.hybrid {
  background: var(--info-bg);
  color: var(--info);
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.price-model-badge.value {
  background: rgba(139, 92, 246, 0.1);
  color: #7c3aed;
  border: 1px solid rgba(139, 92, 246, 0.2);
}
```

**JS en `displayResult()`, agregar después de `resultValidity`:**

```javascript
// Price model indicator
const priceModelEl = document.getElementById('resultPriceModel');
const priceModelBadge = document.getElementById('priceModelBadge');
const model = pricing.model || pricing.price_model;
if (model) {
  const modelLabels = {
    'hybrid': { text: 'Precio basado en tiempo de producción', class: 'hybrid' },
    'value':  { text: 'Precio basado en valor del servicio', class: 'value' }
  };
  const modelInfo = modelLabels[model] || { text: model, class: 'hybrid' };
  priceModelBadge.textContent = modelInfo.text;
  priceModelBadge.className = `price-model-badge ${modelInfo.class}`;
  priceModelEl.style.display = 'block';
} else {
  priceModelEl.style.display = 'none';
}
```

---

## Cambio 2: Mostrar fallback warning

Si el cálculo usó velocidades de fallback (no calibradas), el API devuelve `used_fallback_speeds: true` y `fallback_warning: "..."`.

**En el HTML, agregar DEBAJO del div `selectedSummary`:**

```html
<!-- Fallback warning -->
<div class="fallback-warning" id="fallbackWarning" style="display: none;">
  <div class="fallback-warning-icon">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/>
      <line x1="12" y1="9" x2="12" y2="13"/>
      <line x1="12" y1="17" x2="12.01" y2="17"/>
    </svg>
  </div>
  <div class="fallback-warning-text">
    <strong>Precio estimado</strong>
    <span id="fallbackWarningText">Este precio usa velocidades aproximadas. El precio final puede variar.</span>
  </div>
</div>
```

**CSS a agregar:**

```css
/* FALLBACK WARNING */
.fallback-warning {
  margin-top: 1rem;
  padding: 0.875rem 1rem;
  background: var(--warning-bg);
  border: 1px solid rgba(234, 179, 8, 0.3);
  border-radius: 8px;
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
}

.fallback-warning-icon {
  color: var(--warning);
  flex-shrink: 0;
  margin-top: 0.125rem;
}

.fallback-warning-text {
  font-size: 0.8125rem;
  color: var(--text-secondary);
  line-height: 1.5;
}

.fallback-warning-text strong {
  display: block;
  color: var(--warning);
  font-size: 0.875rem;
  margin-bottom: 0.125rem;
}
```

**JS en `displayResult()`, agregar al final:**

```javascript
// Fallback warning
const fallbackEl = document.getElementById('fallbackWarning');
const usedFallback = q.used_fallback_speeds || q.usedFallbackSpeeds || false;
if (usedFallback) {
  const warningText = q.fallback_warning || 'Este precio usa velocidades aproximadas. El precio final puede variar. Contactenos para un precio exacto.';
  document.getElementById('fallbackWarningText').textContent = warningText;
  fallbackEl.style.display = 'flex';
} else {
  fallbackEl.style.display = 'none';
}
```

---

## Cambio 3: Corregir lectura de material_included (BUG-2)

El frontend lee `q.material_included` a nivel root, pero el API podría devolverlo dentro de `material_cost.included`. Leer con fallback robusto.

**En `displayResult()`, reemplazar la sección de material cost:**

BUSCAR:
```javascript
  // Material cost
  const materialCostRow = document.getElementById('materialCostRow');
  const costMaterialEl = document.getElementById('costMaterial');
  if (q.material_included && materialCost.with_waste > 0) {
```

REEMPLAZAR CON:
```javascript
  // Material cost - leer con fallback por compatibilidad
  const materialIncluded = q.material_included !== undefined ? q.material_included : (materialCost.included !== undefined ? materialCost.included : true);
  const materialCostRow = document.getElementById('materialCostRow');
  const costMaterialEl = document.getElementById('costMaterial');
  if (materialIncluded && materialCost.with_waste > 0) {
```

TAMBIÉN BUSCAR:
```javascript
  } else if (!q.material_included) {
```

REEMPLAZAR CON:
```javascript
  } else if (!materialIncluded) {
```

---

## Cambio 4: Mostrar setup fee y precio unitario cuando qty > 1

**En el HTML, en el breakdown de precio, ANTES del div highlight de "Precio unitario":**

```html
<div class="breakdown-row" id="setupFeeRow" style="display: none;">
  <span class="breakdown-row-label">Setup (cargo unico)</span>
  <span class="breakdown-row-value" id="costSetup">₡0</span>
</div>
```

**JS en `displayResult()`, agregar:**

```javascript
// Setup fee row
const setupFeeRow = document.getElementById('setupFeeRow');
const setupFee = pricing.setup_fee || pricing.cost_setup || 0;
if (setupFee > 0) {
  document.getElementById('costSetup').textContent = `₡${Math.round(setupFee).toLocaleString('es-CR')}`;
  setupFeeRow.style.display = 'flex';
} else {
  setupFeeRow.style.display = 'none';
}

// Update price label to show unit reference when qty > 1
const qty = q.quantity || 1;
const unitPrice = pricing.hybrid_unit || pricing.price_hybrid_unit || 0;
const priceLabelEl = document.getElementById('resultPriceLabel') || document.querySelector('.result-price-label');
if (qty > 1 && unitPrice > 0) {
  priceLabelEl.innerHTML = `${qty} unidades - Precio total <span style="color: var(--text-muted); font-size: 0.8125rem;">(₡${Math.round(unitPrice).toLocaleString('es-CR')} c/u)</span>`;
} else {
  priceLabelEl.textContent = `${qty} unidad(es) - Precio total`;
}
```

---

## Cambio 5: Limpiar elementos nuevos en resetWizard()

**En `resetWizard()`, agregar:**

```javascript
// Reset new indicators
document.getElementById('fallbackWarning').style.display = 'none';
document.getElementById('resultPriceModel').style.display = 'none';
document.getElementById('setupFeeRow').style.display = 'none';
```

---

## Verificación

```bash
# 1. Nuevos elementos existen
grep -n "resultPriceModel\|fallbackWarning\|setupFeeRow\|priceModelBadge" [archivo_html]

# 2. materialIncluded con fallback
grep -n "materialIncluded" [archivo_html]

# 3. resetWizard limpia elementos nuevos
grep -A3 "Reset new indicators" [archivo_html]
```

## Consistencia visual

Todos los nuevos elementos usan:
- Variables CSS existentes (--warning, --info, --text-muted, etc.)
- Font DM Sans heredado del body
- Border-radius 6-8px consistente con el diseño
- Tamaños de fuente existentes (0.75rem, 0.8125rem, 0.875rem)
- Ningún color o fuente nueva — todo del sistema existente

## No tocar

- Lógica de cálculo (ya corregida en fixes backend)
- Flujo del wizard (steps 1, 2, 3)
- Estilos base existentes
- Panel admin (ya resuelto en fixes-v5b.md)
- Lógica de compatibility filtering
- Historial de cotizaciones