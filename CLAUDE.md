# CLAUDE.md — FabricaLaser

## Proyecto
**FabricaLaser.com** es una plataforma de cotización automática de corte y grabado láser. Analiza archivos SVG, extrae métricas geométricas, aplica reglas de pricing paramétricas (modelo híbrido dual) y genera cotizaciones instantáneas. Soporta multi-tecnología: CO2, UV, Fibra, MOPA.

**Ubicación:** `/opt/FabricaLaser`  
**Puerto API:** 8083  
**Base de datos:** PostgreSQL `fabricalaser` (localhost:5432)  
**Cache:** Redis (localhost:6379, db: 3)  
**Dominio:** fabricalaser.com

## Stack
- **Backend:** Go 1.22 + Chi router + GORM (driver: pgx)
- **Frontend Admin:** React + TypeScript (web/admin/)
- **Frontend Wizard:** React + TypeScript (web/wizard/)
- **Motor SVG:** Go puro (encoding/xml + math, cero dependencias externas)
- **Web Server:** Nginx reverse proxy → :8083
- **Proceso:** systemd service `fabricalaser-api`

## Estructura
```
/opt/FabricaLaser/
├── cmd/server/main.go
├── internal/
│   ├── config/                 # Env vars, configuración
│   ├── models/                 # Modelos GORM
│   ├── handlers/               # HTTP handlers por dominio
│   │   ├── auth/               # Registro, login, JWT
│   │   ├── config/             # Endpoints públicos de configuración
│   │   ├── admin/              # CRUD admin (protegido)
│   │   ├── quotes/
│   │   ├── materials/
│   │   ├── orders/
│   │   └── users/              # Gestión usuarios (admin)
│   ├── services/
│   │   ├── auth/               # Lógica autenticación
│   │   ├── cedula/             # Validación GoMeta API (Registro Civil CR)
│   │   ├── svgengine/          # Motor análisis SVG
│   │   ├── pricing/            # Motor pricing híbrido
│   │   └── validation/
│   ├── middleware/              # Auth JWT, CORS, logging, rate limit, quota check
│   └── repository/             # Capa acceso a datos
├── web/
│   ├── landing/                # HTML estático — cara pública de fabricalaser.com
│   ├── admin/                  # React app admin (rol ADMIN)
│   └── wizard/                 # React app wizard (requiere auth)
├── migrations/                 # SQL (001_, 002_...)
├── uploads/                    # SVGs (no commitear)
├── scripts/
├── CLAUDE.md
├── go.mod / go.sum / Makefile
```

## Convenciones Go
- `gofmt` obligatorio, errores siempre manejados (nunca `_`)
- Naming: camelCase (privado), PascalCase (exportado)
- Response JSON: `{"data": ..., "error": null}` o `{"data": null, "error": {"code": "...", "message": "..."}}`
- Flujo: Handler → Service → Repository (nunca saltar capas)
- Handlers: solo parsean request y formatean response. Lógica en services
- Services: no conocen HTTP (no reciben *http.Request)
- Tests: `_test.go` junto al código. Obligatorios para services/ y svgengine/

## Convenciones React/TypeScript
- Componentes funcionales + hooks (no clases)
- TypeScript strict mode
- Estructura: components/, hooks/, services/, types/, pages/
- API centralizada en services/api.ts
- Estado local preferido, elevar solo cuando necesario

## Modelo de Datos

### Entidades
- **User** — Usuario centralizado (cedula unique, cedula_type: fisica|juridica, nombre, apellido, email, telefono, password_hash bcrypt, role: customer|admin, quote_quota default 5, quotes_used, activo, direccion, provincia, canton, distrito, metadata JSONB)
- **Technology** — CO2, UV, Fibra, MOPA (code, name, uv_premium_factor, is_active)
- **Material** — Con factor de ajuste (name, category, factor 1.0-1.8, thicknesses[], notes)
- **EngraveType** — Tipo grabado con factor tiempo (name, factor 1.0-3.0, speed_multiplier)
- **TechRate** — Tarifas por tecnología (engrave_rate_hour, cut_rate_hour, design_rate_hour, cost_per_min_engrave, cost_per_min_cut, setup_fee)
- **SVGAnalysis** — Resultado análisis (cut_length_mm, vector_length_mm, raster_area_mm2, element_count, bounding_box, warnings[])
- **Quote** — Cotización dual (user_id, analysis_id, tech_id, material_id, engrave_type_id, quantity, time_engrave, time_cut, cost_base, price_hybrid, price_value, adjustments{}, status)
- **VolumeDiscount** — Descuentos por cantidad (min_qty, max_qty, discount_pct)
- **Order** — Orden fabricación (quote_id, user_id, status, payment_status, operator_notes)
- **PriceReference** — Tabla precios referencia (service_type, min_usd, max_usd, typical_time)

### Relaciones
- User 1:1 UserProfile
- User 1:N Quote (con control de cuota: quotes_used < quote_quota)
- User 1:N Order
- Technology 1:N TechRate
- SVGAnalysis 1:N Quote
- Quote 1:1 Order

### Arquitectura Web (3 capas)
- **Landing** (fabricalaser.com): HTML estático público — cara del negocio, portafolio, CTA
- **Cotizador** (fabricalaser.com/cotizar): Auth requerido — wizard SVG, historial
- **Admin** (fabricalaser.com/admin): Solo rol ADMIN — gestión total

### Auth y Usuarios (idéntico a /opt/Payments)

**Validación Cédula CR:**
- Física: 9 dígitos, no empieza con 0 (regex: `^[1-9]\d{8}$`)
- Jurídica: 10 dígitos, no empieza con 0 (regex: `^[1-9]\d{9}$`)
- Limpiar caracteres no numéricos antes de validar

**Integración GoMeta API (Registro Civil CR): ✅ IMPLEMENTADA**
- Servicio: `internal/services/cedula/cedula_service.go`
- Endpoint externo: `https://apis.gometa.org/cedulas/{cedula}`
- Timeout: 10 segundos (configurable)
- Cache: 24 horas en metadata del usuario
- Datos obtenidos: nombre oficial, apellidos, tipo (física/jurídica)
- Uso: Pre-llenar registro, validar identidad, facturación electrónica

**Endpoints Auth:**
| Endpoint | Método | Auth | Descripción |
|----------|--------|------|-------------|
| `/api/v1/auth/verificar-cedula` | POST | No | `{identificacion}` → `{existe, tienePassword, tipo, cedula, validadoRegistroCivil, datosRegistroCivil}` |
| `/api/v1/auth/registro` | POST | No | `{identificacion, nombre, email, telefono, password}` → `{token, usuario}` (valida GoMeta, guarda metadata) |
| `/api/v1/auth/login` | POST | No | `{identificacion, password}` → `{token, usuario}` |
| `/api/v1/auth/establecer-password` | POST | No | `{identificacion, password, email?, telefono?}` → `{token, usuario}` (valida GoMeta si no hay metadata) |
| `/api/v1/auth/me` | GET | JWT | `→ {usuario}` |
| `/api/v1/auth/profile` | GET | JWT | `→ {perfil completo con direccion}` |
| `/api/v1/auth/profile` | PUT | JWT | `{email?, telefono?, provincia?, canton?, distrito?, direccion?}` → `{usuario}` |

**Endpoints Config (públicos):**
| Endpoint | Método | Auth | Descripción |
|----------|--------|------|-------------|
| `/api/v1/config` | GET | No | Toda la configuración en una llamada (frontend initial load) |
| `/api/v1/config/technologies` | GET | No | Lista tecnologías activas (CO2, UV, Fibra, MOPA) |
| `/api/v1/config/materials` | GET | No | Lista materiales con factores y espesores |
| `/api/v1/config/engrave-types` | GET | No | Tipos de grabado con factores |
| `/api/v1/config/tech-rates` | GET | No | Tarifas por tecnología |
| `/api/v1/config/volume-discounts` | GET | No | Descuentos por cantidad |
| `/api/v1/config/price-references` | GET | No | Referencias de precios por servicio |

**Endpoints Admin (requieren JWT + role=admin):**
| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/api/v1/admin/technologies` | POST | Crear tecnología |
| `/api/v1/admin/technologies/{id}` | PUT | Actualizar tecnología |
| `/api/v1/admin/technologies/{id}` | DELETE | Desactivar tecnología (soft delete) |
| `/api/v1/admin/materials` | POST | Crear material |
| `/api/v1/admin/materials/{id}` | PUT | Actualizar material |
| `/api/v1/admin/materials/{id}` | DELETE | Desactivar material |
| `/api/v1/admin/engrave-types` | POST | Crear tipo de grabado |
| `/api/v1/admin/engrave-types/{id}` | PUT | Actualizar tipo de grabado |
| `/api/v1/admin/engrave-types/{id}` | DELETE | Desactivar tipo de grabado |
| `/api/v1/admin/tech-rates/{id}` | PUT | Actualizar tarifas por tecnología |
| `/api/v1/admin/volume-discounts` | POST | Crear descuento por volumen |
| `/api/v1/admin/volume-discounts/{id}` | PUT | Actualizar descuento |
| `/api/v1/admin/volume-discounts/{id}` | DELETE | Desactivar descuento |
| `/api/v1/admin/price-references` | POST | Crear referencia de precio |
| `/api/v1/admin/price-references/{id}` | PUT | Actualizar referencia |
| `/api/v1/admin/price-references/{id}` | DELETE | Desactivar referencia |
| `/api/v1/admin/users` | GET | Listar usuarios (placeholder) |
| `/api/v1/admin/users/{id}/quota` | PUT | Actualizar cuota de cotizaciones |

**Endpoints Quotes (requieren JWT):**
| Endpoint | Método | Auth | Descripción |
|----------|--------|------|-------------|
| `/api/v1/quotes/analyze` | POST | JWT+Quota | Subir SVG y analizarlo → `{data: SVGAnalysis, cached, message}` |
| `/api/v1/quotes/calculate` | POST | JWT+Quota | Calcular precio → `{data: Quote, message}` |
| `/api/v1/quotes/my` | GET | JWT | Mis cotizaciones → `{data: [], total, limit, offset}` |
| `/api/v1/quotes/analyses` | GET | JWT | Mis análisis SVG → `{data: [], total, limit, offset}` |
| `/api/v1/quotes/{id}` | GET | JWT | Ver cotización específica → `{data: Quote}` |

**Request `/quotes/analyze` (multipart/form-data):**
```
POST /api/v1/quotes/analyze
Content-Type: multipart/form-data
Authorization: Bearer <token>

svg: <archivo.svg>  (max 5MB)
```

**Response `/quotes/analyze`:**
```json
{
  "data": {
    "id": 1,
    "filename": "logo.svg",
    "width_mm": 100,
    "height_mm": 50,
    "cut_length_mm": 245.5,
    "vector_length_mm": 120.3,
    "raster_area_mm2": 1500.0,
    "element_count": 15,
    "status": "analyzed",
    "warnings": [],
    "created_at": "2026-02-25T..."
  },
  "cached": false,
  "message": "SVG analizado correctamente"
}
```

**Request `/quotes/calculate`:**
```json
{
  "analysis_id": 1,
  "technology_id": 2,
  "material_id": 1,
  "engrave_type_id": 1,
  "quantity": 10,
  "thickness": 3.0
}
```

**Response `/quotes/calculate`:**
```json
{
  "data": {
    "id": 1,
    "time_breakdown": {
      "engrave_mins": 12.5,
      "cut_mins": 8.2,
      "setup_mins": 5.0,
      "total_mins": 25.7
    },
    "cost_breakdown": {
      "engrave": 3.29,
      "cut": 2.43,
      "setup": 0,
      "base": 5.72
    },
    "factors": {
      "material": 1.0,
      "engrave": 1.0,
      "uv_premium": 0.2,
      "margin": 0.4,
      "volume_discount": 0.05
    },
    "pricing": {
      "hybrid_unit": 9.61,
      "hybrid_total": 91.29,
      "value_unit": 8.50,
      "value_total": 80.75,
      "final": 91.29
    },
    "status": "auto_approved",
    "valid_until": "2026-03-04T...",
    "technology": "Láser UV",
    "material": "Madera / MDF",
    "engrave_type": "Vectorial"
  },
  "message": "Cotización calculada correctamente"
}
```

**Códigos de Error Quotes:**
| Código | HTTP | Descripción |
|--------|------|-------------|
| `NO_FILE` | 400 | No se envió archivo SVG |
| `INVALID_FILE_TYPE` | 400 | Archivo no es SVG |
| `INVALID_SVG` | 400 | SVG mal formado |
| `ANALYSIS_ERROR` | 400 | Error al analizar SVG |
| `ANALYSIS_NOT_FOUND` | 404 | Análisis no existe |
| `FORBIDDEN` | 403 | No tiene permiso |
| `QUOTA_EXCEEDED` | 403 | Cuota de cotizaciones agotada |
| `MISSING_FIELDS` | 400 | Faltan campos requeridos |

**Respuesta `/verificar-cedula` (con GoMeta):**
```json
{
  "existe": false,
  "tienePassword": false,
  "tipo": "fisica",
  "cedula": "117520936",
  "validadoRegistroCivil": true,
  "datosRegistroCivil": {
    "nombre": "Evelyn",
    "apellido": "Carvajal Fernandez",
    "nombreCompleto": "Carvajal Fernandez Evelyn",
    "primerNombre": "Evelyn",
    "primerApellido": "Carvajal",
    "segundoApellido": "Fernandez",
    "tipo": "fisica"
  }
}
```

**Códigos de Error Auth:**
| Código | HTTP | Descripción |
|--------|------|-------------|
| `INVALID_CEDULA` | 400 | Formato de cédula inválido |
| `CEDULA_NOT_VALID` | 400 | Cédula no existe en Registro Civil |
| `VALIDATION_OFFLINE` | 503 | Servicio GoMeta no disponible |
| `CEDULA_EXISTS` | 400 | Ya existe cuenta con esta cédula |
| `EMAIL_EXISTS` | 400 | Email ya registrado |
| `INVALID_PASSWORD` | 401 | Contraseña incorrecta |
| `ACCOUNT_DISABLED` | 401 | Cuenta desactivada |

**JWT:**
- Algoritmo: HS256, Expiración: 24h
- Payload: `{id, cedula, nombre, email, role, tipo: "customer"}`
- Header: `Authorization: Bearer <token>`

**Cuotas:**
- 5 cotizaciones al registrarse (quote_quota=5)
- Admin puede extender: N cotizaciones o ilimitado (quote_quota=-1)
- Middleware QuotaMiddleware valida quotes_used < quote_quota

**Roles:**
- `customer`: self-register, cotizar hasta cuota, ver historial
- `admin`: todo, creado por seed o manualmente

**Middleware Stack:**
- AuthMiddleware: verifica JWT, extrae usuario
- RoleMiddleware: verifica role=admin para rutas admin
- QuotaMiddleware: verifica cuota antes de cotizar

**bcrypt:** cost=12 para password_hash

## Reglas de Negocio

### Convenciones SVG (INMUTABLES)
| Color | Hex | Atributo | Operación | Métrica |
|-------|-----|----------|-----------|---------|
| Rojo | #FF0000 | stroke | Corte | Longitud (mm) |
| Azul | #0000FF | stroke | Grabado Vector | Longitud (mm) |
| Negro | #000000 | fill | Grabado Raster | Área (mm²) |

### Factores por Material (seed data)
| Material | Factor |
|----------|--------|
| Madera/MDF | 1.0 |
| Acrílico | 1.2 |
| Plástico ABS/PC | 1.25 |
| Cuero/Piel | 1.3 |
| Vidrio/Cristal | 1.5 |
| Cerámica | 1.6 |
| Metal con coating | 1.8 |

### Factores por Tipo Grabado (seed data)
| Tipo | Factor | Velocidad |
|------|--------|-----------|
| Vectorial | 1.0 | 1x |
| Rasterizado | 1.5 | 2x |
| Fotograbado | 2.5 | 4-5x |
| 3D/Relieve | 3.0 | 6x+ |

### Tarifas Base UV (seed data)
- Operador grabado: $12/hora → $0.263/min (con overhead $3.78/hr)
- Operador corte: $14/hora → $0.296/min (con overhead)
- Diseño: $15/hora
- Margen recomendado: 40%
- Premium UV: 15-25%

### Descuentos Volumen (seed data)
| Cantidad | Descuento |
|----------|-----------|
| 1-9 | 0% |
| 10-24 | 5% |
| 25-49 | 10% |
| 50-99 | 15% |
| 100+ | 20% |

### Fórmula Pricing — Modelo Híbrido
```
Costo_Base = (Tiempo_Grabado × $0.263) + (Tiempo_Corte × $0.296) + Material + Prep + Setup
Precio_Híbrido = Costo_Base × (1 + Margen) × Factor_Material × Factor_TipoGrabado × (1 + Premium_UV)
```

### Fórmula Pricing — Modelo por Valor
```
Precio_Valor = (Precio_Base_Pieza × Cantidad) - Descuento_Volumen + Cargo_Diseño
```

### Clasificación Automática
- **AUTO_APPROVED**: SVG limpio, pocos elementos, factor grabado ≤ 1.5, precio en rango normal
- **NEEDS_REVIEW**: Fotograbado/3D (factor ≥ 2.5), material premium (factor ≥ 1.5), precio alto
- **REJECTED**: Archivo inválido, colores incorrectos, no SVG, excede 10MB

## Motor SVG (internal/services/svgengine/)
Pipeline: Validar → Parsear XML → Clasificar por color → Calcular geometría → Agregar
- Parser: encoding/xml (stdlib)
- Curvas Bézier: subdivisión recursiva (tolerancia: 0.5mm, <1% error)
- Área raster: bounding box inicial
- Go puro, cero dependencias externas

## Comandos
```bash
make run                        # go run cmd/server/main.go
make build                      # go build -o bin/fabricalaser
make test                       # go test ./...
make lint                       # golangci-lint run
make migrate-up                 # Aplicar migraciones
make migrate-down               # Revertir última
make seed                       # Cargar datos simulador v5
cd web/admin && npm run dev     # Admin dev
cd web/wizard && npm run dev    # Wizard dev
make deploy                     # Build + restart service
sudo systemctl restart fabricalaser-api
sudo journalctl -u fabricalaser-api -f
```

## Variables de Entorno
```
FABRICALASER_PORT=8083
FABRICALASER_DB_HOST=localhost
FABRICALASER_DB_PORT=5432
FABRICALASER_DB_NAME=fabricalaser
FABRICALASER_DB_USER=fabricalaser
FABRICALASER_DB_PASSWORD=<configurar>
FABRICALASER_JWT_SECRET=<generar con openssl rand -hex 32>
FABRICALASER_REDIS_ADDR=localhost:6379
FABRICALASER_REDIS_DB=3
FABRICALASER_UPLOAD_DIR=/opt/FabricaLaser/uploads
FABRICALASER_MAX_FILE_SIZE=10485760
FABRICALASER_ENV=development

# GoMeta API (Validación Cédula CR)
FABRICALASER_GOMETA_TIMEOUT=10                    # Timeout en segundos (default: 10)
FABRICALASER_GOMETA_REQUIRE_VALIDATION=false      # Si true, falla registro cuando GoMeta offline
```

## Fase Actual: 3 — Órdenes y Operaciones (PENDIENTE)

**Estado del Proyecto (Actualizado: 2026-02-26):**
| Fase | Nombre | Estado |
|------|--------|--------|
| 0A | Estructura + DB + Seed | ✅ COMPLETADA |
| 0B | Sistema de Autenticación | ✅ COMPLETADA |
| 0C | API Config + Admin | ✅ COMPLETADA |
| 0D | Landing Page + Auth UI | ✅ COMPLETADA |
| 1 | Motor SVG + Pricing API | ✅ COMPLETADA |
| 2A | Wizard del Cliente | ✅ COMPLETADA |
| 2B | Panel Admin | ✅ COMPLETADA |
| 3 | Órdenes y Operaciones | ⏳ PENDIENTE |
| 4 | Pagos y Lanzamiento | ⏳ PENDIENTE |

**MVP Funcional: ✅ ALCANZADO** — Sistema de cotización operativo end-to-end.

**Completado 0A:**
- Proyecto Go con chi, gorm, pgx, bcrypt, jwt-go
- DB `fabricalaser` con 7 tablas
- Seed data: 4 techs, 7 materiales, 4 tipos grabado, tarifas, descuentos
- Admin: cedula=999999999, password=admin123

**Completado 0B:**
- 5 endpoints de autenticación funcionando
- Integración GoMeta API para validación de cédula CR
- Validación contra Registro Civil con pre-llenado de datos oficiales
- Metadata JSONB con datos de GoMeta para facturación electrónica
- Cache de 24 horas para consultas GoMeta
- Middleware de autenticación JWT
- Middleware de roles (admin)
- Tests del servicio de cédula

**Completado 0C:**
- 7 endpoints públicos de configuración (`/api/v1/config/*`)
- Endpoint `/api/v1/config` retorna toda la config en una llamada (para initial load del frontend)
- 18 endpoints admin para CRUD de configuración (`/api/v1/admin/*`)
- Control de acceso: 401 sin token, 403 para no-admin
- 6 repositorios de configuración (technology, material, engrave_type, tech_rate, volume_discount, price_reference)
- Soft delete (is_active=false) en lugar de borrado físico

**Archivos clave implementados:**
- `internal/services/cedula/cedula_service.go` — Cliente HTTP GoMeta API
- `internal/services/auth/auth_service.go` — Lógica con validación externa + UpdateProfile
- `internal/handlers/auth/auth_handler.go` — 7 endpoints auth (incluye profile GET/PUT)
- `internal/handlers/config/config_handler.go` — 7 endpoints config públicos
- `internal/handlers/admin/admin_handler.go` — 18 endpoints admin CRUD
- `internal/repository/*_repository.go` — 7 repositorios (user + 6 config)
- `internal/utils/cedula.go` — Validación formato local
- `web/landing/index.html` — Landing page con auth modal 4-estados
- `web/mi-cuenta/index.html` — Página perfil usuario con edición de direccion
- `migrations/008_user_profile_fields.sql` — Campos perfil CR (direccion, provincia, canton, distrito)

**Completado 0D:**
- Landing page HTML estática (`web/landing/index.html`)
- Auth modal con flujo 4 estados (cedula → registro/login/establecer password)
- Pagina `/mi-cuenta` para perfil de usuario (`web/mi-cuenta/index.html`)
- Endpoint `GET/PUT /api/v1/auth/profile` para perfil con direccion CR
- Campos perfil usuario: direccion, provincia, canton, distrito (migracion 008)
- Router sirve archivos estaticos (/, /mi-cuenta, /cotizar placeholder)
- "COTIZAR ONLINE" verifica auth antes de redirigir
- No se muestra "GoMeta" en UI - solo "Identidad verificada"

**Completado Fase 1:**
- Motor SVG (`internal/services/svgengine/`)
  - Parser: XML con encoding/xml, extracción de elementos (path, rect, circle, ellipse, line, polyline, polygon)
  - Classifier: Clasificación por color (rojo=corte, azul=vector, negro=raster) con tolerancia ±10%
  - Geometry: Cálculos de Bézier con subdivisión adaptativa, Shoelace formula para áreas
  - Analyzer: Orquestador que produce SVGAnalysis completo
- Motor Pricing (`internal/services/pricing/`)
  - ConfigLoader: Carga rates/materials/discounts de DB con cache 5min (NO hardcode)
  - TimeEstimator: Calcula tiempos de grabado/corte basado en geometría
  - Calculator: Implementa modelo híbrido + clasificación automática
- API Quotes (`internal/handlers/quote/`)
  - POST /api/v1/quotes/analyze — Subir y analizar SVG
  - POST /api/v1/quotes/calculate — Calcular precio con opciones
  - GET /api/v1/quotes/my — Mis cotizaciones
  - GET /api/v1/quotes/analyses — Mis análisis SVG
  - GET /api/v1/quotes/:id — Ver cotización específica
- Repositorios: svg_analysis_repository.go, quote_repository.go
- Migraciones: 009_svg_analyses.sql, 010_quotes.sql
- QuotaMiddleware integrado en rutas POST

**Principio clave:** Todos los parámetros de pricing vienen de DB (tech_rates, materials, engrave_types, volume_discounts). NO hay valores hardcodeados.

**Completado Fase 2A — Wizard del Cliente:**
- `web/cotizar/index.html` (2161 líneas) — Wizard 3 pasos completo
- Paso 1: Upload SVG con drag & drop, análisis automático
- Paso 2: Selección tecnología/material con matriz de compatibilidad
- Paso 3: Resultado con desglose completo (tiempos, factores, precios en CRC)
- Tab historial de cotizaciones anteriores
- Auth guard: redirige a landing si no autenticado

**Completado Fase 2B — Panel Admin:**
- `web/admin/index.html` — Dashboard con 4 métricas
- `web/admin/users.html` — CRUD usuarios con búsqueda/filtros/paginación
- `web/admin/quotes.html` — Gestión cotizaciones con modal detalle
- `web/admin/config/*.html` — 5 páginas CRUD configuración
- `web/admin/admin.js` (519 líneas) — Lógica compartida
- `web/admin/admin.css` (937 líneas) — Design system completo

**Siguiente:** Fase 3 — Órdenes y Flujo Operativo (tabla orders, estados, cola de producción).

## Notas para Claude Code
- Monolito modular. NO crear microservicios.
- Reutilizar patrones de /opt/Sorteos (Chi, GORM, middleware) y /opt/Payments (modelo auth por cédula).
- Sin sobreingeniería: mínimo funcional, iterar después.
- Cada archivo = una responsabilidad clara.
- Preguntar antes de decisiones arquitectónicas no definidas aquí.
- Tests obligatorios para services/ y svgengine/.
- Migraciones: SQL puro, numeradas (001_, 002_...).
- Los datos del simulador Excel v5 son la fuente de verdad para seed data.
- Landing page: HTML estático servido por Nginx, estilo consistente con otros sitios del servidor.
- Auth: cédula como identificador único, JWT para sesiones, bcrypt para passwords.
- Cuota: quote_quota=5 por defecto, -1 para ilimitado. Middleware valida antes de cotizar.
- **GoMeta API:** Validación de cédula contra Registro Civil CR. Usar nombre oficial para registro. Guardar en metadata.extras para facturación. Cache 24h. Timeout configurable (default 10s).
- **Config API:** Endpoint `/api/v1/config` retorna toda la configuración del cotizador en una llamada. Usar para initial load del frontend. Los datos son read-only para usuarios, solo admin puede modificar via `/api/v1/admin/*`.

## Reglas de Subagentes y Paralelismo

### Cuándo usar subagentes
Usar subagentes en paralelo siempre que una tarea tenga componentes independientes que toquen capas distintas (BD, API, Frontend, Tests). No hacer secuencial lo que puede ser paralelo.

### Regla del Contrato (OBLIGATORIO)
Antes de lanzar cualquier subagente:

1. **LEER** el código existente relacionado (modelos, handlers, rutas, migraciones)
2. **CREAR o ACTUALIZAR** el archivo `docs/CONTRACTS.md` con:
   - Esquema SQL exacto (tabla, columnas, tipos, constraints)
   - Struct Go exacto (campos, json tags, gorm tags)
   - Endpoints exactos (método, ruta, request body, response body)
   - Campos que consume el frontend (mapeo API → JS)
3. **GUARDAR** el contrato en disco ANTES de lanzar subagentes
4. Cada subagente recibe el contrato como contexto obligatorio
5. Ningún subagente puede inventar, renombrar o agregar campos fuera del contrato
6. Si un subagente necesita algo no definido → reportar, NO improvisar
7. Al terminar cada subagente → el orquestador VERIFICA consistencia vs contrato

### Qué va en el contrato
El contrato es la fuente de verdad. Define nombres exactos que deben coincidir entre BD ↔ Go struct ↔ JSON response ↔ Frontend. Ejemplo de consistencia:

| Capa | Nombre | Ejemplo |
|------|--------|---------|
| Columna SQL | `cut_speed` | `cut_speed DECIMAL(10,2)` |
| Go struct | `CutSpeed` | `CutSpeed float64 \`json:"cut_speed" gorm:"column:cut_speed"\`` |
| JSON response | `cut_speed` | `{ "cut_speed": 25.0 }` |
| Frontend JS | `cut_speed` | `speed.cut_speed` |

**Si alguna capa usa un nombre diferente, es un bug.**

### Ejemplo de contrato (docs/CONTRACTS.md)
```markdown
## Order (Fase 3)

### SQL
\`\`\`sql
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    quote_id INTEGER NOT NULL REFERENCES quotes(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    status VARCHAR(20) DEFAULT 'pending',
    operator_notes TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
\`\`\`

### Go Struct
\`\`\`go
type Order struct {
    ID            uint      `json:"id" gorm:"primaryKey"`
    QuoteID       uint      `json:"quote_id" gorm:"not null"`
    UserID        uint      `json:"user_id" gorm:"not null"`
    Status        string    `json:"status" gorm:"default:pending"`
    OperatorNotes string    `json:"operator_notes"`
    CreatedAt     time.Time `json:"created_at"`
    UpdatedAt     time.Time `json:"updated_at"`
}
\`\`\`

### Endpoints
- POST /api/v1/orders - Crear orden desde cotización
- GET /api/v1/orders/my - Mis órdenes
- PUT /api/v1/admin/orders/{id}/status - Cambiar estado (admin)

### Frontend consume
- order.id, order.status, order.quote_id
- No usar: order_id (incorrecto), order.quoteId (incorrecto)
```