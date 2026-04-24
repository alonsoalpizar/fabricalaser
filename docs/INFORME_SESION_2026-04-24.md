# Informe de sesión — 2026-04-24

Documento de cierre. Tres secciones:

1. **Lo entregado**: qué se hizo, dónde quedó, estado operativo
2. **Lo pendiente**: deuda técnica, pasos del roadmap no ejecutados, observaciones sin resolver
3. **Atención operacional**: cosas que pueden morderte si no las tenés presentes

---

## 1. Lo entregado (8 commits, `main` a 8 commits de `origin/main`)

### Paso 1 — Unificación del contexto dinámico (commit `3852b9e`)

**Problema**: los 3 agentes LLM (chat web, WhatsApp/Telegram, admin chat) construían su "contexto dinámico" (tecnologías, materiales, catálogo de blanks, TelAsesor) por separado. El chat web recibía solo nombres; WhatsApp recibía IDs exactos.

**Solución**: paquete `internal/agent/context/` con un `Provider` único. Los 3 agentes consumen la misma función `GetDynamicContext()` → string con IDs, CostoVectorizacion, asesor, catálogo blanks. Cache 5 min compartido.

**Resultado**: chat web ahora recibe IDs numéricos y catálogo actualizado — pre-requisito que habilita los siguientes pasos.

### Paso 2 — Abstracción multi-LLM con hot reload (commit `a2229a3`, 3525 insertions)

**Problema**: los 3 agentes se acoplaban directo a `cloud.google.com/go/vertexai/genai`. Imposible conmutar proveedor sin recompilar.

**Solución**: interface `llm.Client` en `internal/agent/llm/` con 3 adapters:
- `vertex.go` (wrap del SDK existente, usa ADC del service account)
- `openai.go` (cubre OpenAI, DeepSeek, Kimi, Groq vía HTTP nativo)
- `anthropic.go` (`/v1/messages` vía HTTP nativo)

Factory con hot reload atómico + encriptación AES-256-GCM de api_keys usando `sha256(APP_SECRET)`. UI en `/admin/configuracion-llm.html` para conmutar proveedor y modelo desde el panel.

**Reglas clave aplicadas**: R1 isolation (solo `internal/agent/llm/` importa SDKs), R3 api_key nunca en GET response, R4b test efímero, R5 fallback stub si cliente inicial falla, R12 observabilidad con `slog.Info("llm.chat", ...)`.

**Verificado con 4 proveedores reales**: Vertex, DeepSeek, OpenAI, Kimi. Anthropic adapter está implementado pero no probado con key real.

### Paso 4 — Eliminar precios hardcoded del chat web (commit `ff8b94a`)

**Problema**: `systemInstruction` del chat web tenía precios específicos de llaveros (`₡6.000`, `₡240 c/u`, etc.) y medallas (`₡375 c/u`) que se desincronizaban de `blanks_catalog` en cada cambio. Si el asesor editaba precios en DB, el agente seguía diciendo los viejos.

**Solución**: eliminados los 6 montos específicos del prompt. Agregada instrucción explícita: *"NUNCA inventés ni cites montos específicos. Redirigí al catálogo (fabricalaser.com) o a WhatsApp/Telegram."*. El agente mantiene conocimiento de productos (formas, reglas, mínimos) pero para precios redirige.

**Verificado**: pregunta *"¿Cuánto cuestan 25 llaveros redondos?"* → agente invita al registro con link al catálogo, sin dar cifras.

### Paso 5 — Gestión de prompts desde admin (commits `25935c1` backend + `fd003d9` UI)

**Problema**: los 5 system prompts de los agentes (`chat_web_public`, `chat_web_logged`, `whatsapp_main`, `whatsapp_image`, `admin_chat`) eran `const` Go. Cambiar cualquiera requería editar código + recompilar + redeploy.

**Solución**:
- Tabla dedicada `agent_prompts` + histórica `agent_prompt_versions` (no reutiliza `system_config` — justificado por scope de texto largo + versionado)
- Paquete `internal/agent/prompts/` con Provider que sirve desde DB con cache + fallback hardcoded en 3 niveles (DB error → fila no existe → body vacío) + pub/sub Redis para hot reload
- Handler admin con 7 endpoints bajo `/api/v1/admin/prompts/` (list, get, save, versions, getVersion, rollback, sandbox)
- UI en `/admin/prompts.html` (2152 líneas) con layout 3 columnas, editor con sandbox multi-turn, diff LCS lado a lado, historial con timeline + badges `↩ Restaurado desde v{N}`, shortcuts Cmd+S/Cmd+Enter/Esc, dirty tracking con `beforeunload`, optimistic UI

**Bug fix de rollback**: el save creaba 2 filas en `agent_prompt_versions` (histórica v=N + activa v=N+1). Al rollback intentar insertar la nueva como histórica chocaba con duplicate key. Resuelto con `gorm clause.OnConflict{DoNothing: true}`.

**Verificado end-to-end**: guardar un prompt con marcador, logs muestran `cache invalidated by pubsub`, request inmediato al chat web usa el nuevo prompt, rollback a v1 restaura el body original y crea v3 con `restored_from_version=1`.

### Prompt caching en los 3 adapters (commit `2f27f6b`)

**Problema**: los system prompts largos (~3k tokens WhatsApp, ~2.5k chat web) se cobran completo en cada request. Con 100 mensajes/día = 9M tokens/mes solo repitiendo el prompt.

**Solución por proveedor**:
- **Anthropic (explícito)**: `cache_control: {type: "ephemeral"}` al system si ≥4000 chars (~1000 tokens). 90% descuento en reads.
- **OpenAI-compat (automático)**: captura `prompt_tokens_details.cached_tokens` (OpenAI) y `prompt_cache_hit_tokens` (DeepSeek). 50% descuento automático.
- **Vertex (implicit)**: activo gratis en Gemini 2.5 Flash, pero el SDK v0.13.2 no expone el count. Documentado como deuda técnica.

Nuevo campo `Response.TokensCached` + logging `tokens_cached` en `slog.Info("llm.chat", ...)`.

**Verificado con DeepSeek**: 3 requests consecutivos mostraron `tokens_cached=1792` (95% del prompt cacheado). Ahorro real inmediato sin más configuración.

### Mejoras de UI del admin (commits `93823c4`, `d48eb13`, `c280498`, `107ba9e`, `1f8aef1`, `b5402f1`)

- `/admin/config/general.html`: categorías `llm` y `operational` con labels + íconos + orden priorizado, keys protegidas (`llm_*`) redirigen a `/admin/configuracion-llm.html` en lugar de permitir edición inline peligrosa, background blanco roto corregido a `--bg-card`
- `/admin/configuracion-llm.html`: hero card con franja acento vertical + kicker chip "ACTIVO AHORA" con dot pulsante animado, banner info con texto blanco legible, botón "Probar" en ghost distinto de "Guardar" primary
- Meta `no-cache` en los 16 HTMLs del admin para que el browser siempre revalide HTML
- Cache busting `?v=7` en `admin.js` + `admin.css` cuando el admin.js cambió (con item nuevo del sidebar)

---

## 2. Lo pendiente

### Del roadmap documentado del Paso 2 (plan multi-LLM, sección "Siguientes pasos")

#### Paso 3 — Tools compartidas en `internal/agent/tools/`

**Estado**: NO ejecutado. Cada adapter define sus tools localmente:
- WhatsApp/Telegram: `calcular_cotizacion`, `consultar_blank`, `escalar_a_humano` en `internal/whatsapp/gemini_adapter.go`
- Admin chat: 6 tools más ricas en `internal/handlers/admin/chat/tools.go`
- Chat web: **SIN tools** — redirige siempre al catálogo (Paso 4 compensó parcialmente)

**Qué implicaría**:
- Extraer las definiciones a `internal/agent/tools/` con schema neutro JSON
- Implementar ejecutores una sola vez
- Agregar tools al chat web para que pueda cotizar directamente en conversación (hoy solo redirige a WhatsApp)

**Prioridad**: alta si se quiere que el chat web sea funcional como cotizador conversacional.

#### Paso 6 (opcional) — Consolidar admin chat provider con el compartido

**Estado**: NO ejecutado. El admin chat sigue con su propio `ContextProvider` (7 repos — tarifas, descuentos, factores) distinto de `agentctx.Provider` (4 repos — techs, materials, blanks, sys_config).

**Justificación de dejarlo así**: el admin chat tiene caso de uso distinto — el gestor puede pedir "explicame cómo llegaste a ese precio" y necesita acceso a tarifas/descuentos que no tiene sentido exponer al chat público.

**Prioridad**: baja. Solo si mantener los 2 providers empieza a doler.

### De observaciones del análisis del pricing (sesión anterior)

**Estado**: detectadas como debilidades, NO resueltas. Están en `docs/PRICING_FORMULA_ACTUAL.md` + `docs/REFACTOR_PRICING_V2.md` y en el análisis de la sesión.

Los 5 puntos más críticos:

1. **Doble amplificación en tipos de grabado** — `speed_multiplier` (ralentiza tiempo) + `factor` (markup precio) aplicados multiplicativamente. Fotograbado termina 12.5× más caro que vectorial del mismo tamaño. Probablemente intencional parcialmente, pero los ratios actuales son fuera de mercado.

2. **FactorMaterial NO aplicado en modelo Hybrid** — el `SimHybridWithMaterialFactor` en el código del Calculator es evidencia de que el autor sabe que falta. En jobs donde Hybrid gana (fotograbado grande), no se cobra el markup por material — metales anormalmente baratos.

3. **`materialFactor` usado como divisor de velocidad** — en `time_estimator.go:71` se usa para ralentizar el láser, pero es una variable de *precio comercial*, no física. Para acero inox (factor 2.0) dice "el láser es 2× más lento" lo cual no es real.

4. **Setup fee único sin minimum order amount** — un pedido de 1 pieza de ₡500 cobra `setup + machine + material` pero podría quedar bajo costo. Falta piso mínimo de orden (ej. ₡10,000).

5. **Endpoint `/estimate` con vectorial como perímetro** — cuando el cliente pide vectorial sin SVG, el endpoint usa `perímetro del bounding box` como `VectorLengthMM`. Para texto/números reales, el vector length real puede ser 10× el perímetro. Los precios vectoriales están subestimados desde WhatsApp.

**Prioridad**: alta para los 5. Son bugs de pricing que afectan margen real, no mejoras cosméticas.

### De la implementación del Paso 5

- **Reviewer agent no ejecutado**: el plan preveía un agente final de `reviewer` para audit de seguridad. Se hizo manual (R1 check via Makefile, R3 check por grep). Sería bueno pasar un reviewer formal antes de push a producción.
- **Tests unitarios**: ninguno de los paquetes nuevos tiene tests. `prompts.Provider`, `llm.Factory`, el repository... todo se verificó por smoke test end-to-end.
- **Subida de imágenes al chat web**: los 3 adapters soportan imágenes pero el chat web (/api/v1/chat) solo acepta JSON text. Para habilitar análisis visual desde la web, hay que extender el endpoint a multipart.

### SDK deprecation

- **`cloud.google.com/go/vertexai` v0.13.2 está deprecated** desde junio 2025 y se elimina **junio 2026**.
- Cada arranque del servicio logea 3× `WARNING: Starting on June 24, 2025, the cloud.google.com/go/vertexai/genai package is deprecated...`
- El nuevo SDK es `google.golang.org/genai`. Requiere migración de `vertex.go` (~1 día).
- Beneficio adicional de migrar: exponer `cached_content_token_count` para completar la observabilidad de Vertex.
- **Prioridad**: media. Tenés hasta junio 2026 pero mejor no dejarlo al final.

---

## 3. Atención operacional

### Cosas que pueden morderte si no las tenés presentes

#### 🔐 `FABRICALASER_APP_SECRET` es la llave del reino

- Encripta las `api_key` de todos los proveedores LLM (AES-256-GCM con `sha256(APP_SECRET)`)
- **Si rotás este secreto**, las api_keys encriptadas en `system_config.llm_api_key` se vuelven basura ilegible
- El gestor debe **reingresar las credenciales desde la UI** (`/admin/configuracion-llm.html`) después de rotar
- Está en `/opt/FabricaLaser/.env` — backup seguro del archivo (NO commitear)
- Si se pierde el archivo `.env` y no hay backup: pérdida total de las api_keys encriptadas (recuperables solo yendo a los paneles de OpenAI/DeepSeek/Anthropic y regenerándolas)

#### 📦 Hot reload funciona pero depende de Redis

- Tanto el cambio de proveedor LLM como el guardado de prompts publican en Redis (`prompts:updated`)
- Si Redis cae, los cambios siguen aplicando **localmente** en el proceso que recibió el save, pero no se propagarían a otros procesos si hubiera escalado horizontal (hoy hay un solo servicio, no aplica)
- El `llm.Factory` y `prompts.Provider` caen al fallback si Redis o DB fallan — el servicio NUNCA arranca sin cliente LLM válido

#### 🎭 Cache del browser al modificar `admin.js` o `admin.css`

- Los `<meta cache-control: no-cache>` solo afectan al HTML, **NO** a los assets JS/CSS
- Cada cambio en admin.js (nuevo item de sidebar, helper nuevo) requiere bumpear `?v=N` en los 16 HTMLs del admin
- **Si no lo hago, el gestor no ve los cambios** aunque el servidor sirva la versión nueva
- Está documentado en mi memoria persistente (`~/.claude/.../feedback_admin_js_cache_busting.md`) para no olvidarlo
- Actual en producción: `?v=7`

#### 🗄️ Prompts en 3 lugares redundantes (ventaja, no problema)

Los 5 prompts originales están guardados en:
1. **Código Go**: `internal/agent/prompts/fallbacks.go` (567 líneas, compilado en binario)
2. **DB `agent_prompts`**: v1 + cualquier versión posterior en `agent_prompt_versions`
3. **Git history**: commits anteriores a `25935c1`

Cualquiera de los 3 puede reconstruir los otros. **Nada se pierde** bajo escenarios normales de falla.

#### 🏷️ Dos páginas distintas para config LLM

Los gestores pueden confundirse:

- **`/admin/configuracion-llm.html`** — página dedicada para el PROVEEDOR LLM activo (Vertex/OpenAI/DeepSeek/Kimi/Anthropic) + api_key + modelo. El flujo correcto para cambiar credenciales.
- **`/admin/config/general.html`** — config general del sistema, que MUESTRA las keys `llm_*` en modo lectura con botón "Configurar aquí" que redirige a la página dedicada. Por diseño — editar `llm_api_key` inline ahí rompería la encriptación.

Documentar esto en un onboarding para gestores nuevos.

#### 💸 Kimi tiene 2 endpoints distintos

- `api.moonshot.cn` (China) — keys registradas en plataforma china
- `api.moonshot.ai` (international) — keys desde LATAM/Europa

El default del adapter es `.ai` (internacional) — correcto para Costa Rica. Si por alguna razón alguien tiene una key china, debe override con el campo Endpoint en la UI.

#### 🚪 Proveedor Vertex funciona sin api_key

- Usa ADC (Application Default Credentials) del service account del servidor
- Si alguien desconfigura ADC en GCP o revoca permisos del proyecto `div-aloalpizar`, Vertex empieza a fallar
- El servicio tiene fallback stub (arranca igual), pero los agentes retornan error hasta que el gestor conmute a otro proveedor

#### 🧾 Observabilidad

Para monitorear costo y salud:

```bash
# Ver todas las llamadas LLM de los últimos 10 min
sudo journalctl -u fabricalaser-api --since "10 min ago" | grep "llm.chat"

# Salida típica:
# INFO llm.chat provider=deepseek model=deepseek-chat latency_ms=2298 tokens_in=1878 tokens_out=122 tokens_cached=1792
```

Campos útiles: `provider`, `model`, `latency_ms`, `tokens_in`, `tokens_out`, `tokens_cached`. Con esto podés armar dashboards de costo y performance por proveedor.

#### 🧪 Los prompts se pueden editar sin redeploy — pero validar

- El botón "Probar en sandbox" permite iterar prompts contra el LLM activo sin persistir
- Después de cada "Guardar cambios", el próximo request al chat web / WhatsApp / admin chat usa el nuevo prompt (<2s)
- **Si un prompt introduce un bug grave** (ej. el modelo responde rudo, revela info interna, entra en loop), usar el botón "Restaurar" en el historial para volver a la versión anterior — 1 click
- El historial queda append-only, no se pierde nada

---

## Estado actual del repositorio

- **Branch**: `main`
- **Ahead de `origin/main`**: 8 commits (no pusheados)
- **Working tree**: limpio
- **Servicio**: `fabricalaser-api` activo
- **Provider LLM activo**: `vertex` (Gemini 2.5 Flash via ADC)
- **api_keys**: todas vacías (sin proveedor de pago activo)
- **Prompts en DB**: 5 prompts en v1 (excepto `chat_web_public` en v3 tras test de rollback, con body idéntico a v1)

### Cadena de commits de esta sesión

```
2f27f6b feat: prompt caching en los 3 adapters LLM + TokensCached
b5402f1 ui: bump admin.js + admin.css a ?v=7 en los 16 HTMLs del admin
fd003d9 feat: UI /admin/prompts.html + fix rollback duplicate key (Paso 5 — frontend)
25935c1 feat: gestión de prompts desde admin (Paso 5 — backend)
ff8b94a feat(chat-web): eliminar precios hardcoded (Paso 4)
107ba9e ui: rediseño visual contundente de /admin/configuracion-llm.html
1f8aef1 ui: meta cache-control no-cache en todas las páginas del admin
c280498 ui: mejorar espaciado de /admin/configuracion-llm.html
d48eb13 ui: alinear /admin/config/general.html con el tema oscuro
93823c4 ui: mejorar /admin/config/general.html para LLM y keys protegidas
a2229a3 feat: abstracción multi-LLM con hot reload (Paso 2)
3852b9e refactor: unificar contexto dinámico (Paso 1)
```

### Para pushear a GitHub

```bash
cd /opt/FabricaLaser && git push origin main
```

Antes de pushear, vale:
1. Audit final con el reviewer agent (no ejecutado en esta sesión)
2. Decidir si querés squashear algunos UI commits consecutivos del mismo tema
3. Verificar que el servicio sigue estable tras un período de 10-15 min

---

## Próximos pasos sugeridos (priorizados)

### Alta prioridad
1. **Atacar los bugs del pricing** (debilidades de sesión anterior)
2. **Paso 3**: tools compartidas + chat web funcional como cotizador
3. **Audit del reviewer antes de push**

### Media prioridad
4. Migración a nuevo SDK Vertex (`google.golang.org/genai`)
5. Tests unitarios mínimos para `prompts.Provider` y `llm.Factory`
6. Subida de imágenes al chat web

### Baja prioridad / opcional
7. Consolidar admin chat provider
8. A/B testing de prompts (roadmap Paso 6 multi-LLM)
9. Métricas de costo por proveedor en dashboard admin
10. Permisos granulares por-prompt

---

**Fin del informe.** — 2026-04-24
