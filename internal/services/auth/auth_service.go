package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/config"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/alonsoalpizar/fabricalaser/internal/services/cedula"
	emailSvc "github.com/alonsoalpizar/fabricalaser/internal/services/email"
	"github.com/alonsoalpizar/fabricalaser/internal/utils"
	"gorm.io/datatypes"
)

var (
	ErrInvalidCedula        = errors.New("formato de cédula inválido. Use 9 dígitos para física o 10 para jurídica")
	ErrCedulaNotFound       = errors.New("cédula no registrada")
	ErrAccountExists        = errors.New("ya existe una cuenta con esta cédula")
	ErrEmailExists          = errors.New("ya existe una cuenta con este email")
	ErrTelefonoExists       = errors.New("ya existe una cuenta con este número de teléfono")
	ErrInvalidTelefono      = errors.New("número de teléfono inválido. Ingrese 8 dígitos (ej: 88887777)")
	ErrInvalidPassword      = errors.New("contraseña incorrecta")
	ErrAccountDisabled      = errors.New("cuenta desactivada. Contacte al administrador")
	ErrWeakPassword         = errors.New("la contraseña debe tener al menos 6 caracteres")
	ErrUserHasPassword      = errors.New("cliente no encontrado o ya tiene contraseña")
	ErrCedulaNotValid       = errors.New("cédula no válida según el registro civil de Costa Rica")
	ErrValidationOffline    = errors.New("servicio de validación de cédula no disponible. Intente más tarde")
	ErrResetTokenInvalid    = errors.New("el enlace de recuperación no es válido o ya expiró")
	ErrCurrentPasswordWrong = errors.New("la contraseña actual es incorrecta")
)

// VerifyCedulaResult represents the result of cedula verification
type VerifyCedulaResult struct {
	Existe              bool                   `json:"existe"`
	TienePassword       bool                   `json:"tienePassword"`
	Tipo                string                 `json:"tipo"`
	Cedula              string                 `json:"cedula"`
	ValidadoRegistroCivil bool                 `json:"validadoRegistroCivil"`
	DatosRegistroCivil  *DatosRegistroCivil    `json:"datosRegistroCivil,omitempty"`
	Cliente             map[string]interface{} `json:"cliente,omitempty"`
}

// DatosRegistroCivil contains official data from GoMeta/Civil Registry
type DatosRegistroCivil struct {
	Nombre              string `json:"nombre"`
	Apellido            string `json:"apellido"`
	NombreCompleto      string `json:"nombreCompleto"`
	PrimerNombre        string `json:"primerNombre"`
	SegundoNombre       string `json:"segundoNombre"`
	PrimerApellido      string `json:"primerApellido"`
	SegundoApellido     string `json:"segundoApellido"`
	Tipo                string `json:"tipo"`
	TipoIdentificacion  string `json:"tipoIdentificacion"`
	SituacionTributaria string `json:"situacionTributaria,omitempty"`
}

// AuthResult represents the result of login/register
type AuthResult struct {
	Token   string                 `json:"token"`
	Usuario map[string]interface{} `json:"usuario"`
}

type AuthService struct {
	userRepo      *repository.UserRepository
	cedulaService *cedula.CedulaService
	config        *config.Config
}

func NewAuthService() *AuthService {
	return &AuthService{
		userRepo:      repository.NewUserRepository(),
		cedulaService: cedula.NewCedulaService(),
		config:        config.Get(),
	}
}

// VerificarCedula checks if a cedula exists, validates against GoMeta, and returns official data
func (s *AuthService) VerificarCedula(identificacion string) (*VerifyCedulaResult, error) {
	// First validate format locally
	validation := utils.ValidateCedula(identificacion)
	if !validation.Valid {
		return nil, ErrInvalidCedula
	}

	result := &VerifyCedulaResult{
		Tipo:   string(validation.Type),
		Cedula: validation.Cedula,
	}

	// Check if user exists in database
	user, err := s.userRepo.FindByCedula(validation.Cedula)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}

	if user != nil {
		// User exists
		result.Existe = true
		result.TienePassword = user.HasPassword()

		// If user exists but has no password, include basic info
		if !result.TienePassword {
			result.Cliente = map[string]interface{}{
				"id":     user.ID,
				"nombre": user.Nombre,
				"email":  user.Email,
				"activo": user.Activo,
			}
		}

		// Check if we have cached GoMeta data
		if user.Metadata != nil {
			var metadata map[string]interface{}
			if err := json.Unmarshal(user.Metadata, &metadata); err == nil {
				if cedula.CacheValido(metadata) {
					result.ValidadoRegistroCivil = true
					if extras, ok := metadata["extras"].(map[string]interface{}); ok {
						result.DatosRegistroCivil = extractDatosFromMetadata(extras)
					}
					return result, nil
				}
			}
		}
	} else {
		result.Existe = false
		result.TienePassword = false
	}

	// Validate against GoMeta API (for new users or to refresh data)
	goMetaResult, err := s.cedulaService.ValidarCedula(validation.Cedula)
	if err != nil {
		return nil, ErrInvalidCedula
	}

	if goMetaResult.Offline {
		// Service offline - still return basic info
		result.ValidadoRegistroCivil = false
		return result, nil
	}

	if !goMetaResult.Valida {
		// Cedula not found in civil registry
		result.ValidadoRegistroCivil = false
		// If user doesn't exist, this is an error for registration
		if !result.Existe {
			return nil, ErrCedulaNotValid
		}
		return result, nil
	}

	// Cedula is valid - include official data
	result.ValidadoRegistroCivil = true
	nombre, apellido := goMetaResult.GetNombreFormateado()
	result.DatosRegistroCivil = &DatosRegistroCivil{
		Nombre:              nombre,
		Apellido:            apellido,
		NombreCompleto:      goMetaResult.NombreCompleto,
		PrimerNombre:        goMetaResult.PrimerNombre,
		SegundoNombre:       goMetaResult.SegundoNombre,
		PrimerApellido:      goMetaResult.PrimerApellido,
		SegundoApellido:     goMetaResult.SegundoApellido,
		Tipo:                goMetaResult.Tipo,
		TipoIdentificacion:  goMetaResult.TipoIdentificacion,
		SituacionTributaria: goMetaResult.SituacionTributaria,
	}

	return result, nil
}

// Login authenticates a user with cedula and password
func (s *AuthService) Login(identificacion, password string) (*AuthResult, error) {
	validation := utils.ValidateCedula(identificacion)
	if !validation.Valid {
		return nil, ErrInvalidCedula
	}

	user, err := s.userRepo.FindByCedulaWithPassword(validation.Cedula)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrCedulaNotFound
		}
		return nil, err
	}

	if !user.Activo {
		return nil, ErrAccountDisabled
	}

	if !utils.CheckPassword(password, *user.PasswordHash) {
		return nil, ErrInvalidPassword
	}

	// Update last login
	_ = s.userRepo.UpdateLastLogin(user.ID)

	// Generate token
	token, err := utils.GenerateToken(user.ID, user.Cedula, user.Nombre, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		Token:   token,
		Usuario: user.ToPublicJSON(),
	}, nil
}

// Registro creates a new user account with GoMeta validation
func (s *AuthService) Registro(identificacion, nombre, email, telefono, password string) (*AuthResult, error) {
	// Validate cedula format
	validation := utils.ValidateCedula(identificacion)
	if !validation.Valid {
		return nil, ErrInvalidCedula
	}

	// Validate password
	if !utils.ValidatePasswordStrength(password) {
		return nil, ErrWeakPassword
	}

	// Check if account with cedula already exists
	exists, err := s.userRepo.ExistsByCedulaWithPassword(validation.Cedula)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAccountExists
	}

	// Check if email is already used
	emailExists, err := s.userRepo.ExistsByEmailWithPassword(email)
	if err != nil {
		return nil, err
	}
	if emailExists {
		return nil, ErrEmailExists
	}

	// Validate and check telefono uniqueness
	if !utils.ValidateTelefonoCR(telefono) {
		return nil, ErrInvalidTelefono
	}
	telefonoClean := utils.CleanTelefono(telefono)
	telefonoExists, err := s.userRepo.ExistsByTelefonoWithPassword(telefonoClean)
	if err != nil {
		return nil, err
	}
	if telefonoExists {
		return nil, ErrTelefonoExists
	}

	// Validate against GoMeta API
	goMetaResult, err := s.cedulaService.ValidarCedula(validation.Cedula)
	if err != nil {
		return nil, ErrInvalidCedula
	}

	// Handle offline case
	if goMetaResult.Offline {
		if s.config.GoMetaRequireValidation {
			return nil, ErrValidationOffline
		}
		// Allow registration without validation if not required
	} else if !goMetaResult.Valida {
		// Cedula not found in civil registry
		return nil, ErrCedulaNotValid
	}

	// Hash password
	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Use official name from GoMeta if available, otherwise use provided name
	var nombreFinal, apellidoFinal string
	if goMetaResult.Valida && goMetaResult.NombreCompleto != "" {
		nombreFinal, apellidoFinal = goMetaResult.GetNombreFormateado()
		// Para cédulas jurídicas, GoMeta devuelve primerNombre/primerApellido vacíos.
		// Usar NombreCompleto (nombre oficial de la empresa) como nombre.
		if nombreFinal == "" {
			nombreFinal = goMetaResult.NombreCompleto
		}
	} else {
		// Split provided name
		nombreFinal = strings.TrimSpace(nombre)
		if parts := strings.SplitN(nombreFinal, " ", 2); len(parts) > 1 {
			nombreFinal = parts[0]
			apellidoFinal = parts[1]
		}
	}

	// Prepare metadata with GoMeta data
	var metadata datatypes.JSON
	if goMetaResult.Valida {
		metadataMap := goMetaResult.ToMetadata()
		metadataJSON, _ := json.Marshal(metadataMap)
		metadata = metadataJSON
	} else {
		metadata = datatypes.JSON([]byte("{}"))
	}

	// Check if user exists without password (created by admin)
	existingUser, err := s.userRepo.FindByCedula(validation.Cedula)
	telefono = telefonoClean // always store clean digits
	if err == nil && existingUser != nil && !existingUser.HasPassword() {
		// Update existing user
		existingUser.Nombre = nombreFinal
		if apellidoFinal != "" {
			existingUser.Apellido = &apellidoFinal
		}
		existingUser.Email = email
		if telefono != "" {
			existingUser.Telefono = &telefono
		}
		existingUser.PasswordHash = &passwordHash
		existingUser.CedulaType = string(validation.Type)
		existingUser.Activo = true
		existingUser.Metadata = metadata

		if err := s.userRepo.Update(existingUser); err != nil {
			return nil, err
		}

		_ = s.userRepo.UpdateLastLogin(existingUser.ID)
		emailSvc.SendWelcome(existingUser.Email, existingUser.Nombre)

		token, err := utils.GenerateToken(existingUser.ID, existingUser.Cedula, existingUser.Nombre, existingUser.Email, existingUser.Role)
		if err != nil {
			return nil, err
		}

		return &AuthResult{
			Token:   token,
			Usuario: existingUser.ToPublicJSON(),
		}, nil
	}

	// Create new user
	user := &models.User{
		Cedula:       validation.Cedula,
		CedulaType:   string(validation.Type),
		Nombre:       nombreFinal,
		Email:        email,
		PasswordHash: &passwordHash,
		Role:         "customer",
		QuoteQuota:   5,
		QuotesUsed:   0,
		Activo:       true,
		Metadata:     metadata,
	}

	if apellidoFinal != "" {
		user.Apellido = &apellidoFinal
	}

	if telefono != "" {
		user.Telefono = &telefono
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	_ = s.userRepo.UpdateLastLogin(user.ID)
	emailSvc.SendWelcome(user.Email, user.Nombre)

	token, err := utils.GenerateToken(user.ID, user.Cedula, user.Nombre, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		Token:   token,
		Usuario: user.ToPublicJSON(),
	}, nil
}

// EstablecerPassword sets password for a user created by admin (without password)
func (s *AuthService) EstablecerPassword(identificacion, password, email, telefono string) (*AuthResult, error) {
	validation := utils.ValidateCedula(identificacion)
	if !validation.Valid {
		return nil, ErrInvalidCedula
	}

	if !utils.ValidatePasswordStrength(password) {
		return nil, ErrWeakPassword
	}

	// Find user without password
	user, err := s.userRepo.FindByCedula(validation.Cedula)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserHasPassword
		}
		return nil, err
	}

	if user.HasPassword() {
		return nil, ErrUserHasPassword
	}

	// Check if email is used by another user
	if email != "" {
		existingUser, err := s.userRepo.FindByEmail(email)
		if err == nil && existingUser != nil && existingUser.ID != user.ID {
			return nil, ErrEmailExists
		}
	}

	// Validate against GoMeta if no metadata exists
	if user.Metadata == nil || string(user.Metadata) == "{}" || string(user.Metadata) == "null" {
		goMetaResult, _ := s.cedulaService.ValidarCedula(validation.Cedula)
		if goMetaResult != nil && goMetaResult.Valida {
			metadataMap := goMetaResult.ToMetadata()
			metadataJSON, _ := json.Marshal(metadataMap)
			user.Metadata = metadataJSON
		}
	}

	// Hash password
	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Update user
	if err := s.userRepo.SetPassword(user.ID, passwordHash, email, telefono, string(validation.Type)); err != nil {
		return nil, err
	}

	// Update metadata if we have it
	if user.Metadata != nil && string(user.Metadata) != "{}" {
		_ = s.userRepo.UpdateMetadata(user.ID, user.Metadata)
	}

	// Reload user
	user, _ = s.userRepo.FindByID(user.ID)

	token, err := utils.GenerateToken(user.ID, user.Cedula, user.Nombre, user.Email, user.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		Token:   token,
		Usuario: user.ToPublicJSON(),
	}, nil
}

// GetCurrentUser returns the current authenticated user
func (s *AuthService) GetCurrentUser(userID uint) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}

// GetUserByCedula returns a user by cedula (used for email hint in recovery)
func (s *AuthService) GetUserByCedula(identificacion string) (*models.User, error) {
	validation := utils.ValidateCedula(identificacion)
	if !validation.Valid {
		return nil, ErrInvalidCedula
	}
	return s.userRepo.FindByCedulaWithPassword(validation.Cedula)
}

// UpdateProfile updates user profile fields
func (s *AuthService) UpdateProfile(userID uint, email, telefono, direccion, provincia, canton, distrito *string) (*models.User, error) {
	// Check if email is used by another user
	if email != nil && *email != "" {
		existingUser, err := s.userRepo.FindByEmail(*email)
		if err == nil && existingUser != nil && existingUser.ID != userID {
			return nil, ErrEmailExists
		}
	}

	// Update profile
	if err := s.userRepo.UpdateProfile(userID, email, telefono, direccion, provincia, canton, distrito); err != nil {
		return nil, err
	}

	// Return updated user
	return s.userRepo.FindByID(userID)
}

// SolicitarRecuperacion genera un token de reset y envía el email.
// Siempre retorna nil (anti-enumeración: no revela si la cédula existe).
func (s *AuthService) SolicitarRecuperacion(identificacion string) error {
	// Validar formato localmente — si inválido, retornar nil silenciosamente
	validation := utils.ValidateCedula(identificacion)
	if !validation.Valid {
		return nil
	}

	// Buscar usuario con password (solo cuentas activadas pueden hacer reset)
	user, err := s.userRepo.FindByCedulaWithPassword(validation.Cedula)
	if err != nil || user == nil {
		return nil
	}

	// Si no tiene email, no podemos enviar el enlace
	if user.Email == "" {
		return nil
	}

	// Generar token: 32 bytes aleatorios → hex 64 chars
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil
	}
	token := hex.EncodeToString(tokenBytes)
	expires := time.Now().Add(time.Hour)

	// Guardar token (sobreescribe cualquier token previo)
	if err := s.userRepo.SetPasswordResetToken(user.ID, token, expires); err != nil {
		return nil
	}

	// Enviar email en goroutine (no bloqueante)
	emailSvc.SendPasswordReset(user.Email, user.Nombre, token)

	return nil
}

// ResetPassword valida el token y establece la nueva contraseña.
func (s *AuthService) ResetPassword(token, newPassword string) error {
	if token == "" {
		return ErrResetTokenInvalid
	}

	if !utils.ValidatePasswordStrength(newPassword) {
		return ErrWeakPassword
	}

	user, err := s.userRepo.FindByResetToken(token)
	if err != nil {
		return ErrResetTokenInvalid
	}

	passwordHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePassword(user.ID, passwordHash); err != nil {
		return err
	}

	// Limpiar token inmediatamente post-uso
	_ = s.userRepo.ClearPasswordResetToken(user.ID)

	return nil
}

// CambiarPassword permite a un usuario autenticado cambiar su propia contraseña.
// Requiere la contraseña actual para prevenir cambios no autorizados en sesiones abiertas.
func (s *AuthService) CambiarPassword(userID uint, currentPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if user.PasswordHash == nil || !utils.CheckPassword(currentPassword, *user.PasswordHash) {
		return ErrCurrentPasswordWrong
	}

	if !utils.ValidatePasswordStrength(newPassword) {
		return ErrWeakPassword
	}

	passwordHash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(userID, passwordHash)
}

// extractDatosFromMetadata extracts DatosRegistroCivil from metadata extras
func extractDatosFromMetadata(extras map[string]interface{}) *DatosRegistroCivil {
	getString := func(m map[string]interface{}, key string) string {
		if v, ok := m[key].(string); ok {
			return v
		}
		return ""
	}

	primerNombre := getString(extras, "primerNombre")
	segundoNombre := getString(extras, "segundoNombre")
	primerApellido := getString(extras, "primerApellido")
	segundoApellido := getString(extras, "segundoApellido")

	nombre := primerNombre
	if segundoNombre != "" {
		nombre += " " + segundoNombre
	}
	apellido := primerApellido
	if segundoApellido != "" {
		apellido += " " + segundoApellido
	}

	return &DatosRegistroCivil{
		Nombre:              nombre,
		Apellido:            apellido,
		NombreCompleto:      getString(extras, "nombreOficial"),
		PrimerNombre:        primerNombre,
		SegundoNombre:       segundoNombre,
		PrimerApellido:      primerApellido,
		SegundoApellido:     segundoApellido,
		Tipo:                strings.ToLower(getString(extras, "tipo")),
		TipoIdentificacion:  getString(extras, "tipoIdentificacion"),
		SituacionTributaria: getString(extras, "situacionTributaria"),
	}
}
