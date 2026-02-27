# fixes-v5.md — Costos Configurables: Global + Por Tecnología

## ⚠️ REGLA DE PERSISTENCIA
Este archivo es la FUENTE DE VERDAD. Si perdés contexto: **RELEER completo antes de continuar.**

## Problema

Los valores `cost_per_min_engrave` y `cost_per_min_cut` en `tech_rates` son números pre-calculados manualmente. Además, el overhead (costos fijos) es diferente por máquina — una CO2 consume más electricidad y tiene depreciación diferente que una UV.

## Solución

Separar costos fijos en dos niveles:

**Global (system_config):** Costos del taller compartidos entre todas las máquinas
- Alquiler espacio
- Internet/servicios

**Por tecnología (tech_rates):** Costos específicos de cada máquina
- Electricidad
- Mantenimiento
- Depreciación del equipo
- Seguro
- Consumibles

**Fórmula:**
```
overhead_global_hora = sum(overhead globales) / horas_trabajo_mes
overhead_maquina_hora = sum(overhead por tech) / horas_trabajo_mes
cost_per_min = (tarifa_operador + overhead_global_hora + overhead_maquina_hora) / 60
```

## NO usar subagentes. Secuencial.

---

## Paso 1: Agregar costos GLOBALES en system_config

Crear seed nuevo (NO modificar seeds existentes):

```sql
-- Costos fijos GLOBALES del taller (₡/mes) — compartidos entre todas las tecnologías
INSERT INTO system_config (config_key, config_value, value_type, category, description) VALUES
('overhead_alquiler', '51500', 'number', 'overhead_global', 'Alquiler espacio mensual (₡) - costo oportunidad'),
('overhead_internet', '10300', 'number', 'overhead_global', 'Internet/servicios proporcional mensual (₡)'),
('horas_trabajo_mes', '120', 'number', 'overhead_global', 'Horas de trabajo estimadas por mes')
ON CONFLICT (config_key) DO UPDATE SET
    config_value = EXCLUDED.config_value,
    description = EXCLUDED.description;
```

---

## Paso 2: Agregar costos POR TECNOLOGÍA en tech_rates

Agregar columnas nuevas a la tabla `tech_rates`:

```sql
ALTER TABLE tech_rates
  ADD COLUMN IF NOT EXISTS electricidad_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS mantenimiento_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS depreciacion_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS seguro_mes FLOAT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS consumibles_mes FLOAT NOT NULL DEFAULT 0;

COMMENT ON COLUMN tech_rates.electricidad_mes IS 'Electricidad mensual de esta máquina (₡)';
COMMENT ON COLUMN tech_rates.mantenimiento_mes IS 'Mantenimiento preventivo mensual de esta máquina (₡)';
COMMENT ON COLUMN tech_rates.depreciacion_mes IS 'Depreciación mensual del equipo (₡) - costo/meses_vida_util';
COMMENT ON COLUMN tech_rates.seguro_mes IS 'Seguro mensual del equipo (₡)';
COMMENT ON COLUMN tech_rates.consumibles_mes IS 'Consumibles mensuales de esta máquina (₡)';
```

Seed con valores del Excel (inicialmente iguales para todas, Alonso los ajustará por máquina):

```sql
-- Costos específicos por máquina (₡/mes)
-- Valores iniciales del Simulador Excel v5 — ajustar por tecnología real
UPDATE tech_rates SET
    electricidad_mes = 15450,
    mantenimiento_mes = 25750,
    depreciacion_mes = 96990,
    seguro_mes = 12875,
    consumibles_mes = 20600
WHERE technology_id IN (SELECT id FROM technologies);
```

---

## Paso 3: Modelo Go — TechRate

En el modelo `TechRate`, agregar los campos nuevos:

```go
ElectricidadMes  float64 `gorm:"type:float;not null;default:0" json:"electricidad_mes"`
MantenimientoMes float64 `gorm:"type:float;not null;default:0" json:"mantenimiento_mes"`
DepreciacionMes  float64 `gorm:"type:float;not null;default:0" json:"depreciacion_mes"`
SeguroMes        float64 `gorm:"type:float;not null;default:0" json:"seguro_mes"`
ConsumiblesMes   float64 `gorm:"type:float;not null;default:0" json:"consumibles_mes"`
```

---

## Paso 4: config_loader.go — Cálculo dinámico

### 4a. Overhead global (del taller)

```go
// GetOverheadGlobalPerHourCRC returns shared taller overhead per hour in CRC
// Includes: alquiler, internet — costs shared across all machines
func (c *PricingConfig) GetOverheadGlobalPerHourCRC() float64 {
    alquiler := c.GetSystemConfigFloat("overhead_alquiler", 0)
    internet := c.GetSystemConfigFloat("overhead_internet", 0)
    totalGlobal := alquiler + internet

    horasMes := c.GetSystemConfigFloat("horas_trabajo_mes", 120)
    if horasMes <= 0 {
        horasMes = 120
    }
    return totalGlobal / horasMes
}
```

### 4b. Overhead por máquina (de tech_rates)

```go
// GetOverheadMaquinaPerHourCRC returns machine-specific overhead per hour in CRC
// Includes: electricidad, mantenimiento, depreciación, seguro, consumibles
func (c *PricingConfig) GetOverheadMaquinaPerHourCRC(techID uint) float64 {
    rate := c.TechRates[techID]
    if rate == nil {
        return 0
    }

    totalMaquina := rate.ElectricidadMes +
        rate.MantenimientoMes +
        rate.DepreciacionMes +
        rate.SeguroMes +
        rate.ConsumiblesMes

    horasMes := c.GetSystemConfigFloat("horas_trabajo_mes", 120)
    if horasMes <= 0 {
        horasMes = 120
    }
    return totalMaquina / horasMes
}
```

### 4c. Cost per minute dinámico

```go
// GetCostPerMinEngrave returns cost per minute for engraving
// Formula: (tarifa_operador + overhead_global + overhead_maquina) / 60
func (c *PricingConfig) GetCostPerMinEngrave(techID uint) float64 {
    rate := c.TechRates[techID]
    if rate == nil {
        return 0
    }
    overheadGlobal := c.GetOverheadGlobalPerHourCRC()
    overheadMaquina := c.GetOverheadMaquinaPerHourCRC(techID)
    return (rate.EngraveRateHour + overheadGlobal + overheadMaquina) / 60
}

// GetCostPerMinCut returns cost per minute for cutting
// Formula: (tarifa_operador + overhead_global + overhead_maquina) / 60
func (c *PricingConfig) GetCostPerMinCut(techID uint) float64 {
    rate := c.TechRates[techID]
    if rate == nil {
        return 0
    }
    overheadGlobal := c.GetOverheadGlobalPerHourCRC()
    overheadMaquina := c.GetOverheadMaquinaPerHourCRC(techID)
    return (rate.CutRateHour + overheadGlobal + overheadMaquina) / 60
}
```

### 4d. Verificar que calculator.go usa estos métodos con techID

Buscar en `calculator.go` donde se obtiene el cost_per_min. Si actualmente hace:

```go
costPerMinEngrave := config.GetCostPerMinEngrave()
```

Cambiar a:

```go
costPerMinEngrave := config.GetCostPerMinEngrave(techID)
costPerMinCut := config.GetCostPerMinCut(techID)
```

**IMPORTANTE:** Verificar la firma actual de estos métodos. Si ya reciben techID, solo hay que cambiar la implementación interna. Si no reciben techID, hay que actualizar la firma Y todas las llamadas.

---

## Paso 5: El campo overhead_rate_hour en tech_rates

El campo `overhead_rate_hour` existente en `tech_rates` se vuelve un campo CALCULADO de referencia. No se usa para el cálculo real — pero se puede actualizar para auditoría:

```go
// Para mostrar en el admin como referencia:
// overhead_rate_hour = GetOverheadGlobalPerHourCRC() + GetOverheadMaquinaPerHourCRC(techID)
```

NO eliminar el campo. Dejarlo como referencia visual en el admin.

---

## Verificación

```bash
# 1. Compilar
go build ./...

# 2. Verificar columnas nuevas en modelo
grep -n "ElectricidadMes\|MantenimientoMes\|DepreciacionMes\|SeguroMes\|ConsumiblesMes" internal/models/

# 3. Verificar cálculo dinámico
grep -n "GetOverheadGlobalPerHourCRC\|GetOverheadMaquinaPerHourCRC\|GetCostPerMinEngrave" internal/services/pricing/config_loader.go

# 4. Verificar que calculator usa techID
grep -n "GetCostPerMinEngrave\|GetCostPerMinCut" internal/services/pricing/calculator.go

# 5. Verificar seeds
grep -n "overhead_alquiler\|overhead_internet\|horas_trabajo_mes" seeds/
grep -n "electricidad_mes\|mantenimiento_mes\|depreciacion_mes" seeds/
```

## Validación matemática

```
CO2 (con valores iniciales iguales para todas):

Global:
  alquiler ₡51,500 + internet ₡10,300 = ₡61,800/mes
  ÷ 120 horas = ₡515/hora

Máquina CO2:
  electricidad ₡15,450 + mantenimiento ₡25,750 + depreciación ₡96,990
  + seguro ₡12,875 + consumibles ₡20,600 = ₡171,665/mes
  ÷ 120 horas = ₡1,430.54/hora

Total overhead: ₡515 + ₡1,430.54 = ₡1,945.54/hora

cost_per_min_engrave CO2 = (₡6,180 + ₡1,945.54) / 60 = ₡135.43/min
cost_per_min_cut CO2 = (₡7,210 + ₡1,945.54) / 60 = ₡152.59/min

Resultado prácticamente igual al actual (₡135.45 / ₡152.62)
Diferencia < ₡0.03 por redondeo.
```

## Beneficio

- Cambiar electricidad de la CO2: editar `electricidad_mes` en tech_rates de CO2
- Cambiar alquiler del taller: editar `overhead_alquiler` en system_config
- Agregar nueva máquina UV potente: poner sus propios costos en tech_rates
- Todo se recalcula automáticamente, sin tocar código
- Cada máquina tiene su costo real, no un promedio global