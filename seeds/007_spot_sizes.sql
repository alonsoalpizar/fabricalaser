-- Seed: Spot sizes por tecnología
-- Estos valores ya se aplican en la migración 017, este archivo es de referencia
-- Los valores son del manual de las máquinas del taller

-- CO2: Láser de tubo, spot típico 0.1mm (100 micrones)
UPDATE technologies SET spot_size_mm = 0.10 WHERE code = 'CO2';

-- FIBRA: Láser de fibra, spot muy fino 0.03mm (30 micrones)
UPDATE technologies SET spot_size_mm = 0.03 WHERE code = 'FIBRA';

-- MOPA: Fibra con control de pulso, spot 0.04mm (40 micrones)
UPDATE technologies SET spot_size_mm = 0.04 WHERE code = 'MOPA';

-- UV: Láser ultravioleta, spot más fino 0.02mm (20 micrones)
UPDATE technologies SET spot_size_mm = 0.02 WHERE code = 'UV';
