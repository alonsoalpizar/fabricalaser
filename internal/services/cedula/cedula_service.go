package cedula

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/config"
	"github.com/alonsoalpizar/fabricalaser/internal/utils"
)

const (
	goMetaAPIURL = "https://apis.gometa.org/cedulas"
)

var (
	ErrCedulaNotFound    = errors.New("cédula no encontrada en el registro civil")
	ErrServiceOffline    = errors.New("servicio de validación no disponible temporalmente")
	ErrInvalidCedula     = errors.New("formato de cédula inválido")
	ErrInvalidResponse   = errors.New("respuesta inválida del servicio de validación")
)

// GoMetaResponse represents the response from GoMeta API
type GoMetaResponse struct {
	Results []GoMetaResult `json:"results"`
}

// GoMetaResult represents a single result from GoMeta
type GoMetaResult struct {
	Cedula     string `json:"cedula"`
	FirstName1 string `json:"firstname1"`
	FirstName2 string `json:"firstname2"`
	LastName1  string `json:"lastname1"`
	LastName2  string `json:"lastname2"`
	FullName   string `json:"fullname"`
	GuessType  string `json:"guess_type"` // FISICA, JURIDICA
}

// CedulaValidationResult contains the complete validation result
type CedulaValidationResult struct {
	Valida              bool                   `json:"valida"`
	Offline             bool                   `json:"offline,omitempty"`
	Cedula              string                 `json:"cedula"`
	Nombre              string                 `json:"nombre,omitempty"`
	NombreCompleto      string                 `json:"nombreCompleto,omitempty"`
	PrimerNombre        string                 `json:"primerNombre,omitempty"`
	SegundoNombre       string                 `json:"segundoNombre,omitempty"`
	PrimerApellido      string                 `json:"primerApellido,omitempty"`
	SegundoApellido     string                 `json:"segundoApellido,omitempty"`
	Tipo                string                 `json:"tipo"` // fisica, juridica
	TipoIdentificacion  string                 `json:"tipoIdentificacion"`
	SituacionTributaria string                 `json:"situacionTributaria,omitempty"`
	Actividades         []map[string]string    `json:"actividades,omitempty"`
	FechaConsulta       time.Time              `json:"fechaConsulta"`
	Fuente              string                 `json:"fuente"`
	Error               string                 `json:"error,omitempty"`
}

// CedulaService handles cedula validation against GoMeta API
type CedulaService struct {
	httpClient *http.Client
	timeout    time.Duration
}

// NewCedulaService creates a new CedulaService
func NewCedulaService() *CedulaService {
	cfg := config.Get()
	timeout := time.Duration(cfg.GoMetaTimeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return &CedulaService{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// ValidarCedula validates a cedula against GoMeta API (Costa Rican civil registry)
func (s *CedulaService) ValidarCedula(identificacion string) (*CedulaValidationResult, error) {
	// First validate format locally
	validation := utils.ValidateCedula(identificacion)
	if !validation.Valid {
		return nil, ErrInvalidCedula
	}

	result := &CedulaValidationResult{
		Cedula:        validation.Cedula,
		Tipo:          string(validation.Type),
		FechaConsulta: time.Now().UTC(),
		Fuente:        "gometa",
	}

	// Call GoMeta API
	url := fmt.Sprintf("%s/%s", goMetaAPIURL, validation.Cedula)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		result.Offline = true
		result.Error = ErrServiceOffline.Error()
		return result, nil // Return result with offline flag, not error
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FabricaLaser/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Network error - service offline
		result.Offline = true
		result.Error = ErrServiceOffline.Error()
		return result, nil
	}
	defer resp.Body.Close()

	// Handle different status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Parse response
		var goMetaResp GoMetaResponse
		if err := json.NewDecoder(resp.Body).Decode(&goMetaResp); err != nil {
			result.Offline = true
			result.Error = ErrInvalidResponse.Error()
			return result, nil
		}

		// Check if results exist
		if len(goMetaResp.Results) == 0 {
			result.Valida = false
			result.Error = ErrCedulaNotFound.Error()
			return result, nil
		}

		// Extract data from first result
		data := goMetaResp.Results[0]
		result.Valida = true
		result.PrimerNombre = formatName(data.FirstName1)
		result.SegundoNombre = formatName(data.FirstName2)
		result.PrimerApellido = formatName(data.LastName1)
		result.SegundoApellido = formatName(data.LastName2)
		result.NombreCompleto = formatName(data.FullName)
		result.Nombre = result.PrimerNombre

		// Determine tipo from API response
		if data.GuessType != "" {
			result.TipoIdentificacion = strings.ToLower(data.GuessType)
			if strings.ToUpper(data.GuessType) == "JURIDICA" {
				result.Tipo = "juridica"
			} else {
				result.Tipo = "fisica"
			}
		}

		return result, nil

	case http.StatusNotFound:
		result.Valida = false
		result.Error = ErrCedulaNotFound.Error()
		return result, nil

	default:
		// Other errors - treat as offline
		result.Offline = true
		result.Error = fmt.Sprintf("error del servicio: código %d", resp.StatusCode)
		return result, nil
	}
}

// ToMetadata converts the validation result to metadata JSON for storage
func (r *CedulaValidationResult) ToMetadata() map[string]interface{} {
	return map[string]interface{}{
		"extras": map[string]interface{}{
			"validadoRegistroCivil": r.Valida,
			"cedula":                r.Cedula,
			"nombreOficial":         r.NombreCompleto,
			"tipo":                  strings.ToUpper(r.Tipo),
			"tipoIdentificacion":    r.TipoIdentificacion,
			"primerNombre":          r.PrimerNombre,
			"segundoNombre":         r.SegundoNombre,
			"primerApellido":        r.PrimerApellido,
			"segundoApellido":       r.SegundoApellido,
			"situacionTributaria":   r.SituacionTributaria,
			"actividades":           r.Actividades,
			"fechaConsulta":         r.FechaConsulta.Format(time.RFC3339),
			"fuente":                r.Fuente,
		},
	}
}

// GetNombreFormateado returns the formatted name from the validation result
func (r *CedulaValidationResult) GetNombreFormateado() (nombre string, apellido string) {
	nombre = r.PrimerNombre
	if r.SegundoNombre != "" {
		nombre += " " + r.SegundoNombre
	}

	apellido = r.PrimerApellido
	if r.SegundoApellido != "" {
		apellido += " " + r.SegundoApellido
	}

	return nombre, apellido
}

// formatName cleans and formats a name string
func formatName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	// Capitalize first letter of each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// CacheValido checks if cached metadata is still valid (24 hours)
func CacheValido(metadata map[string]interface{}) bool {
	extras, ok := metadata["extras"].(map[string]interface{})
	if !ok {
		return false
	}

	fechaStr, ok := extras["fechaConsulta"].(string)
	if !ok {
		return false
	}

	fecha, err := time.Parse(time.RFC3339, fechaStr)
	if err != nil {
		return false
	}

	// Cache valid for 24 hours
	return time.Since(fecha) < 24*time.Hour
}
