# Refactorización Pricing Engine v2 - FabricaLaser

**Fecha:** 2026-02-26
**Estado:** En Planificación
**Prioridad:** Alta

---

## Resumen Ejecutivo

Rediseño completo del motor de cotización para:
1. Eliminar todos los hardcodes del código
2. Implementar matriz Tecnología × Material × Grosor
3. Sistema 100% configurable desde admin
4. Moneda única: Colones (CRC) sin conversiones

---

## Estado Actual (Problemas Identificados)

### Hardcodes en el Código

| Archivo | Constante | Valor | Problema |
|---------|-----------|-------|----------|
| `time_estimator.go` | baseEngraveAreaSpeed | 500 mm²/min | No configurable |
| `time_estimator.go` | baseEngraveLineSpeed | 100 mm/min | No configurable |
| `time_estimator.go` | baseCutSpeed | 20 mm/min | No configurable |
| `time_estimator.go` | setupTimeMinutes | 5 min | No configurable |
| `calculator.go` | complexityAutoApprove | 6.0 | No configurable |
| `calculator.go` | complexityNeedsReview | 12.0 | No configurable |
| `calculator.go` | quoteValidityDays | 7 | No configurable |
| `calculator.go` | minValueBase | 2575.0 | No configurable |
| `calculator.go` | pricePerMM2 | 0.515 | No configurable |
| `config_loader.go` | defaultMargin | 0.40 | No configurable |

### Grosor No Afecta Cálculos
- El campo `thickness` se guarda pero NO se usa en fórmulas
- Acrílico 3mm y 10mm dan el mismo tiempo de corte (incorrecto)

### Falta Matriz de Compatibilidad
- No hay definición de qué tecnología puede con qué material
- No hay velocidades específicas por combinación tech+material+grosor

---

## Fases de Implementación

### Fase 1: Limpieza (Deshacer errores)
**Objetivo:** Revertir cambios incorrectos relacionados con tipo de cambio

- [ ] Revisar `calculator.go` - eliminar cualquier cálculo con ×515
- [ ] Verificar que los seeds tienen valores correctos en colones
- [ ] Confirmar que frontend muestra ₡ sin conversiones

**Archivos:**
- `internal/services/pricing/calculator.go`
- `seeds/001_initial_data.sql`

---

### Fase 2: Tabla de Configuración General
**Objetivo:** Mover hardcodes a BD configurable

#### 2.1 Nueva tabla `system_config`
```sql
CREATE TABLE system_config (
    id SERIAL PRIMARY KEY,
    key VARCHAR(100) UNIQUE NOT NULL,
    value TEXT NOT NULL,
    value_type VARCHAR(20) DEFAULT 'string', -- string, number, boolean, json
    category VARCHAR(50),
    description TEXT,
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### 2.2 Datos iniciales
```sql
INSERT INTO system_config (key, value, value_type, category, description) VALUES
('base_engrave_area_speed', '500', 'number', 'speeds', 'Velocidad base grabado área (mm²/min)'),
('base_engrave_line_speed', '100', 'number', 'speeds', 'Velocidad base grabado línea (mm/min)'),
('base_cut_speed', '20', 'number', 'speeds', 'Velocidad base corte (mm/min)'),
('setup_time_minutes', '5', 'number', 'times', 'Tiempo setup por trabajo (min)'),
('complexity_auto_approve', '6.0', 'number', 'complexity', 'Factor máximo auto-aprobación'),
('complexity_needs_review', '12.0', 'number', 'complexity', 'Factor máximo para revisión'),
('quote_validity_days', '7', 'number', 'quotes', 'Días validez cotización'),
('min_value_base', '2575', 'number', 'pricing', 'Precio mínimo base (₡)'),
('price_per_mm2', '0.515', 'number', 'pricing', 'Precio por mm² (₡)'),
('min_area_mm2', '100', 'number', 'pricing', 'Área mínima para cobrar (mm²)');
```

#### 2.3 Backend
- Modelo: `internal/models/system_config.go`
- Repository: `internal/repository/system_config_repo.go`
- Handler: `internal/handlers/admin/system_config_handler.go`
- Rutas: GET/PUT `/admin/system-config`

#### 2.4 Frontend Admin
- Nueva página: `web/admin/config/general.html`
- Formulario para editar cada configuración por categoría

---

### Fase 3: Matriz Tecnología × Material × Grosor
**Objetivo:** Definir velocidades y compatibilidad por cada combinación

#### 3.1 Nueva tabla `tech_material_speeds`
```sql
CREATE TABLE tech_material_speeds (
    id SERIAL PRIMARY KEY,
    technology_id INT REFERENCES technologies(id) ON DELETE CASCADE,
    material_id INT REFERENCES materials(id) ON DELETE CASCADE,
    thickness DECIMAL(5,2) NOT NULL,
    cut_speed_mm_min DECIMAL(10,2),      -- NULL si no corta
    engrave_speed_mm_min DECIMAL(10,2),  -- NULL si no graba
    is_compatible BOOLEAN DEFAULT true,
    notes TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(technology_id, material_id, thickness)
);
```

#### 3.2 Ejemplo de datos
| Tecnología | Material | Grosor | Corte mm/min | Grabado mm/min | Compatible |
|------------|----------|--------|--------------|----------------|------------|
| CO2 | Acrílico | 3mm | 25 | 500 | ✓ |
| CO2 | Acrílico | 5mm | 15 | 500 | ✓ |
| CO2 | Acrílico | 10mm | 8 | 500 | ✓ |
| UV | Acrílico | 3mm | NULL | 400 | ✗ (no corta) |
| UV | Vidrio | 3mm | NULL | 300 | ✓ (solo graba) |
| FIBRA | Metal | - | NULL | 600 | ✓ (solo graba) |

#### 3.3 Backend
- Modelo: `internal/models/tech_material_speed.go`
- Repository: `internal/repository/tech_material_speed_repo.go`
- Handler: `internal/handlers/admin/tech_material_speed_handler.go`
- Rutas: CRUD `/admin/tech-material-speeds`
- Endpoint público: GET `/api/compatible-options?material_id=X&thickness=Y`

#### 3.4 Frontend Admin
- Nueva página: `web/admin/config/speeds.html`
- UI tipo matriz/grid para configurar
- Filtros por tecnología y material
- Bulk edit para múltiples grosores

---

### Fase 4: Refactorizar Motor de Cálculo
**Objetivo:** Usar nueva configuración en cálculos

#### 4.1 Modificar `config_loader.go`
- Cargar `system_config` en cache
- Métodos: `GetConfigFloat(key)`, `GetConfigInt(key)`

#### 4.2 Modificar `time_estimator.go`
- Recibir `thickness` como parámetro
- Buscar velocidad en `tech_material_speeds`
- Fallback a velocidad base de `system_config` si no hay match

```go
func (e *TimeEstimator) Estimate(
    analysis *models.SVGAnalysis,
    techID uint,
    materialID uint,
    engraveTypeID uint,
    thickness float64,  // NUEVO
    quantity int,
) TimeEstimate {
    // Buscar velocidad específica
    speed := e.config.GetMaterialSpeed(techID, materialID, thickness)
    if speed == nil {
        // Usar velocidad base de system_config
        speed = e.config.GetBaseSpeed()
    }
    // ...resto del cálculo
}
```

#### 4.3 Modificar `calculator.go`
- Leer constantes de `system_config`
- Pasar `thickness` a `TimeEstimator`

---

### Fase 5: Actualizar Cotizador Frontend
**Objetivo:** Wizard inteligente que filtra opciones válidas

#### 5.1 Flujo del Wizard
1. Usuario sube SVG → Se analiza
2. Selecciona Material → API devuelve tecnologías compatibles
3. Selecciona Tecnología → API devuelve grosores disponibles
4. Selecciona Grosor → Calcula precio con velocidades específicas

#### 5.2 Modificar `web/cotizar/index.html`
- Llamar `/api/compatible-options` al cambiar selección
- Deshabilitar opciones no compatibles
- Mostrar notas de compatibilidad

#### 5.3 Símbolos de Moneda
- Verificar que todos los `$` sean `₡`
- Usar `toLocaleString('es-CR')` para formato
- NO hacer ningún cálculo de conversión

---

### Fase 6: Testing y Validación
**Objetivo:** Asegurar que los cálculos son correctos

#### 6.1 Tests Unitarios
- `pricing/calculator_test.go`
- `pricing/time_estimator_test.go`

#### 6.2 Casos de Prueba
| Escenario | Material | Grosor | Tech | Esperado |
|-----------|----------|--------|------|----------|
| Simple | Madera 3mm | 3 | CO2 | Validar vs Excel |
| Medio | Acrílico 5mm | 5 | CO2 | Validar vs Excel |
| Complejo | Acrílico 10mm | 10 | CO2 | Validar vs Excel |

#### 6.3 Validación
- Comparar resultados con simulador Excel original
- Ajustar velocidades según diferencias

---

## Archivos a Crear

| Archivo | Descripción |
|---------|-------------|
| `migrations/00X_system_config.sql` | Tabla system_config |
| `migrations/00X_tech_material_speeds.sql` | Tabla matriz |
| `internal/models/system_config.go` | Modelo |
| `internal/models/tech_material_speed.go` | Modelo |
| `internal/repository/system_config_repo.go` | Repository |
| `internal/repository/tech_material_speed_repo.go` | Repository |
| `internal/handlers/admin/system_config_handler.go` | Handler |
| `internal/handlers/admin/tech_material_speed_handler.go` | Handler |
| `web/admin/config/general.html` | Admin UI config |
| `web/admin/config/speeds.html` | Admin UI matriz |

## Archivos a Modificar

| Archivo | Cambios |
|---------|---------|
| `internal/services/pricing/config_loader.go` | Cargar system_config |
| `internal/services/pricing/time_estimator.go` | Usar matriz de velocidades |
| `internal/services/pricing/calculator.go` | Leer config de BD |
| `internal/routes/admin.go` | Nuevas rutas |
| `web/admin/admin.js` | Sidebar nueva sección |
| `web/cotizar/index.html` | Filtros dinámicos |

---

## Orden de Ejecución

| Fase | Descripción | Dependencias | Paralelizable |
|------|-------------|--------------|---------------|
| 1 | Limpieza | Ninguna | No |
| 2 | System Config | Fase 1 | Sí (con Fase 3) |
| 3 | Matriz Speeds | Fase 1 | Sí (con Fase 2) |
| 4 | Refactor Motor | Fase 2, 3 | No |
| 5 | Frontend Cotizador | Fase 4 | No |
| 6 | Testing | Fase 5 | No |

---

## Notas Importantes

1. **Sin Docker** - Deploy nativo con systemd
2. **Moneda** - Solo colones (₡), sin conversiones de tipo de cambio
3. **Compatibilidad** - Mantener API existente mientras se migra
4. **Rollback** - Cada fase debe poder revertirse independiente
5. **UX Admin** - La UI de la matriz debe ser intuitiva para configurar muchas combinaciones

---

## Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-02-26 | Documento inicial creado |
