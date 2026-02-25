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
- **User** — Usuario centralizado (cedula unique, cedula_type: fisica|juridica, name, email, phone, password_hash bcrypt, role: customer|admin, quote_quota default 5, quotes_used, status: active|suspended)
- **UserProfile** — Perfil extendido progresivo (user_id, address, provincia, canton, distrito, actividad_comercial, admin_notes)
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

## Fase Actual: 0D — Landing Page

**Fases de Fundación:**
| Fase | Nombre | Estado |
|------|--------|--------|
| 0A | Estructura + DB + Seed | ✅ COMPLETADA |
| 0B | Sistema de Autenticación | ✅ COMPLETADA |
| 0C | API Config + Admin | ✅ COMPLETADA |
| 0D | Landing Page | ⏳ Pendiente |

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
- `internal/services/auth/auth_service.go` — Lógica con validación externa
- `internal/handlers/auth/auth_handler.go` — 5 endpoints auth
- `internal/handlers/config/config_handler.go` — 7 endpoints config públicos
- `internal/handlers/admin/admin_handler.go` — 18 endpoints admin CRUD
- `internal/repository/*_repository.go` — 7 repositorios (user + 6 config)
- `internal/utils/cedula.go` — Validación formato local

**Siguiente:** Fase 0D — Landing Page (HTML estático, Nginx).

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