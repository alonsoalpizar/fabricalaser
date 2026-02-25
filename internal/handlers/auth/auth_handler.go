package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	authService "github.com/alonsoalpizar/fabricalaser/internal/services/auth"
)

type AuthHandler struct {
	service *authService.AuthService
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		service: authService.NewAuthService(),
	}
}

// Response helpers
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  data,
		"error": nil,
	})
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": nil,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// VerificarCedula handles POST /api/v1/auth/verificar-cedula
func (h *AuthHandler) VerificarCedula(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identificacion string `json:"identificacion"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Identificacion == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELD", "La identificación es requerida")
		return
	}

	result, err := h.service.VerificarCedula(req.Identificacion)
	if err != nil {
		code := "INVALID_CEDULA"
		status := http.StatusBadRequest

		switch err {
		case authService.ErrCedulaNotValid:
			code = "CEDULA_NOT_VALID"
		case authService.ErrValidationOffline:
			code = "VALIDATION_OFFLINE"
			status = http.StatusServiceUnavailable
		}

		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identificacion string `json:"identificacion"`
		Password       string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Identificacion == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELD", "Identificación y contraseña son requeridas")
		return
	}

	result, err := h.service.Login(req.Identificacion, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		code := "AUTH_ERROR"

		switch err {
		case authService.ErrInvalidCedula:
			status = http.StatusBadRequest
			code = "INVALID_CEDULA"
		case authService.ErrCedulaNotFound:
			code = "NOT_FOUND"
		case authService.ErrInvalidPassword:
			code = "INVALID_PASSWORD"
		case authService.ErrAccountDisabled:
			code = "ACCOUNT_DISABLED"
		}

		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// Registro handles POST /api/v1/auth/registro
func (h *AuthHandler) Registro(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identificacion string `json:"identificacion"`
		Nombre         string `json:"nombre"`
		Email          string `json:"email"`
		Telefono       string `json:"telefono"`
		Password       string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	// Validations
	if req.Identificacion == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELD", "La identificación es requerida")
		return
	}

	if strings.TrimSpace(req.Nombre) == "" || len(strings.TrimSpace(req.Nombre)) < 2 {
		respondError(w, http.StatusBadRequest, "INVALID_NAME", "El nombre es requerido (mínimo 2 caracteres)")
		return
	}

	if req.Email == "" || !isValidEmail(req.Email) {
		respondError(w, http.StatusBadRequest, "INVALID_EMAIL", "Email válido es requerido")
		return
	}

	if req.Password == "" || len(req.Password) < 6 {
		respondError(w, http.StatusBadRequest, "WEAK_PASSWORD", "La contraseña debe tener al menos 6 caracteres")
		return
	}

	result, err := h.service.Registro(
		req.Identificacion,
		strings.TrimSpace(req.Nombre),
		strings.ToLower(strings.TrimSpace(req.Email)),
		strings.TrimSpace(req.Telefono),
		req.Password,
	)

	if err != nil {
		status := http.StatusBadRequest
		code := "REGISTRATION_ERROR"

		switch err {
		case authService.ErrAccountExists:
			code = "CEDULA_EXISTS"
		case authService.ErrEmailExists:
			code = "EMAIL_EXISTS"
		case authService.ErrInvalidCedula:
			code = "INVALID_CEDULA"
		case authService.ErrWeakPassword:
			code = "WEAK_PASSWORD"
		case authService.ErrCedulaNotValid:
			code = "CEDULA_NOT_VALID"
		case authService.ErrValidationOffline:
			code = "VALIDATION_OFFLINE"
			status = http.StatusServiceUnavailable
		}

		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

// EstablecerPassword handles POST /api/v1/auth/establecer-password
func (h *AuthHandler) EstablecerPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identificacion string `json:"identificacion"`
		Password       string `json:"password"`
		Email          string `json:"email"`
		Telefono       string `json:"telefono"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	if req.Identificacion == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELD", "La identificación es requerida")
		return
	}

	if req.Password == "" || len(req.Password) < 6 {
		respondError(w, http.StatusBadRequest, "WEAK_PASSWORD", "La contraseña debe tener al menos 6 caracteres")
		return
	}

	if req.Email != "" && !isValidEmail(req.Email) {
		respondError(w, http.StatusBadRequest, "INVALID_EMAIL", "Formato de email inválido")
		return
	}

	result, err := h.service.EstablecerPassword(
		req.Identificacion,
		req.Password,
		strings.ToLower(strings.TrimSpace(req.Email)),
		strings.TrimSpace(req.Telefono),
	)

	if err != nil {
		status := http.StatusBadRequest
		code := "SET_PASSWORD_ERROR"

		switch err {
		case authService.ErrUserHasPassword:
			code = "USER_HAS_PASSWORD"
			status = http.StatusNotFound
		case authService.ErrEmailExists:
			code = "EMAIL_EXISTS"
		case authService.ErrInvalidCedula:
			code = "INVALID_CEDULA"
		}

		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// Me handles GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userID := r.Context().Value("userID")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "No autenticado")
		return
	}

	user, err := h.service.GetCurrentUser(userID.(uint))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Usuario no encontrado")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"usuario": user.ToPublicJSON(),
	})
}

// GetProfile handles GET /api/v1/auth/profile
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "No autenticado")
		return
	}

	user, err := h.service.GetCurrentUser(userID.(uint))
	if err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "Usuario no encontrado")
		return
	}

	// Return full profile including address fields
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":              user.ID,
		"cedula":          user.Cedula,
		"cedula_type":     user.CedulaType,
		"nombre":          user.Nombre,
		"apellido":        user.Apellido,
		"nombre_completo": user.NombreCompleto(),
		"email":           user.Email,
		"telefono":        user.Telefono,
		"role":            user.Role,
		"quote_quota":     user.QuoteQuota,
		"quotes_used":     user.QuotesUsed,
		"remaining_quotes": user.RemainingQuotes(),
		"direccion":       user.Direccion,
		"provincia":       user.Provincia,
		"canton":          user.Canton,
		"distrito":        user.Distrito,
		"activo":          user.Activo,
		"created_at":      user.CreatedAt,
	})
}

// UpdateProfile handles PUT /api/v1/auth/profile
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "No autenticado")
		return
	}

	var req struct {
		Email     *string `json:"email"`
		Telefono  *string `json:"telefono"`
		Direccion *string `json:"direccion"`
		Provincia *string `json:"provincia"`
		Canton    *string `json:"canton"`
		Distrito  *string `json:"distrito"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_JSON", "JSON inválido")
		return
	}

	// Validate email if provided
	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*req.Email))
		if email == "" || !isValidEmail(email) {
			respondError(w, http.StatusBadRequest, "INVALID_EMAIL", "Email válido es requerido")
			return
		}
		req.Email = &email
	}

	// Trim optional fields
	if req.Telefono != nil {
		tel := strings.TrimSpace(*req.Telefono)
		req.Telefono = &tel
	}
	if req.Direccion != nil {
		dir := strings.TrimSpace(*req.Direccion)
		req.Direccion = &dir
	}
	if req.Provincia != nil {
		prov := strings.TrimSpace(*req.Provincia)
		req.Provincia = &prov
	}
	if req.Canton != nil {
		cant := strings.TrimSpace(*req.Canton)
		req.Canton = &cant
	}
	if req.Distrito != nil {
		dist := strings.TrimSpace(*req.Distrito)
		req.Distrito = &dist
	}

	user, err := h.service.UpdateProfile(
		userID.(uint),
		req.Email,
		req.Telefono,
		req.Direccion,
		req.Provincia,
		req.Canton,
		req.Distrito,
	)

	if err != nil {
		code := "UPDATE_ERROR"
		status := http.StatusBadRequest

		if err.Error() == "ya existe una cuenta con este email" {
			code = "EMAIL_EXISTS"
		}

		respondError(w, status, code, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Perfil actualizado correctamente",
		"usuario": map[string]interface{}{
			"id":              user.ID,
			"cedula":          user.Cedula,
			"cedula_type":     user.CedulaType,
			"nombre":          user.Nombre,
			"apellido":        user.Apellido,
			"nombre_completo": user.NombreCompleto(),
			"email":           user.Email,
			"telefono":        user.Telefono,
			"direccion":       user.Direccion,
			"provincia":       user.Provincia,
			"canton":          user.Canton,
			"distrito":        user.Distrito,
		},
	})
}

// isValidEmail checks if email format is valid
func isValidEmail(email string) bool {
	// Simple check - contains @ and at least one . after @
	at := strings.Index(email, "@")
	if at < 1 {
		return false
	}
	dot := strings.LastIndex(email, ".")
	return dot > at+1 && dot < len(email)-1
}
