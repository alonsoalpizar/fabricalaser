package cedula

import (
	"testing"
)

func TestValidarCedula_Real(t *testing.T) {
	service := NewCedulaService()

	// Test with a valid Costa Rican cedula format
	// Using a test cedula (this should return actual data from GoMeta)
	testCedulas := []struct {
		cedula      string
		shouldExist bool
	}{
		{"117520936", true},  // Física válida (ejemplo)
		{"999999999", false}, // Física que probablemente no existe
	}

	for _, tc := range testCedulas {
		t.Run(tc.cedula, func(t *testing.T) {
			result, err := service.ValidarCedula(tc.cedula)

			if err != nil {
				t.Logf("Error validating %s: %v", tc.cedula, err)
				return
			}

			t.Logf("Cedula: %s", result.Cedula)
			t.Logf("Válida: %v", result.Valida)
			t.Logf("Offline: %v", result.Offline)
			t.Logf("Tipo: %s", result.Tipo)

			if result.Valida {
				t.Logf("Nombre: %s", result.Nombre)
				t.Logf("Nombre Completo: %s", result.NombreCompleto)
				t.Logf("Primer Nombre: %s", result.PrimerNombre)
				t.Logf("Segundo Nombre: %s", result.SegundoNombre)
				t.Logf("Primer Apellido: %s", result.PrimerApellido)
				t.Logf("Segundo Apellido: %s", result.SegundoApellido)
			}

			if result.Error != "" {
				t.Logf("Error: %s", result.Error)
			}
		})
	}
}

func TestToMetadata(t *testing.T) {
	result := &CedulaValidationResult{
		Valida:         true,
		Cedula:         "117520936",
		Nombre:         "Juan",
		NombreCompleto: "Juan Carlos Pérez Mora",
		PrimerNombre:   "Juan",
		SegundoNombre:  "Carlos",
		PrimerApellido: "Pérez",
		SegundoApellido: "Mora",
		Tipo:           "fisica",
		Fuente:         "gometa",
	}

	metadata := result.ToMetadata()

	extras, ok := metadata["extras"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected extras map in metadata")
	}

	if extras["cedula"] != "117520936" {
		t.Errorf("Expected cedula 117520936, got %v", extras["cedula"])
	}

	if extras["primerNombre"] != "Juan" {
		t.Errorf("Expected primerNombre Juan, got %v", extras["primerNombre"])
	}

	t.Logf("Metadata: %+v", metadata)
}

func TestGetNombreFormateado(t *testing.T) {
	result := &CedulaValidationResult{
		PrimerNombre:    "Juan",
		SegundoNombre:   "Carlos",
		PrimerApellido:  "Pérez",
		SegundoApellido: "Mora",
	}

	nombre, apellido := result.GetNombreFormateado()

	if nombre != "Juan Carlos" {
		t.Errorf("Expected nombre 'Juan Carlos', got '%s'", nombre)
	}

	if apellido != "Pérez Mora" {
		t.Errorf("Expected apellido 'Pérez Mora', got '%s'", apellido)
	}
}
