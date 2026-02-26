-- Migration 012: Technology-Material-Thickness Speeds Matrix
-- FabricaLaser - Velocidades por combinación tecnología/material/grosor

CREATE TABLE IF NOT EXISTS tech_material_speeds (
    id SERIAL PRIMARY KEY,
    technology_id INTEGER NOT NULL REFERENCES technologies(id) ON DELETE CASCADE,
    material_id INTEGER NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    thickness DECIMAL(5,2) NOT NULL,
    cut_speed_mm_min DECIMAL(10,2),
    engrave_speed_mm_min DECIMAL(10,2),
    is_compatible BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(technology_id, material_id, thickness)
);

CREATE INDEX IF NOT EXISTS idx_tms_technology ON tech_material_speeds(technology_id);
CREATE INDEX IF NOT EXISTS idx_tms_material ON tech_material_speeds(material_id);
CREATE INDEX IF NOT EXISTS idx_tms_thickness ON tech_material_speeds(thickness);
CREATE INDEX IF NOT EXISTS idx_tms_compatible ON tech_material_speeds(is_compatible) WHERE is_compatible = true;
CREATE INDEX IF NOT EXISTS idx_tms_active ON tech_material_speeds(is_active) WHERE is_active = true;

COMMENT ON TABLE tech_material_speeds IS 'Matriz de velocidades por combinación tecnología/material/grosor';
COMMENT ON COLUMN tech_material_speeds.cut_speed_mm_min IS 'Velocidad de corte mm/min (NULL si no corta)';
COMMENT ON COLUMN tech_material_speeds.engrave_speed_mm_min IS 'Velocidad de grabado mm/min (NULL si no graba)';
COMMENT ON COLUMN tech_material_speeds.is_compatible IS 'Si esta combinación es posible';
