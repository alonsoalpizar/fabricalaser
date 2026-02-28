# fixes-ux.md — Mejoras UX Pantalla de Resultado (Step 3)

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## NO usar subagentes. Secuencial.

## Contexto

La pantalla de resultado del cotizador (Step 3) funciona correctamente pero tiene oportunidades de mejora en UX. Tres cambios específicos:

1. Detalle técnico expandible para factores internos
2. Preview del SVG con colores interpretados
3. Mejor mensaje de "Requiere revisión"

## Archivo a Modificar

El HTML del cotizador público que contiene el wizard Step 3.
```bash
grep -rl "resultPrice\|displayResult" templates/ static/ public/ web/
```

---

## Cambio 1: Detalle Técnico Expandible

### Problema
El desglose de costos muestra factores internos (Factor material 1.20x, Factor grabado 1.50x, Premium UV +0%) que no significan nada para el cliente y generan confusión o desconfianza.

### Solución
Mostrar un desglose simplificado por defecto. Los factores técnicos quedan ocultos detrás de un botón "Ver detalle técnico" que expande/colapsa.

### HTML

Reemplazar el contenido del breakdown-card de "Desglose de costos" con:

```html
<!-- Desglose de costos -->
<div class="breakdown-card">
  <div class="breakdown-title">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M12 2v20M17 5H9.5a3.5 3.5 0 000 7h5a3.5 3.5 0 010 7H6"/>
    </svg>
    Desglose de costos
  </div>
  
  <!-- Desglose simplificado (siempre visible) -->
  <div class="breakdown-row">
    <span class="breakdown-row-label">Producción (tiempo máquina)</span>
    <span class="breakdown-row-value" id="costMachine">₡0</span>
  </div>
  <div class="breakdown-row material-cost-row" id="materialCostRow">
    <span class="breakdown-row-label">Material</span>
    <span class="breakdown-row-value" id="costMaterial">₡0</span>
  </div>
  <div class="breakdown-row" id="setupFeeRow" style="display: none;">
    <span class="breakdown-row-label">Preparación de máquina</span>
    <span class="breakdown-row-value" id="costSetup">₡0</span>
  </div>
  <div class="breakdown-row" id="discountRow" style="display: none;">
    <span class="breakdown-row-label">Descuento volumen</span>
    <span class="breakdown-row-value discount-value" id="factorDiscount">-0%</span>
  </div>
  <div class="breakdown-row highlight">
    <span class="breakdown-row-label">Precio unitario</span>
    <span class="breakdown-row-value" id="priceUnit">₡0</span>
  </div>

  <!-- Detalle técnico (colapsado por defecto) -->
  <button type="button" class="tech-detail-toggle" id="techDetailToggle" onclick="toggleTechDetail()">
    <svg class="tech-detail-chevron" id="techDetailChevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <polyline points="6 9 12 15 18 9"/>
    </svg>
    Detalle técnico
  </button>
  <div class="tech-detail-panel" id="techDetailPanel" style="display: none;">
    <div class="breakdown-row tech-row">
      <span class="breakdown-row-label">Factor material</span>
      <span class="breakdown-row-value" id="factorMaterial">1.0x</span>
    </div>
    <div class="breakdown-row tech-row">
      <span class="breakdown-row-label">Factor grabado</span>
      <span class="breakdown-row-value" id="factorEngrave">1.0x</span>
    </div>
    <div class="breakdown-row tech-row">
      <span class="breakdown-row-label">Premium UV</span>
      <span class="breakdown-row-value" id="factorUV">+0%</span>
    </div>
    <div class="breakdown-row tech-row">
      <span class="breakdown-row-label">Margen</span>
      <span class="breakdown-row-value" id="factorMargin">40%</span>
    </div>
    <div class="breakdown-row tech-row">
      <span class="breakdown-row-label">Modelo de precio</span>
      <span class="breakdown-row-value" id="techPriceModel">-</span>
    </div>
  </div>
</div>
```

### CSS

```css
/* TECH DETAIL EXPANDIBLE */
.tech-detail-toggle {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.375rem;
  padding: 0.625rem 0;
  margin-top: 0.5rem;
  background: none;
  border: none;
  border-top: 1px dashed var(--border);
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  font-family: inherit;
  transition: color 0.2s;
}

.tech-detail-toggle:hover {
  color: var(--text);
}

.tech-detail-chevron {
  transition: transform 0.3s ease;
}

.tech-detail-chevron.expanded {
  transform: rotate(180deg);
}

.tech-detail-panel {
  margin-top: 0.25rem;
  padding-top: 0.25rem;
  animation: slideDown 0.3s ease;
}

@keyframes slideDown {
  from { opacity: 0; max-height: 0; }
  to { opacity: 1; max-height: 200px; }
}

.tech-row {
  opacity: 0.7;
  font-size: 0.8125rem;
}

.tech-row .breakdown-row-label {
  font-size: 0.8125rem;
}

.tech-row .breakdown-row-value {
  font-size: 0.8125rem;
}

.discount-value {
  color: var(--success) !important;
}
```

### JavaScript

```javascript
function toggleTechDetail() {
  const panel = document.getElementById('techDetailPanel');
  const chevron = document.getElementById('techDetailChevron');
  
  if (panel.style.display === 'none') {
    panel.style.display = 'block';
    chevron.classList.add('expanded');
  } else {
    panel.style.display = 'none';
    chevron.classList.remove('expanded');
  }
}
```

En `displayResult()`, agregar la lógica para llenar el detalle técnico:

```javascript
// Tech detail
document.getElementById('factorMargin').textContent = ((pricing.factor_margin || pricing.margin_percent || 0.40) * 100).toFixed(0) + '%';
document.getElementById('techPriceModel').textContent = (pricing.model || pricing.price_model || '-');

// Show discount row only when discount > 0
const discountRow = document.getElementById('discountRow');
const discountPct = pricing.discount_volume_pct || pricing.discount_pct || 0;
if (discountPct > 0) {
  document.getElementById('factorDiscount').textContent = `-${(discountPct * 100).toFixed(0)}%`;
  discountRow.style.display = 'flex';
} else {
  discountRow.style.display = 'none';
}
```

En `resetWizard()`, agregar:
```javascript
document.getElementById('techDetailPanel').style.display = 'none';
document.getElementById('techDetailChevron').classList.remove('expanded');
```

---

## Cambio 2: Preview del SVG con Colores Interpretados

### Problema
El cliente sube un SVG y ve solo números. No tiene confirmación visual de que el sistema interpretó correctamente su diseño (qué es corte, qué es grabado, qué es raster).

### Solución
Mostrar una miniatura del SVG subido junto al resultado, con una leyenda de colores.

### HTML

Agregar DESPUÉS del div `selectedSummary` y ANTES del `fallbackWarning`:

```html
<!-- SVG Preview -->
<div class="svg-preview-card" id="svgPreviewCard" style="display: none;">
  <div class="svg-preview-header">
    <div class="breakdown-title">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="3" y="3" width="18" height="18" rx="2"/>
        <circle cx="8.5" cy="8.5" r="1.5"/>
        <polyline points="21 15 16 10 5 21"/>
      </svg>
      Tu diseño
    </div>
  </div>
  <div class="svg-preview-content">
    <div class="svg-preview-image" id="svgPreviewImage">
      <!-- SVG se renderiza aquí -->
    </div>
    <div class="svg-preview-legend">
      <div class="legend-item" id="legendCut" style="display: none;">
        <span class="legend-color" style="background: #FF0000;"></span>
        <span class="legend-label">Corte</span>
        <span class="legend-value" id="legendCutValue">-</span>
      </div>
      <div class="legend-item" id="legendVector" style="display: none;">
        <span class="legend-color" style="background: #0000FF;"></span>
        <span class="legend-label">Grabado vector</span>
        <span class="legend-value" id="legendVectorValue">-</span>
      </div>
      <div class="legend-item" id="legendRaster" style="display: none;">
        <span class="legend-color" style="background: #000000;"></span>
        <span class="legend-label">Grabado raster</span>
        <span class="legend-value" id="legendRasterValue">-</span>
      </div>
      <div class="legend-item">
        <span class="legend-color" style="background: transparent; border: 1px dashed var(--border);"></span>
        <span class="legend-label">Bounding box</span>
        <span class="legend-value" id="legendDimensions">-</span>
      </div>
    </div>
  </div>
</div>
```

### CSS

```css
/* SVG PREVIEW */
.svg-preview-card {
  margin-top: 1.5rem;
  background: var(--bg-warm);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1.25rem;
}

.svg-preview-header {
  margin-bottom: 1rem;
}

.svg-preview-content {
  display: flex;
  gap: 1.5rem;
  align-items: flex-start;
}

@media (max-width: 600px) {
  .svg-preview-content {
    flex-direction: column;
  }
}

.svg-preview-image {
  flex: 1;
  background: white;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1rem;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
  max-height: 200px;
  overflow: hidden;
}

.svg-preview-image svg {
  max-width: 100%;
  max-height: 180px;
  width: auto;
  height: auto;
}

.svg-preview-legend {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  min-width: 160px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
}

.legend-color {
  width: 12px;
  height: 12px;
  border-radius: 2px;
  flex-shrink: 0;
}

.legend-label {
  color: var(--text-muted);
  flex: 1;
}

.legend-value {
  font-weight: 500;
  color: var(--text);
  font-size: 0.75rem;
}
```

### JavaScript

La idea es reusar el SVG que el usuario ya subió. Al hacer el análisis en Step 1, guardar el contenido SVG en el state:

En la función que procesa el upload del SVG (donde se lee el archivo), agregar:
```javascript
// Guardar SVG raw para preview en resultado
const reader = new FileReader();
reader.onload = function(e) {
  state.svgContent = e.target.result;
};
reader.readAsText(file);
```

Si el SVG ya se lee como texto en algún punto del upload, simplemente guardar:
```javascript
state.svgContent = svgTextContent; // el texto raw del SVG
```

En `displayResult()`, agregar la lógica del preview:

```javascript
// SVG Preview
const svgPreviewCard = document.getElementById('svgPreviewCard');
const svgPreviewImage = document.getElementById('svgPreviewImage');

if (state.svgContent) {
  // Renderizar SVG en el preview
  svgPreviewImage.innerHTML = state.svgContent;
  
  // Ajustar el SVG renderizado para que quepa
  const svgEl = svgPreviewImage.querySelector('svg');
  if (svgEl) {
    svgEl.removeAttribute('width');
    svgEl.removeAttribute('height');
    svgEl.style.maxWidth = '100%';
    svgEl.style.maxHeight = '180px';
    
    // Usar viewBox si existe, o crear uno basado en el contenido
    if (!svgEl.getAttribute('viewBox')) {
      const bbox = svgEl.getBBox();
      svgEl.setAttribute('viewBox', `${bbox.x} ${bbox.y} ${bbox.width} ${bbox.height}`);
    }
  }
  
  // Llenar leyenda con datos del análisis
  const analysis = state.analysis; // datos del análisis SVG del Step 1
  
  if (analysis) {
    const legendCut = document.getElementById('legendCut');
    const legendVector = document.getElementById('legendVector');
    const legendRaster = document.getElementById('legendRaster');
    
    // Corte
    const cutLength = analysis.cut_length_mm || analysis.cutLengthMM || 0;
    if (cutLength > 0) {
      document.getElementById('legendCutValue').textContent = `${cutLength.toFixed(1)} mm`;
      legendCut.style.display = 'flex';
    } else {
      legendCut.style.display = 'none';
    }
    
    // Vector
    const vectorLength = analysis.vector_length_mm || analysis.vectorLengthMM || 0;
    if (vectorLength > 0) {
      document.getElementById('legendVectorValue').textContent = `${vectorLength.toFixed(1)} mm`;
      legendVector.style.display = 'flex';
    } else {
      legendVector.style.display = 'none';
    }
    
    // Raster
    const rasterArea = analysis.raster_area_mm2 || analysis.rasterAreaMM2 || 0;
    if (rasterArea > 0) {
      document.getElementById('legendRasterValue').textContent = `${rasterArea.toFixed(0)} mm²`;
      legendRaster.style.display = 'flex';
    } else {
      legendRaster.style.display = 'none';
    }
    
    // Dimensiones
    const width = analysis.width || analysis.widthMM || 0;
    const height = analysis.height || analysis.heightMM || 0;
    document.getElementById('legendDimensions').textContent = `${width.toFixed(1)} × ${height.toFixed(1)} mm`;
  }
  
  svgPreviewCard.style.display = 'block';
} else {
  svgPreviewCard.style.display = 'none';
}
```

En `resetWizard()`, agregar:
```javascript
document.getElementById('svgPreviewCard').style.display = 'none';
document.getElementById('svgPreviewImage').innerHTML = '';
state.svgContent = null;
```

**NOTA:** Verificar cómo se lee el archivo SVG en el Step 1. Si se usa FormData para enviarlo al API directamente, puede que no se lea como texto. En ese caso, hacer una lectura paralela con FileReader solo para el preview. Buscar:
```bash
grep -n "FileReader\|readAsText\|readAsDataURL\|FormData\|fileInput" [archivo_html]
```

---

## Cambio 3: Mejorar Mensaje de "Requiere Revisión"

### Problema
El badge "Requiere revisión" sin contexto es intimidante. El cliente no sabe por qué ni qué implica.

### Solución
Agregar un mensaje explicativo debajo del badge cuando el estado es "needs_review".

### HTML

Agregar DEBAJO del div `result-price-model` (o debajo del badge de estado):

```html
<!-- Status explanation -->
<div class="status-explanation" id="statusExplanation" style="display: none;">
  <span id="statusExplanationText"></span>
</div>
```

### CSS

```css
/* STATUS EXPLANATION */
.status-explanation {
  margin-top: 0.75rem;
  padding: 0.625rem 1rem;
  background: var(--bg-warm);
  border-radius: 8px;
  font-size: 0.8125rem;
  color: var(--text-muted);
  text-align: center;
  line-height: 1.5;
  max-width: 400px;
  margin-left: auto;
  margin-right: auto;
}

.status-explanation.review {
  background: var(--warning-bg);
  border: 1px solid rgba(234, 179, 8, 0.2);
}

.status-explanation.approved {
  background: var(--success-bg);
  border: 1px solid rgba(34, 197, 94, 0.2);
}
```

### JavaScript

En `displayResult()`, agregar después de configurar el badge de estado:

```javascript
// Status explanation
const statusExplanation = document.getElementById('statusExplanation');
const explanationText = document.getElementById('statusExplanationText');

const status = q.status || 'auto_approved';

const statusMessages = {
  'auto_approved': {
    text: 'Tu cotización está lista. Puedes proceder con tu pedido.',
    class: 'approved'
  },
  'needs_review': {
    text: 'Nuestro equipo revisará esta cotización y te contactará para confirmar el precio. Esto suele tomar menos de 24 horas.',
    class: 'review'
  },
  'approved': {
    text: 'Cotización aprobada por nuestro equipo.',
    class: 'approved'
  },
  'rejected': {
    text: 'Esta cotización fue rechazada. Contactanos para más información.',
    class: 'review'
  },
  'expired': {
    text: 'Esta cotización venció. Puedes crear una nueva.',
    class: ''
  }
};

const statusInfo = statusMessages[status];
if (statusInfo && status !== 'auto_approved') {
  explanationText.textContent = statusInfo.text;
  statusExplanation.className = `status-explanation ${statusInfo.class}`;
  statusExplanation.style.display = 'block';
} else if (status === 'auto_approved') {
  // Para auto_approved, mostrar solo si queremos. Opcional.
  statusExplanation.style.display = 'none';
} else {
  statusExplanation.style.display = 'none';
}
```

En `resetWizard()`:
```javascript
document.getElementById('statusExplanation').style.display = 'none';
```

---

---

## Cambio 4: Botón "Solicitar este trabajo" como CTA principal

### Problema
Los botones actuales son "Modificar opciones" y "Nueva cotización". No hay acción para que el cliente acepte la cotización y avance. "Nueva cotización" no debería ser el CTA principal.

### Solución
Reorganizar los botones: CTA principal = "Solicitar este trabajo", secundario = "Modificar opciones", terciario = "Nueva cotización".

### HTML

Reemplazar la sección de STEP 3 ACTIONS:

```html
<!-- STEP 3 ACTIONS -->
<div class="wizard-actions result-actions">
  <button class="btn btn-secondary" onclick="goToStep(2)">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M19 12H5M12 19l-7-7 7-7"/>
    </svg>
    Modificar opciones
  </button>
  <div class="result-actions-right">
    <button class="btn btn-ghost" onclick="resetWizard()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 5v14M5 12h14"/>
      </svg>
      Nueva cotización
    </button>
    <button class="btn btn-primary btn-request" id="btnRequestWork" onclick="requestWork()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M22 11.08V12a10 10 0 11-5.93-9.14"/>
        <polyline points="22 4 12 14.01 9 11.01"/>
      </svg>
      Solicitar este trabajo
    </button>
  </div>
</div>
```

### CSS

```css
/* RESULT ACTIONS */
.result-actions {
  flex-wrap: wrap;
  gap: 0.75rem;
}

.result-actions-right {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}

.btn-request {
  padding: 0.875rem 2rem;
  font-size: 1rem;
}

@media (max-width: 600px) {
  .result-actions {
    flex-direction: column;
  }
  .result-actions-right {
    width: 100%;
    flex-direction: column;
  }
  .result-actions-right .btn {
    width: 100%;
    justify-content: center;
  }
}
```

### JavaScript

```javascript
function requestWork() {
  // Si la cotización fue auto_approved, marcarla como "converted" o llevar al flujo de pedido
  // Si requiere revisión, informar al cliente que será contactado
  
  const status = state.lastQuoteStatus || 'auto_approved';
  
  if (status === 'needs_review') {
    // Cotización pendiente de revisión
    alert('Tu cotización está en revisión. Te contactaremos en menos de 24 horas para confirmar el precio y proceder.');
    // TODO: Enviar notificación al admin
    return;
  }
  
  if (status === 'auto_approved' || status === 'approved') {
    // Cotización aprobada — proceder
    // TODO: Redirigir a flujo de pedido o formulario de contacto
    // Por ahora, abrir WhatsApp con resumen
    const quoteId = state.lastQuoteId || '';
    const price = document.getElementById('resultPrice').textContent;
    const message = encodeURIComponent(
      `Hola! Quiero solicitar el trabajo de mi cotización #${quoteId}.\n` +
      `Precio: ₡${price}\n` +
      `Material: ${document.getElementById('summaryMaterial').textContent}\n` +
      `Tecnología: ${document.getElementById('summaryTech').textContent}`
    );
    // Reemplazar XXXXXXXXXX con número de WhatsApp del negocio
    window.open(`https://wa.me/506XXXXXXXX?text=${message}`, '_blank');
  }
}
```

**NOTA:** El número de WhatsApp y el flujo exacto de "solicitar trabajo" se definirán después. Por ahora el botón puede abrir WhatsApp con un mensaje pre-armado o mostrar un modal de confirmación. Lo importante es que el CTA exista.

---

## Cambio 5: Descargar PDF / Compartir por WhatsApp

### Problema
El cliente no puede guardar ni compartir su cotización fácilmente. En Costa Rica WhatsApp es el canal principal de comunicación comercial.

### Solución
Agregar botones de "Descargar PDF" y "Compartir por WhatsApp" debajo del resultado.

### HTML

Agregar DESPUÉS del div `result-breakdown` y ANTES de los STEP 3 ACTIONS:

```html
<!-- Share / Download Actions -->
<div class="share-actions" id="shareActions">
  <button class="share-btn" onclick="shareWhatsApp()">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
      <path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413z"/>
    </svg>
    Compartir por WhatsApp
  </button>
  <button class="share-btn" onclick="downloadQuotePDF()">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/>
      <polyline points="7 10 12 15 17 10"/>
      <line x1="12" y1="15" x2="12" y2="3"/>
    </svg>
    Descargar PDF
  </button>
</div>
```

### CSS

```css
/* SHARE ACTIONS */
.share-actions {
  display: flex;
  justify-content: center;
  gap: 0.75rem;
  margin-top: 1.5rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border);
}

.share-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.625rem 1.25rem;
  background: var(--bg-warm);
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--text-muted);
  cursor: pointer;
  font-family: inherit;
  transition: all 0.2s;
}

.share-btn:hover {
  background: var(--bg-card-hover);
  color: var(--text);
  border-color: var(--text-muted);
}

@media (max-width: 600px) {
  .share-actions {
    flex-direction: column;
  }
  .share-btn {
    width: 100%;
    justify-content: center;
  }
}
```

### JavaScript

```javascript
function shareWhatsApp() {
  const price = document.getElementById('resultPrice').textContent;
  const qty = document.getElementById('resultQty').textContent;
  const material = document.getElementById('summaryMaterial').textContent;
  const tech = document.getElementById('summaryTech').textContent;
  const quoteId = state.lastQuoteId || '';
  const validity = document.getElementById('resultValidity').textContent;
  
  const message = encodeURIComponent(
    `🔧 *Cotización FabricaLaser*\n\n` +
    `📋 Cotización #${quoteId}\n` +
    `💰 Precio: ₡${price}\n` +
    `📦 Cantidad: ${qty} unidad(es)\n` +
    `🪵 Material: ${material}\n` +
    `⚡ Tecnología: ${tech}\n` +
    `📅 Válido hasta: ${validity}\n\n` +
    `Ver en: ${window.location.origin}/cotizar?id=${quoteId}`
  );
  
  window.open(`https://api.whatsapp.com/send?text=${message}`, '_blank');
}

function downloadQuotePDF() {
  const quoteId = state.lastQuoteId || '';
  if (!quoteId) {
    alert('No se pudo generar el PDF. Intenta de nuevo.');
    return;
  }
  
  // Opción A: Si hay endpoint de PDF en el backend
  window.open(`${API_BASE}/quotes/${quoteId}/pdf`, '_blank');
  
  // Opción B: Si no hay endpoint, generar en frontend con html2canvas + jsPDF
  // (requiere librerías adicionales — implementar después si no hay endpoint)
}
```

**NOTA sobre PDF:** Si el backend no tiene endpoint de generación de PDF, hay dos caminos:
1. Crear endpoint `/api/v1/quotes/:id/pdf` que genere el PDF server-side (recomendado)
2. Generar en frontend con `html2canvas` + `jsPDF` (más rápido de implementar pero menos profesional)

Por ahora implementar el botón apuntando al endpoint del API. Si no existe, el botón muestra "Próximamente" o genera una versión simple en frontend.

---

## Cambio 6: Mostrar área del diseño en resumen

### Problema
El cliente no sabe cuánto material de su lámina se va a usar. Las dimensiones del diseño le dan contexto.

### Solución
Agregar las dimensiones del bounding box al resumen de opciones seleccionadas.

### HTML

Agregar dentro del div `selectedSummary`, al final:

```html
<div class="summary-item" id="areaSummaryItem">
  <span class="summary-item-label">Área del diseño:</span>
  <span class="summary-item-value" id="summaryArea">-</span>
</div>
```

### JavaScript

En `displayResult()`, agregar:

```javascript
// Área del diseño
const analysis = state.analysis;
if (analysis) {
  const w = analysis.width || analysis.bounds_width || 0;
  const h = analysis.height || analysis.bounds_height || 0;
  if (w > 0 && h > 0) {
    const areaMM2 = w * h;
    let areaDisplay;
    if (areaMM2 > 10000) {
      areaDisplay = `${w.toFixed(0)} × ${h.toFixed(0)} mm (${(areaMM2 / 100).toFixed(1)} cm²)`;
    } else {
      areaDisplay = `${w.toFixed(1)} × ${h.toFixed(1)} mm`;
    }
    document.getElementById('summaryArea').textContent = areaDisplay;
    document.getElementById('areaSummaryItem').style.display = 'flex';
  } else {
    document.getElementById('areaSummaryItem').style.display = 'none';
  }
} else {
  document.getElementById('areaSummaryItem').style.display = 'none';
}
```

---

## Verificación

```bash
# 1. Verificar elementos nuevos
grep -n "techDetailPanel\|svgPreviewCard\|statusExplanation\|btnRequestWork\|shareActions\|areaSummaryItem" [archivo_html]

# 2. Verificar funciones JS
grep -n "toggleTechDetail\|requestWork\|shareWhatsApp\|downloadQuotePDF" [archivo_html]

# 3. Verificar CSS
grep -n "tech-detail-toggle\|svg-preview\|status-explanation\|share-btn\|btn-request" [archivo_html]

# 4. Verificar reset limpia todo
grep -A10 "resetWizard" [archivo_html] | grep -c "techDetail\|svgPreview\|statusExplanation\|shareActions"
```

# 2. Verificar funciones JS
grep -n "toggleTechDetail\|svgContent\|statusMessages" [archivo_html]

# 3. Verificar CSS
grep -n "tech-detail-toggle\|svg-preview\|status-explanation" [archivo_html]

# 4. Verificar reset
grep -A5 "resetWizard" [archivo_html] | grep -c "techDetail\|svgPreview\|statusExplanation"
```

## Consistencia Visual

- Todos los estilos usan variables CSS existentes
- Border-radius 8-12px consistente
- Font sizes: 0.75rem, 0.8125rem, 0.875rem (existentes)
- Animación sutil en el chevron (0.3s) y el panel (slideDown)
- El SVG preview tiene fondo blanco para que los colores del diseño se vean claramente contra cualquier theme

---

## Cambio 4: "Total" en vez de "Precio unitario" cuando qty=1

### Problema
Cuando qty=1, "Precio unitario ₡9,677" es redundante — es lo mismo que el total. Confunde.

### Solución
En `displayResult()`, cambiar la etiqueta dinámicamente:

```javascript
// Ajustar label de precio unitario/total
const priceUnitLabel = document.querySelector('#priceUnit').closest('.breakdown-row').querySelector('.breakdown-row-label');
const qty = q.quantity || 1;
if (qty > 1) {
  priceUnitLabel.textContent = 'Precio unitario';
} else {
  priceUnitLabel.textContent = 'Total';
}
```

---

## Cambio 5: Reordenar botones de acción

### Problema
"Nueva cotización" es el botón primario (rojo) pero descarta todo. La acción primaria debería ser algo útil para el cliente.

### Solución
Reorganizar los botones:

```html
<!-- STEP 3 ACTIONS -->
<div class="wizard-actions">
  <button class="btn btn-secondary" onclick="goToStep(2)">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M19 12H5M12 19l-7-7 7-7"/>
    </svg>
    Modificar opciones
  </button>
  <div style="display: flex; gap: 0.75rem;">
    <button class="btn btn-secondary" onclick="copyQuoteSummary()" title="Copiar resumen">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
        <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
      </svg>
      Copiar
    </button>
    <button class="btn btn-ghost" onclick="resetWizard()">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 5v14M5 12h14"/>
      </svg>
      Nueva cotización
    </button>
  </div>
</div>
```

### JavaScript — Copiar resumen al portapapeles

```javascript
function copyQuoteSummary() {
  const price = document.getElementById('resultPrice').textContent;
  const qty = document.getElementById('resultQty').textContent;
  const validity = document.getElementById('resultValidity').textContent;
  
  // Obtener opciones seleccionadas
  const material = document.getElementById('summaryMaterial').textContent;
  const tech = document.getElementById('summaryTech').textContent;
  const thickness = document.getElementById('summaryThickness').textContent;
  
  const timeTotal = document.getElementById('timeTotal').textContent;
  
  const summary = [
    `🔷 Cotización FabricaLaser`,
    ``,
    `Precio: ₡${price}`,
    `Cantidad: ${qty} unidad(es)`,
    ``,
    `Material: ${material}`,
    `Tecnología: ${tech}`,
    `Grosor: ${thickness}`,
    `Tiempo estimado: ${timeTotal}`,
    ``,
    `Válido hasta: ${validity}`,
    ``,
    `fabricalaser.com`
  ].join('\n');
  
  navigator.clipboard.writeText(summary).then(() => {
    // Feedback visual
    const btn = event.target.closest('.btn');
    const originalText = btn.innerHTML;
    btn.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><polyline points="20 6 9 17 4 12"/></svg> Copiado`;
    btn.style.color = 'var(--success)';
    setTimeout(() => {
      btn.innerHTML = originalText;
      btn.style.color = '';
    }, 2000);
  }).catch(() => {
    // Fallback si clipboard API no disponible
    alert('Resumen:\n\n' + summary);
  });
}
```

---

## No Tocar

- Lógica de cálculo (backend)
- Steps 1 y 2 del wizard
- Historial de cotizaciones
- Panel admin