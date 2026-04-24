package pricing

import "github.com/alonsoalpizar/fabricalaser/internal/models"

// defaultVectorComplexityFactor es el factor usado cuando el caller pasa un valor
// <= 0 (no configurado). Mantiene compatibilidad mínima si se invoca sin contexto.
const defaultVectorComplexityFactor = 3.5

// BuildSyntheticAnalysis construye un SVGAnalysis en memoria desde medidas en mm,
// simulando lo que haría el analizador SVG con geometría rectangular simple.
//
// Reglas:
//   - engraveTypeID == 0 (solo corte, sin grabado): vector y raster en 0
//   - engraveTypeID == 1 (Vectorial): usa perímetro × vectorComplexityFactor como VectorLengthMM.
//     El perímetro del bounding box es el mínimo absoluto del recorrido vectorial (equivale a
//     grabar solo el contorno). Un diseño real (logo, texto, ilustración) tiene típicamente 3×-5×
//     más recorrido; el factor lo captura. Default 3.5 si el caller pasa 0.
//   - engraveTypeID == 2,3,4 (Raster/Foto/3D): usa área como RasterAreaMM2
//   - incluyeCorte == true: CutLengthMM = perímetro del rectángulo
//   - BoundsMaxX/Y: define el bounding box para que TotalArea() y ComplexityFactor() funcionen
//
// Esta función es la única fuente de verdad para construir SVGAnalysis sintéticos:
// la usan estimate_handler.go (bot WhatsApp/Telegram) y la tool calcular_cotizacion
// del chat admin. Nunca duplicar — modificar acá afecta ambos canales.
//
// vectorComplexityFactor se carga desde system_config.synthetic_vector_complexity_factor
// en los call sites, lo que permite al gestor ajustarlo sin recompilar cuando compare
// precios reales contra cotizaciones emitidas.
func BuildSyntheticAnalysis(altoMM, anchoMM float64, incluyeCorte bool, engraveTypeID uint, vectorComplexityFactor float64) *models.SVGAnalysis {
	if vectorComplexityFactor <= 0 {
		vectorComplexityFactor = defaultVectorComplexityFactor
	}

	var cutLengthMM, vectorLengthMM, rasterAreaMM2 float64

	if incluyeCorte {
		cutLengthMM = 2 * (altoMM + anchoMM)
	}

	if engraveTypeID == 1 {
		perimeter := 2 * (altoMM + anchoMM)
		vectorLengthMM = perimeter * vectorComplexityFactor
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
