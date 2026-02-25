package utils

import (
	"errors"
	"strings"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("token inválido")
	ErrExpiredToken = errors.New("sesión expirada")
	ErrMissingToken = errors.New("token requerido")
)

// TokenClaims represents the JWT claims
type TokenClaims struct {
	ID     uint   `json:"id"`
	Cedula string `json:"cedula"`
	Nombre string `json:"nombre"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Tipo   string `json:"tipo"` // "customer" for user tokens
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT token for a user
func GenerateToken(id uint, cedula, nombre, email, role string) (string, error) {
	cfg := config.Get()

	claims := TokenClaims{
		ID:     id,
		Cedula: cedula,
		Nombre: nombre,
		Email:  email,
		Role:   role,
		Tipo:   "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "fabricalaser",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ValidateToken parses and validates a JWT token
func ValidateToken(tokenString string) (*TokenClaims, error) {
	cfg := config.Get()

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(cfg.JWTSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ExtractTokenFromHeader extracts the token from Authorization header
// Format: "Bearer <token>"
func ExtractTokenFromHeader(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}
