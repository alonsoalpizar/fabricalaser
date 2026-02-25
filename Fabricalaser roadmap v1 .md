# FabricaLaser.com — Roadmap Técnico y Plan de Desarrollo

**Versión 1.0 | Febrero 2026**  
**Ing. Alonso Alpízar**

---

## 1. Resumen Ejecutivo

FabricaLaser.com es una plataforma de cotización automática, venta online y gestión operativa para servicios de corte y grabado láser. El sistema analiza archivos SVG, extrae métricas geométricas, aplica reglas de pricing paramétricas y genera cotizaciones instantáneas.

El proyecto nace de un problema real: en Costa Rica, el 80% de los talleres láser trabaja por cotización manual sin publicar precios. FabricaLaser.com rompe ese modelo ofreciendo precios transparentes, cotización instantánea y diferenciación técnica (vector vs fotograbado, ventaja UV).

### 1.1 Problema

- Cotizaciones manuales que toman horas — el operador debe calcular tiempos, costos y márgenes a mano
- Grabado poco rentable por imprevisibilidad en tiempos (vectorial vs raster vs fotograbado varían 1x a 6x)
- Dependencia total del conocimiento del operador para estimar trabajos
- Ningún competidor en CR publica precios ni diferencia tipos de grabado
- Imposibilidad de escalar sin multiplicar personal calificado

### 1.2 Oportunidad de Mercado (Análisis de Competencia CR)

Del análisis de 10 competidores identificados en Costa Rica:

- 80% trabaja solo por cotización — NO publican precios
- NADIE diferencia explícitamente Vector vs Fotograbado (factor de 1x a 2.5x en tiempo)
- Pocos mencionan tecnología UV como ventaja competitiva

**Oportunidad clara:** publicar precios transparentes, comunicar diferencia técnica, y posicionar ventaja UV en vidrio/cristal como premium.

### 1.3 Estrategia Anti-Competencia y Registro

El sistema de cotización automática es un activo valioso. Para evitar que competidores lo usen para cotizar sus propios trabajos, se implementa un registro obligatorio por cédula (física o jurídica) con cuota limitada de cotizaciones.

- **Registro por cédula:** Cédula física (9 dígitos) o jurídica (10 dígitos) como identificador único. Modelo idéntico a pagar.alonsoalpizar.com.
- **Cuota inicial:** 5 cotizaciones gratuitas al registrarse. Suficiente para probar el sistema y generar interés.
- **Extensión desde admin:** Una vez establecida la relación comercial, el operador extiende la cuota desde backoffice (puede ser un valor N o cotizaciones ilimitadas).
- **Identificación real:** La cédula permite identificar exactamente quién cotiza. Si un competidor se registra, queda identificado.

### 1.4 Arquitectura Web (3 Capas)

| Capa | URL | Acceso | Propósito |
|------|-----|--------|-----------|
| **Landing Page** | fabricalaser.com | Público (sin registro) | Cara del negocio: servicios, portafolio, diferenciación UV, CTA a cotizar |
| **Cotizador** | fabricalaser.com/cotizar | Registro requerido (cédula + password) | Wizard de cotización SVG, historial de cotizaciones |
| **Admin / Backoffice** | fabricalaser.com/admin | Solo rol ADMIN | Gestión completa: usuarios, cotizaciones, órdenes, tarifas, cuotas |

### 1.5 Modelo de Usuarios y Autenticación

Sistema centralizado de usuarios **idéntico al modelo de pagar.alonsoalpizar.com** (`/opt/Payments`). Registro simple sin doble factor de autenticación (fase inicial).

#### 1.5.1 Campos del Usuario

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `id` | SERIAL | ID único autoincremental |
| `cedula` | VARCHAR(10) UNIQUE | Cédula física (9 dígitos) o jurídica (10 dígitos) |
| `cedula_type` | VARCHAR(10) | `fisica` o `juridica` |
| `nombre` | VARCHAR(100) | Nombre completo o razón social |
| `apellido` | VARCHAR(100) | Apellido(s) - puede ser NULL para jurídicas |
| `email` | VARCHAR(255) | Email único para notificaciones |
| `telefono` | VARCHAR(20) | Teléfono de contacto |
| `password_hash` | VARCHAR(255) | Hash bcrypt de la contraseña |
| `role` | VARCHAR(20) | `customer` o `admin` |
| `quote_quota` | INTEGER | Cuota de cotizaciones (default: 5, -1 = ilimitado) |
| `quotes_used` | INTEGER | Cotizaciones consumidas |
| `activo` | BOOLEAN | Cuenta activa/suspendida (default: true) |
| `ultimo_login` | TIMESTAMP | Último inicio de sesión |
| `metadata` | JSONB | Datos adicionales (ej: datos de Hacienda) |
| `created_at` | TIMESTAMP | Fecha de creación |
| `updated_at` | TIMESTAMP | Última modificación |

#### 1.5.2 Validación de Cédula CR

**Mismo esquema que Payments:**
- **Física:** 9 dígitos exactos, no empieza con 0 (ej: `123456789`)
- **Jurídica:** 10 dígitos exactos, no empieza con 0 (ej: `3101234567`)
- Validación regex: `^[1-9]\d{8}$` (física) o `^[1-9]\d{9}$` (jurídica)
- Limpieza automática: eliminar guiones, espacios, caracteres no numéricos

**Integración GoMeta API (opcional pero recomendada):**
- Validar cédulas contra Registro Civil / Hacienda de Costa Rica
- Auto-completar nombre/apellido desde datos oficiales
- Almacenar datos de Hacienda en `metadata` para facturación electrónica

#### 1.5.3 Endpoints de Autenticación

Replicar exactamente el flujo de `/opt/Payments/backend/src/`:

| Endpoint | Método | Auth | Descripción |
|----------|--------|------|-------------|
| `/api/v1/auth/verificar-cedula` | POST | No | Verifica si cédula existe y si tiene password |
| `/api/v1/auth/login` | POST | No | Login con cédula + password → JWT |
| `/api/v1/auth/registro` | POST | No | Registro nuevo usuario (asigna cuota=5) |
| `/api/v1/auth/establecer-password` | POST | No | Establece password para usuario creado por admin |
| `/api/v1/auth/me` | GET | JWT | Retorna datos del usuario autenticado |

**Flujo de Verificación de Cédula (pre-login/registro):**
```json
POST /api/v1/auth/verificar-cedula
{ "identificacion": "123456789" }

// Respuesta si existe con password:
{ "existe": true, "tienePassword": true, "tipo": "fisica", "cedula": "123456789" }

// Respuesta si existe sin password (creado por admin):
{ "existe": true, "tienePassword": false, "tipo": "fisica", "cedula": "123456789", "cliente": {...} }

// Respuesta si no existe:
{ "existe": false, "tienePassword": false, "tipo": "fisica", "cedula": "123456789" }
```

**Flujo de Registro:**
```json
POST /api/v1/auth/registro
{
  "identificacion": "123456789",
  "nombre": "Juan Pérez",
  "email": "juan@ejemplo.com",
  "telefono": "88887777",
  "password": "miPassword123"
}

// Respuesta exitosa:
{ "token": "eyJhbG...", "usuario": { "id": 1, "cedula": "123456789", ... } }
```

**Flujo de Login:**
```json
POST /api/v1/auth/login
{ "identificacion": "123456789", "password": "miPassword123" }

// Respuesta exitosa:
{ "token": "eyJhbG...", "usuario": { "id": 1, "cedula": "123456789", ... } }
```

#### 1.5.4 JWT Token

- Algoritmo: HS256
- Expiración: 24 horas
- Payload: `{ id, cedula, nombre, email, role, tipo: "customer" }`
- Header: `Authorization: Bearer <token>`

#### 1.5.5 Perfil Progresivo (post-registro)

- Dirección completa (provincia, cantón, distrito)
- Actividad comercial (para jurídicas)
- Notas internas (solo visibles para admin)
- Historial de cotizaciones y órdenes

#### 1.5.6 Roles

| Rol | Permisos | Registro |
|-----|----------|----------|
| **customer** | Cotizar (hasta su cuota), ver historial, editar perfil, crear órdenes | Self-register vía /cotizar |
| **admin** | Todo: gestionar usuarios, cuotas, cotizaciones, órdenes, tarifas, materiales, tecnologías | Creado manualmente o vía seed |

### 1.6 Propuesta de Valor

Un sistema tipo "Ponoko local" para LATAM: el cliente sube un SVG, el sistema detecta operaciones por convenciones de color (rojo=corte, azul=grabado vector, negro=grabado raster), calcula métricas geométricas, estima tiempos y genera un precio automáticamente. Trabajos simples se auto-aprueban; complejos pasan a revisión humana. Soporta multi-tecnología (CO2, UV, Fibra, MOPA) desde el inicio.

---

## 2. Stack Técnico

### 2.1 Arquitectura General

Monolito modular en Go, consistente con el ecosistema existente del servidor (Sorteos, CalleViva). Un solo binario desplegable con separación interna clara por módulos.

| Capa | Tecnología | Justificación |
|------|-----------|---------------|
| Backend API | Go 1.22 + Chi router | Consistente con stack, alto rendimiento |
| Base de Datos | PostgreSQL 16 | DB dedicada `fabricalaser` en instancia compartida |
| Cache | Redis 7 | Cache cotizaciones, sesiones, rate limiting |
| Frontend | React + TypeScript | Admin y Wizard, componentes compartidos |
| Motor SVG | Go puro (encoding/xml + math) | Un solo binario, sin deps externas |
| Web Server | Nginx 1.24 (reverse proxy) | SSL, proxy a :8083, static files |
| Almacenamiento | Filesystem local | SVGs en /opt/FabricaLaser/uploads |

### 2.2 Estructura del Proyecto

**Ubicación:** `/opt/FabricaLaser` | **Puerto API:** 8083 | **DB:** fabricalaser | **Dominio:** fabricalaser.com

```
/opt/FabricaLaser/
├── cmd/server/main.go
├── internal/
│   ├── config/                 # Variables de entorno, configuración
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
│   ├── middleware/              # Auth JWT, CORS, logging, rate limit, quota
│   └── repository/             # Capa acceso a datos
├── web/
│   ├── landing/                # HTML estático — cara pública
│   ├── admin/                  # React app admin (rol ADMIN)
│   └── wizard/                 # React app wizard (requiere auth)
├── migrations/                 # SQL (001_, 002_...)
├── uploads/                    # SVGs subidos (no commitear)
├── scripts/
├── CLAUDE.md
├── go.mod / go.sum / Makefile
```

---

## 3. Modelo de Datos Core

Soporta multi-tecnología desde el inicio (CO2, UV, Fibra, MOPA). El modelo refleja directamente la estructura del simulador existente, digitalizando los parámetros que hoy viven en Excel. **Todos los valores son editables desde el admin (CRUD dinámico).** El seed data solo carga valores iniciales del simulador v5.

### 3.1 Entidades Principales

| Entidad | Propósito | Campos Clave |
|---------|-----------|-------------|
| **User** | Usuario centralizado (auth + perfil) | cedula (unique), cedula_type (fisica\|juridica), name, email, phone, password_hash, role (customer\|admin), quote_quota, quotes_used, status |
| **UserProfile** | Perfil extendido (progresivo) | user_id, address, provincia, canton, distrito, actividad_comercial, admin_notes |
| **Technology** | Tipo láser: CO2, UV, Fibra, MOPA | code, name, description, uv_premium_factor, is_active |
| **Material** | Material físico con factor de ajuste | name, category, factor (1.0-1.8), thicknesses[], notes |
| **EngraveType** | Tipo de grabado con factor de tiempo | name, factor (1.0-3.0), speed_multiplier, description |
| **TechRate** | Tarifas base por tecnología | tech_id, engrave_rate_hour, cut_rate_hour, design_rate_hour, setup_fee, cost_per_min_engrave, cost_per_min_cut |
| **SVGAnalysis** | Resultado del análisis geométrico | file_hash, cut_length_mm, vector_length_mm, raster_area_mm2, element_count, bounding_box, warnings[] |
| **Quote** | Cotización generada (modelo híbrido) | user_id, analysis_id, tech_id, material_id, engrave_type_id, quantity, time_engrave_min, time_cut_min, cost_base, adjustments{}, price_hybrid, price_value, status |
| **VolumeDiscount** | Descuentos por cantidad | min_qty, max_qty, discount_pct (0-0.20) |
| **Order** | Orden de fabricación | quote_id, user_id, status, payment_status, operator_notes |
| **PriceReference** | Tabla de precios de referencia | service_type, min_usd, max_usd, typical_time, description |

### 3.2 Relaciones

- User 1:1 UserProfile
- User 1:N Quote (con control de cuota: quotes_used < quote_quota)
- User 1:N Order
- Technology 1:N TechRate
- SVGAnalysis 1:N Quote
- Quote 1:1 Order

### 3.3 Datos Iniciales (del Simulador v5)

Estos valores se cargan como seed data y luego son **100% editables desde el admin**.

**Factores por Material:**

| Material | Factor | Nota |
|----------|--------|------|
| Madera / MDF | 1.0 | Material base de referencia |
| Acrílico transparente | 1.2 | Calibración especial requerida |
| Plástico ABS/PC | 1.25 | Configuración especial |
| Cuero / Piel | 1.3 | Material premium |
| Vidrio / Cristal | 1.5 | Alto riesgo, UV ideal |
| Cerámica | 1.6 | Material delicado |
| Metal con coating | 1.8 | Máxima precisión requerida |

**Factores por Tipo de Grabado:**

| Tipo | Factor | Descripción | Velocidad Relativa |
|------|--------|-------------|-------------------|
| Vectorial (líneas) | 1.0 | Logos, texto, contornos | Rápido (1x) |
| Rasterizado simple | 1.5 | Áreas sólidas, rellenos | Medio (2x) |
| Fotograbado (fotos) | 2.5 | Imágenes con degradados | Lento (4-5x) |
| 3D / Relieve | 3.0 | Múltiples pasadas | Muy lento (6x+) |

**Tarifas Base (UV):**

| Concepto | Valor | Unidad |
|----------|-------|--------|
| Tarifa operador GRABADO | $12.00 | USD/hora |
| Tarifa operador CORTE | $14.00 | USD/hora |
| Tarifa diseño | $15.00 | USD/hora |
| Costo fijo por hora (overhead) | $3.78 | USD/hora |
| Costo total/hora GRABADO (tarifa + fijos) | $15.78 | USD/hora |
| Costo total/hora CORTE (tarifa + fijos) | $17.78 | USD/hora |
| Costo por minuto GRABADO | $0.263 | USD/minuto |
| Costo por minuto CORTE | $0.296 | USD/minuto |
| Margen ganancia recomendado | 40% | |
| Premium UV | 15-25% | Sobre precio base |

**Descuentos por Volumen:**

| Cantidad | Descuento |
|----------|-----------|
| 1 - 9 piezas | 0% |
| 10 - 24 piezas | 5% |
| 25 - 49 piezas | 10% |
| 50 - 99 piezas | 15% |
| 100+ piezas | 20% |

**Precios de Referencia:**

| Servicio | Mín USD | Máx USD | Tiempo Típico |
|----------|---------|---------|---------------|
| Grabado básico (<5cm²) | $3 | $10 | 1-3 min |
| Grabado estándar (5-15cm²) | $10 | $25 | 3-8 min |
| Grabado complejo (15-30cm²) | $25 | $50 | 8-15 min |
| Fotograbado | $40 | $100 | 15-40 min |
| Corte simple (<20cm) | $2 | $8 | 0.5-2 min |
| Corte complejo (>20cm) | $8 | $25 | 2-8 min |
| Corte + Grabado | $8 | $40 | 3-15 min |

---

## 4. Roadmap por Fases

Cada fase es funcional e independiente. No se avanza a la siguiente sin que la actual esté estable y validada.

---

### FASE 0A: Estructura y Base de Datos

**Objetivo:** Crear el esqueleto del proyecto Go y la base de datos con todos los modelos y seed data.

**Entregables:**

1. **Proyecto Go inicializado** (`/opt/FabricaLaser/`)
   ```
   /opt/FabricaLaser/
   ├── cmd/server/main.go          # Entry point
   ├── internal/
   │   ├── config/config.go        # Env vars, configuración
   │   ├── models/                  # Structs GORM
   │   │   ├── user.go
   │   │   ├── technology.go
   │   │   ├── material.go
   │   │   ├── engrave_type.go
   │   │   ├── tech_rate.go
   │   │   ├── volume_discount.go
   │   │   └── price_reference.go
   │   ├── repository/              # Capa acceso DB
   │   └── database/db.go           # Conexión PostgreSQL
   ├── migrations/
   │   ├── 001_users.sql
   │   ├── 002_technologies.sql
   │   ├── 003_materials.sql
   │   ├── 004_engrave_types.sql
   │   ├── 005_tech_rates.sql
   │   ├── 006_volume_discounts.sql
   │   └── 007_price_references.sql
   ├── seeds/
   │   └── 001_initial_data.sql     # Datos del simulador v5
   ├── go.mod / go.sum
   ├── Makefile
   ├── .env.example
   └── CLAUDE.md
   ```

2. **Base de datos PostgreSQL** `fabricalaser`
   - Tabla `users` con todos los campos (sección 1.5.1)
   - Índice único parcial: `CREATE UNIQUE INDEX ON users(cedula) WHERE password_hash IS NOT NULL`
   - Tablas: technologies, materials, engrave_types, tech_rates, volume_discounts, price_references

3. **Seed data del simulador v5**
   - 4 tecnologías: CO2, UV, Fibra, MOPA
   - 7 materiales con factores (1.0 - 1.8)
   - 4 tipos de grabado con factores (1.0 - 3.0)
   - Tarifas base UV ($12-15/hora)
   - 5 rangos de descuento por volumen (0% - 20%)
   - 7 precios de referencia
   - Usuario admin: cedula=`999999999`, password=`admin123`, role=`admin`

**Comandos a implementar:**
```bash
make init          # go mod init + deps
make migrate-up    # Aplica migraciones
make migrate-down  # Revierte última migración
make seed          # Carga seed data
make db-reset      # Drop + create + migrate + seed
```

**Criterio de Éxito:**
```bash
# DB existe y tiene datos
psql -d fabricalaser -c "SELECT COUNT(*) FROM technologies"  # 4
psql -d fabricalaser -c "SELECT COUNT(*) FROM materials"     # 7
psql -d fabricalaser -c "SELECT COUNT(*) FROM users WHERE role='admin'"  # 1

# Proyecto compila
cd /opt/FabricaLaser && go build ./...  # Sin errores
```

---

### FASE 0B: Sistema de Autenticación

**Objetivo:** Implementar auth por cédula **idéntico a /opt/Payments**, con JWT y middleware.

**Dependencia:** Fase 0A completada.

**Entregables:**

1. **Estructura de archivos:**
   ```
   internal/
   ├── handlers/auth/
   │   └── auth_handler.go         # Handlers HTTP
   ├── services/auth/
   │   └── auth_service.go         # Lógica de negocio
   ├── middleware/
   │   ├── auth.go                 # JWT middleware
   │   ├── role.go                 # Role middleware (admin)
   │   └── quota.go                # Quota middleware (cotizaciones)
   └── utils/
       ├── jwt.go                  # Generar/verificar tokens
       ├── password.go             # bcrypt hash/compare
       └── cedula.go               # Validación cédula CR
   ```

2. **Endpoints de Auth** (replicar flujo de Payments):

   | Endpoint | Método | Body | Response |
   |----------|--------|------|----------|
   | `/api/v1/auth/verificar-cedula` | POST | `{identificacion}` | `{existe, tienePassword, tipo, cedula}` |
   | `/api/v1/auth/registro` | POST | `{identificacion, nombre, email, telefono, password}` | `{token, usuario}` |
   | `/api/v1/auth/login` | POST | `{identificacion, password}` | `{token, usuario}` |
   | `/api/v1/auth/establecer-password` | POST | `{identificacion, password, email?, telefono?}` | `{token, usuario}` |
   | `/api/v1/auth/me` | GET | — (JWT header) | `{usuario}` |

3. **Validación de Cédula CR:**
   ```go
   // Física: 9 dígitos, no empieza con 0
   var cedulaFisicaRegex = regexp.MustCompile(`^[1-9]\d{8}$`)

   // Jurídica: 10 dígitos, no empieza con 0
   var cedulaJuridicaRegex = regexp.MustCompile(`^[1-9]\d{9}$`)
   ```

4. **JWT Token:**
   - Algoritmo: HS256
   - Expiración: 24 horas
   - Secret: `FABRICALASER_JWT_SECRET` (env var)
   - Payload: `{id, cedula, nombre, email, role, tipo: "customer"}`

5. **Middleware Stack:**
   - `AuthMiddleware`: Extrae y valida JWT del header `Authorization: Bearer <token>`
   - `RoleMiddleware(role)`: Verifica que `req.User.Role == role`
   - `QuotaMiddleware`: Verifica `quotes_used < quote_quota` (o `quote_quota == -1`)

6. **bcrypt:** cost=12 para password_hash

**Criterio de Éxito:**
```bash
# 1. Verificar cédula (no existe)
curl -X POST http://localhost:8083/api/v1/auth/verificar-cedula \
  -H "Content-Type: application/json" \
  -d '{"identificacion": "123456789"}'
# → {"data": {"existe": false, "tienePassword": false, "tipo": "fisica", "cedula": "123456789"}}

# 2. Registro
curl -X POST http://localhost:8083/api/v1/auth/registro \
  -H "Content-Type: application/json" \
  -d '{"identificacion": "123456789", "nombre": "Test User", "email": "test@test.com", "telefono": "88881234", "password": "test1234"}'
# → {"data": {"token": "eyJ...", "usuario": {..., "quote_quota": 5}}}

# 3. Login
curl -X POST http://localhost:8083/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"identificacion": "123456789", "password": "test1234"}'
# → {"data": {"token": "eyJ...", "usuario": {...}}}

# 4. Me (con token)
curl http://localhost:8083/api/v1/auth/me \
  -H "Authorization: Bearer eyJ..."
# → {"data": {"usuario": {...}}}

# 5. Me sin token → 401
curl http://localhost:8083/api/v1/auth/me
# → {"error": {"code": "UNAUTHORIZED", "message": "Token requerido"}}
```

---

### FASE 0C: API de Configuración + Servidor

**Objetivo:** Endpoints públicos para leer configuración (materiales, tecnologías, etc.) y despliegue en servidor.

**Dependencia:** Fase 0B completada.

**Entregables:**

1. **Endpoints de configuración (públicos, solo lectura):**

   | Endpoint | Método | Response |
   |----------|--------|----------|
   | `/api/v1/health` | GET | `{"status": "ok", "version": "1.0.0"}` |
   | `/api/v1/technologies` | GET | `[{id, code, name, uv_premium_factor, is_active}]` |
   | `/api/v1/materials` | GET | `[{id, name, category, factor, thicknesses, notes}]` |
   | `/api/v1/engrave-types` | GET | `[{id, name, factor, speed_multiplier, description}]` |
   | `/api/v1/volume-discounts` | GET | `[{min_qty, max_qty, discount_pct}]` |
   | `/api/v1/price-references` | GET | `[{service_type, min_usd, max_usd, typical_time}]` |

2. **Estructura handlers:**
   ```
   internal/handlers/
   ├── auth/auth_handler.go
   ├── config/
   │   ├── technology_handler.go
   │   ├── material_handler.go
   │   ├── engrave_type_handler.go
   │   └── config_handler.go       # health, volume-discounts, price-refs
   └── router.go                    # Chi router con todas las rutas
   ```

3. **Servidor:**
   - **Nginx** (`/etc/nginx/sites-available/fabricalaser.com`):
     ```nginx
     server {
         listen 80;
         server_name fabricalaser.com www.fabricalaser.com;

         location /api/ {
             proxy_pass http://127.0.0.1:8083;
             proxy_set_header Host $host;
             proxy_set_header X-Real-IP $remote_addr;
         }

         location / {
             root /opt/FabricaLaser/web/landing;
             index index.html;
         }
     }
     ```

   - **systemd** (`/etc/systemd/system/fabricalaser-api.service`):
     ```ini
     [Unit]
     Description=FabricaLaser API
     After=network.target postgresql.service

     [Service]
     Type=simple
     User=www-data
     WorkingDirectory=/opt/FabricaLaser
     ExecStart=/opt/FabricaLaser/bin/fabricalaser-api
     EnvironmentFile=/opt/FabricaLaser/.env
     Restart=always

     [Install]
     WantedBy=multi-user.target
     ```

   - **.env** con todas las variables requeridas

4. **Makefile completo:**
   ```makefile
   build:
   	go build -o bin/fabricalaser-api cmd/server/main.go

   run:
   	go run cmd/server/main.go

   deploy:
   	make build
   	sudo systemctl restart fabricalaser-api
   ```

**Criterio de Éxito:**
```bash
# API responde
curl http://localhost:8083/api/v1/health
# → {"status": "ok"}

curl http://localhost:8083/api/v1/technologies | jq length
# → 4

curl http://localhost:8083/api/v1/materials | jq length
# → 7

# Servicio corre
sudo systemctl status fabricalaser-api
# → Active: active (running)

# Nginx configurado
sudo nginx -t
# → syntax is ok
```

---

### FASE 0D: Landing Page

**Objetivo:** Página pública de FabricaLaser.com con información del negocio y CTA a cotizar.

**Dependencia:** Fase 0C completada (Nginx configurado).

**Entregables:**

1. **Landing page HTML estática** (`/opt/FabricaLaser/web/landing/`)
   ```
   web/landing/
   ├── index.html
   ├── css/
   │   └── styles.css
   ├── js/
   │   └── main.js (mínimo, scroll suave, etc.)
   └── img/
       ├── logo.svg
       ├── hero.jpg
       └── portfolio/
   ```

2. **Secciones de la landing:**
   - **Hero:** Título + CTA "Cotizar Ahora" → `/cotizar`
   - **Servicios:** Corte, Grabado Vector, Grabado Raster, Fotograbado
   - **Tecnologías:** CO2, UV (destacar ventaja UV en vidrio)
   - **Materiales:** Lista con iconos
   - **Portafolio:** Galería de trabajos (placeholder inicial)
   - **Precios:** "Cotización instantánea" + CTA
   - **Contacto:** Email, teléfono, ubicación
   - **Footer:** Links, redes sociales

3. **Estilo:** Consistente con otros sitios del servidor (colores, tipografía).

4. **SSL:** Configurar Let's Encrypt para fabricalaser.com

**Criterio de Éxito:**
```bash
# Landing visible
curl -I https://fabricalaser.com
# → HTTP/2 200

# CTA apunta a /cotizar
curl -s https://fabricalaser.com | grep -o 'href="/cotizar"'
# → href="/cotizar"
```

---

### FASE 1: Motor SVG + Cotizador Core

**Duración:** 3-5 sesiones

El corazón del sistema. Analizar SVGs, extraer métricas y generar cotizaciones usando el modelo híbrido del simulador.

**1A — Motor de Análisis SVG** (`internal/services/svgengine/`)
- Parser SVG en Go puro (encoding/xml)
- Clasificación por color: rojo (#FF0000 stroke) = corte, azul (#0000FF stroke) = grabado vector, negro (#000000 fill) = grabado raster
- Cálculo de longitud de paths (líneas rectas + curvas Bézier por subdivisión recursiva, tolerancia 0.5mm)
- Cálculo de área raster (bounding box como aproximación inicial)
- Validación: formato SVG, colores permitidos, tamaño máximo (10MB)
- Output: struct SVGAnalysis con todas las métricas y warnings

**1B — Motor de Pricing** (`internal/services/pricing/`)

Implementa el modelo híbrido del simulador con dos cálculos paralelos:

- **Modelo Híbrido (costo+margen):** Costo_Base + Margen(40%) + Ajuste_Material + Ajuste_TipoGrabado + Premium_UV
- **Modelo por Valor:** Precio_base_pieza × cantidad − descuento_volumen + cargo_diseño
- El operador ve ambos modelos y elige, o el sistema usa el mayor como precio sugerido
- Aplicación de factores: material (1.0-1.8), tipo grabado (1.0-3.0), premium UV (15-25%)
- Descuentos por volumen automáticos (5%-20% según tabla)
- Clasificación: auto_approved | needs_review | rejected (umbrales configurables)

**1C — API de Cotización**
- `POST /api/v1/quotes/analyze` — sube SVG, retorna SVGAnalysis (requiere auth, consume cuota)
- `POST /api/v1/quotes/calculate` — analysis + material + tech + tipo grabado + cantidad = cotización dual
- `GET /api/v1/quotes/:id` — detalle con ambos modelos de precio
- `GET /api/v1/quotes/my` — historial de cotizaciones del usuario autenticado
- `GET /api/v1/materials` — lista con factores y compatibilidad (público)
- `GET /api/v1/engrave-types` — tipos de grabado con factores (público)
- Middleware de cuota: valida quotes_used < quote_quota antes de permitir cotización

**Criterio de Éxito:** Subir un SVG real del taller y recibir cotización dual (híbrido + valor) con desglose completo en < 2 segundos. Validar que los números coinciden con el simulador Excel para los mismos parámetros.

---

### FASE 2: Frontend — Wizard + Admin

**Duración:** 3-5 sesiones

**2A — Wizard del Cliente** (`web/wizard/`)
- Paso 1: Subir SVG (drag & drop) con validación visual instantánea
- Paso 2: Preview SVG con capas coloreadas identificadas visualmente
- Paso 3: Selección de tecnología, material y tipo de grabado (filtrado por compatibilidad)
- Paso 4: Cantidad de piezas con descuento por volumen visible en tiempo real
- Paso 5: Cotización instantánea con desglose (tiempos, costos, ajustes)
- Paso 6: Guardar cotización / Solicitar orden (sin pago en esta fase)
- Guía educativa integrada: tooltips sobre colores SVG, tipos de grabado, y preparación de archivos

**2B — Panel Admin** (`web/admin/`)
- Dashboard: cotizaciones del día, pendientes revisión, órdenes activas, métricas, usuarios nuevos
- Gestión Usuarios: lista, detalle, ver cédula, ajustar cuota de cotizaciones (extender o ilimitar), cambiar estado, notas internas
- CRUD: Tecnologías, Materiales (con factores), Tipos de Grabado, Tarifas
- Gestión Cotizaciones: lista, detalle, aprobar/rechazar, override de precio, ver ambos modelos
- Vista del SVGAnalysis con métricas geométricas
- Tabla de precios de referencia (editable, del simulador)

**Criterio de Éxito:** Cliente sube SVG, selecciona opciones, ve cotización y la guarda. Operador ve todas las cotizaciones, aprueba/rechaza, ajusta tarifas y factores desde el admin.

---

### FASE 3: Órdenes y Flujo Operativo

**Duración:** 2-3 sesiones

- Órdenes de fabricación: cotización aprobada se convierte en orden
- Flujo de estados: pending → confirmed → in_production → completed → delivered
- Gestión de clientes: registro, historial, órdenes recurrentes
- Cola de producción para el operador con prioridad y estados
- Notificaciones email en cambios de estado (vía Postfix local)
- Notas internas del operador por orden

**Criterio de Éxito:** Flujo completo: cliente cotiza, operador aprueba, se genera orden, se mueve por estados hasta entrega. El operador tiene visibilidad completa de la cola de producción.

---

### FASE 4: Pagos y Lanzamiento Público

**Duración:** 2-4 sesiones

- Integración SINPE Móvil (manual o automatizada)
- Integración tarjeta (Stripe / gateway local)
- Checkout en wizard para trabajos auto-aprobados
- Dominio fabricalaser.com con SSL
- Plantillas SVG predefinidas para clientes sin archivos propios
- Analítica: cotizaciones/día, conversión, revenue, materiales populares
- Rate limiting y hardening de seguridad

**Criterio de Éxito:** Un cliente externo puede entrar a fabricalaser.com, cotizar, pagar y generar una orden sin intervención del operador (para trabajos auto-aprobados).

---

## 5. Detalle Técnico: Motor SVG

Componente más crítico del sistema. Go puro, cero dependencias externas.

### 5.1 Pipeline

| # | Operación | Input | Output |
|---|-----------|-------|--------|
| 1 | Validación | Archivo raw bytes | SVG válido o error |
| 2 | Parsing XML | SVG válido | Árbol de elementos |
| 3 | Clasificación color | Elementos + atributos | cut[], vector[], raster[] |
| 4 | Geometría | Grupos clasificados | Longitudes mm, Áreas mm² |
| 5 | Agregación | Métricas individuales | SVGAnalysis completo |

### 5.2 Convenciones de Color (Estándar del Sistema)

| Color | Hex | Atributo SVG | Operación | Métrica |
|-------|-----|-------------|-----------|---------|
| Rojo | #FF0000 | stroke | Corte | Longitud mm |
| Azul | #0000FF | stroke | Grabado Vector | Longitud mm |
| Negro | #000000 | fill | Grabado Raster | Área mm² |

### 5.3 Bézier y Librerías

Curvas Bézier cúbicas: subdivisión recursiva con tolerancia 0.5mm (< 1% error). Arcos SVG: conversión a Bézier cúbico (patrón estándar). Librerías Go a evaluar: `srwiley/oksvg` (path parsing), `tdewolff/canvas` (geometría). Alternativa: implementación propia para máximo control y cero dependencias.

---

## 6. Modelo de Pricing (del Simulador v5)

El sistema implementa el modelo híbrido del simulador existente, que calcula dos precios paralelos y permite al operador elegir el más conveniente.

### 6.1 Modelo Híbrido (Costo + Margen)

```
Costo_Base = Costo_Tiempo_Grabado + Costo_Tiempo_Corte + Costo_Material + Costo_Preparación + Costo_Setup

Costo_Tiempo_Grabado = Tiempo_Grabado_min × $0.263/min  (costo total/min grabado)
Costo_Tiempo_Corte   = Tiempo_Corte_min   × $0.296/min  (costo total/min corte)
Costo_Preparación    = Tiempo_Prep_min    × $0.250/min  (tarifa diseño)
Costo_Setup          = Tiempo_Setup_min   × $0.263/min  (tarifa grabado)

Precio_Híbrido = Costo_Base
               + (Costo_Base × Margen_40%)
               + (Costo_Base × (Factor_Material - 1.0))
               + (Costo_Base × (Factor_TipoGrabado - 1.0))
               + (Costo_Base × Premium_UV)
```

### 6.2 Modelo por Valor

```
Precio_Valor = (Precio_Base_Pieza × Cantidad)
             - Descuento_Volumen
             + Cargo_Diseño
```

El precio base por pieza se define manualmente o se sugiere desde la tabla de precios de referencia.

### 6.3 Clasificación Automática

| Estado | Condiciones | Acción |
|--------|------------|--------|
| **AUTO_APPROVED** | SVG limpio, < N elementos, sin raster pesado, precio en rango normal, factor grabado ≤ 1.5 | Cliente puede continuar |
| **NEEDS_REVIEW** | Fotograbado/3D (factor ≥ 2.5), muchos elementos, material premium (factor ≥ 1.5), precio alto | Operador revisa |
| **REJECTED** | Archivo inválido, colores incorrectos, excede tamaño máximo, no es SVG | Error al cliente |

---

## 7. Archivos para Claude Code

### 7.1 CLAUDE.md

Archivo raíz que define todo el contexto para Claude Code: descripción, stack, estructura, versiones exactas, convenciones Go y React, modelo de datos completo, reglas de negocio (colores SVG, fórmulas pricing, factores, clasificación), comandos build/test/deploy, fase actual y alcance.

### 7.2 Skills

| Skill | Propósito | Cuándo crearlo |
|-------|-----------|---------------|
| **fabricalaser-api** | Convenciones backend Go, patrones CRUD, middleware | Fase 0 (básico) |
| **fabricalaser-svg** | Pipeline análisis SVG, clasificación color, geometría | Fase 1 (cuando haya código real) |
| **fabricalaser-pricing** | Fórmula híbrida, factores, descuentos, clasificación | Fase 1 (cuando haya código real) |
| **fabricalaser-frontend** | Convenciones React/TS, componentes, hooks, API calls | Fase 2 (cuando haya componentes base) |

---

## 8. Cronograma

| Fase | Nombre | Depende de | Prioridad |
|------|--------|-----------|-----------|
| **0A** | Estructura + DB + Seed | — | 🔴 CRÍTICA |
| **0B** | Sistema de Autenticación | 0A | 🔴 CRÍTICA |
| **0C** | API Config + Servidor | 0B | 🔴 CRÍTICA |
| **0D** | Landing Page | 0C | 🟠 ALTA |
| **1** | Motor SVG + Pricing | 0C | 🔴 CRÍTICA |
| **2** | Frontend Wizard + Admin | 1 | 🟠 ALTA |
| **3** | Órdenes y Operaciones | 2 | 🟢 MEDIA |
| **4** | Pagos y Lanzamiento | 3 | 🟢 MEDIA |

**Diagrama de dependencias:**
```
0A → 0B → 0C → 0D (Landing)
              ↓
              1 (Motor SVG) → 2 (Frontend) → 3 (Órdenes) → 4 (Pagos)
```

**Nota:** 0D (Landing) y Fase 1 pueden ejecutarse en paralelo después de 0C.

**MVP funcional (0A-0C + 1 + 2):** Sistema de cotización funcionando end-to-end.
**Sistema completo (todas las fases):** Incluye pagos y flujo operativo completo.

---

## 9. Decisiones Arquitectónicas

| Decisión | Elegido | Razón |
|----------|---------|-------|
| Motor SVG | Go puro | Un binario, sin deps, control total |
| Router HTTP | Chi | Consistente con Sorteos/CalleViva |
| ORM | GORM | Migraciones, relaciones, consistente |
| Frontend | React + TypeScript | Tipado fuerte, ecosistema, consistente |
| Multi-tech | Desde el inicio | El modelo lo soporta sin costo extra |
| Modelo pricing | Híbrido dual | Del simulador v5: costo+margen Y valor |
| Arquitectura | Monolito modular | Simple, un deploy, separación interna |
| Archivos | Filesystem local | Simple, sin costo. S3 futuro |
| Pagos | Fase 4 | Primero validar motor + UX |
| Bézier | Subdivisión recursiva | Simple, preciso, configurable |
| Nombre/Dominio | FabricaLaser.com | Descriptivo, local, memorable |
| Auth/Usuarios | Cédula CR + JWT + bcrypt | Modelo de pagar.alonsoalpizar.com |
| Anti-competencia | Cuota 5 cotizaciones | Cédula identifica, cuota limita uso |
| Landing page | HTML estático + Nginx | Consistente con otros sitios del servidor |

---

## 10. Siguiente Paso: Fase 0A

Con este roadmap aprobado, ejecutar **Fase 0A** en Claude Code:

### Checklist Fase 0A

- [ ] Crear estructura de directorios en `/opt/FabricaLaser/`
- [ ] `go mod init github.com/alonsoalpizar/fabricalaser`
- [ ] Agregar dependencias: chi, gorm, pgx, redis, bcrypt, jwt-go
- [ ] Crear `internal/config/config.go` (env vars)
- [ ] Crear `internal/database/db.go` (conexión PostgreSQL)
- [ ] Crear modelos GORM: user, technology, material, engrave_type, tech_rate, volume_discount, price_reference
- [ ] Escribir migraciones SQL (001-007)
- [ ] Escribir seed data con valores del simulador v5
- [ ] Crear base de datos PostgreSQL `fabricalaser`
- [ ] Ejecutar migraciones y seed
- [ ] Crear Makefile con comandos básicos
- [ ] Actualizar CLAUDE.md

### Validación Fase 0A

```bash
# Verificar que compila
cd /opt/FabricaLaser && go build ./...

# Verificar datos en DB
psql -d fabricalaser -c "SELECT code, name FROM technologies"
psql -d fabricalaser -c "SELECT name, factor FROM materials"
psql -d fabricalaser -c "SELECT cedula, role FROM users WHERE role='admin'"
```

### Siguiente: Fase 0B

Una vez validada 0A, continuar con **Fase 0B: Sistema de Autenticación**.

---

*Este documento es un artefacto vivo que se actualiza al completar cada fase. Fuente única de verdad para el desarrollo de FabricaLaser.com.*