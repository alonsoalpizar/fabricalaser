package pricing

import "github.com/alonsoalpizar/fabricalaser/internal/models"

// BuildSyntheticAnalysis construye un SVGAnalysis en memoria desde medidas en mm,
// simulando lo que haría el analizador SVG con geometría rectangular simple.
//
// Reglas:
//   - engraveTypeID == 0 (solo corte, sin grabado): vector y raster en 0
//   - engraveTypeID == 1 (Vectorial): usa perímetro como VectorLengthMM
//   - engraveTypeID == 2,3,4 (Raster/Foto/3D): usa área como RasterAreaMM2
//   - incluyeCorte == true: CutLengthMM = perímetro del rectángulo
//   - BoundsMaxX/Y: define el bounding box para que TotalArea() y ComplexityFactor() funcionen
//
// Esta función es la única fuente de verdad para construir SVGAnalysis sintéticos:
// la usan estimate_handler.go (bot WhatsApp/Telegram) y la tool calcular_cotizacion
// del chat admin. Nunca duplicar — modificar acá afecta ambos canales.
func BuildSyntheticAnalysis(altoMM, anchoMM float64, incluyeCorte bool, engraveTypeID uint) *models.SVGAnalysis {
	var cutLengthMM, vectorLengthMM, rasterAreaMM2 float64

	if incluyeCorte {
		cutLengthMM = 2 * (altoMM + anchoMM)
	}

	if engraveTypeID == 1 {
		vectorLengthMM = 2 * (altoMM + anchoMM)
	} else if engraveTypeID > 1 {
		rasterAreaMM2 = altoMM * anchoMM
	}

	return &models.SVGAnalysis{
		RasterAreaMM2:  rasterAreaMM2,
		VectorLengthMM: vectorLengthMM,
		CutLengthMM:    cutLengthMM,
		BoundsMaxX:     anchoMM,
		BoundsMaxY:     altoMM,
	}
}
