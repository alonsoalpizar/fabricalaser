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
│   │   ├── quotes/
│   │   ├── materials/
│   │   ├── orders/
│   │   ├── users/              # Gestión usuarios (admin)
│   │   └── admin/
│   ├── services/
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

**Endpoints Auth:**
| Endpoint | Método | Auth | Descripción |
|----------|--------|------|-------------|
| `/api/v1/auth/verificar-cedula` | POST | No | `{identificacion}` → `{existe, tienePassword, tipo, cedula}` |
| `/api/v1/auth/registro` | POST | No | `{identificacion, nombre, email, telefono, password}` → `{token, usuario}` |
| `/api/v1/auth/login` | POST | No | `{identificacion, password}` → `{token, usuario}` |
| `/api/v1/auth/establecer-password` | POST | No | `{identificacion, password, email?, telefono?}` → `{token, usuario}` |
| `/api/v1/auth/me` | GET | JWT | `→ {usuario}` |

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
```

## Fase Actual: 0A — Estructura y Base de Datos

**Fases de Fundación:**
| Fase | Nombre | Estado |
|------|--------|--------|
| 0A | Estructura + DB + Seed | 🔄 EN PROGRESO |
| 0B | Sistema de Autenticación | ⏳ Pendiente |
| 0C | API Config + Servidor | ⏳ Pendiente |
| 0D | Landing Page | ⏳ Pendiente |

**Objetivo 0A:** Proyecto Go inicializado, DB PostgreSQL con migraciones, seed data del simulador v5.
**Siguiente:** Fase 0B — Auth por cédula (replicar /opt/Payments).

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