package utils

import (
	"regexp"
	"strings"
)

// CedulaType represents the type of Costa Rican ID
type CedulaType string

const (
	CedulaFisica   CedulaType = "fisica"
	CedulaJuridica CedulaType = "juridica"
)

var (
	// Física: 9 dígitos exactos, no empieza con 0
	cedulaFisicaRegex = regexp.MustCompile(`^[1-9]\d{8}$`)
	// Jurídica: 10 dígitos exactos, no empieza con 0
	cedulaJuridicaRegex = regexp.MustCompile(`^[1-9]\d{9}$`)
	// Solo dígitos
	onlyDigitsRegex = regexp.MustCompile(`\D`)
)

// CedulaValidation contains the result of cedula validation
type CedulaValidation struct {
	Valid  bool
	Type   CedulaType
	Cedula string
}

// ValidateCedula validates a Costa Rican cedula (física or jurídica)
// Returns the validation result with cleaned cedula and type
func ValidateCedula(identificacion string) CedulaValidation {
	if identificacion == "" {
		return CedulaValidation{Valid: false}
	}

	// Clean: remove all non-digit characters
	cleaned := CleanCedula(identificacion)

	// Check física (9 digits)
	if cedulaFisicaRegex.MatchString(cleaned) {
		return CedulaValidation{
			Valid:  true,
			Type:   CedulaFisica,
			Cedula: cleaned,
		}
	}

	// Check jurídica (10 digits)
	if cedulaJuridicaRegex.MatchString(cleaned) {
		return CedulaValidation{
			Valid:  true,
			Type:   CedulaJuridica,
			Cedula: cleaned,
		}
	}

	return CedulaValidation{Valid: false}
}

// CleanCedula removes all non-digit characters from a cedula string
func CleanCedula(cedula string) string {
	return onlyDigitsRegex.ReplaceAllString(strings.TrimSpace(cedula), "")
}

// IsCedulaFisica checks if the cedula is a valid física (9 digits)
func IsCedulaFisica(cedula string) bool {
	return cedulaFisicaRegex.MatchString(CleanCedula(cedula))
}

// IsCedulaJuridica checks if the cedula is a valid jurídica (10 digits)
func IsCedulaJuridica(cedula string) bool {
	return cedulaJuridicaRegex.MatchString(CleanCedula(cedula))
}
