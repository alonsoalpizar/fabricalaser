package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/alonsoalpizar/fabricalaser/internal/utils"
)

// AuthMiddleware verifies JWT token and adds user info to context
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from header
		token := utils.ExtractTokenFromHeader(r.Header.Get("Authorization"))
		if token == "" {
			respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Token de autenticación requerido")
			return
		}

		// Validate token
		claims, err := utils.ValidateToken(token)
		if err != nil {
			code := "INVALID_TOKEN"
			message := "Token inválido"

			if err == utils.ErrExpiredToken {
				code = "TOKEN_EXPIRED"
				message = "Sesión expirada. Por favor inicie sesión nuevamente."
			}

			respondAuthError(w, http.StatusUnauthorized, code, message)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), "userID", claims.ID)
		ctx = context.WithValue(ctx, "userCedula", claims.Cedula)
		ctx = context.WithValue(ctx, "userName", claims.Nombre)
		ctx = context.WithValue(ctx, "userEmail", claims.Email)
		ctx = context.WithValue(ctx, "userRole", claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthOptional middleware - doesn't fail if no token, just adds info if present
func AuthOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := utils.ExtractTokenFromHeader(r.Header.Get("Authorization"))
		if token != "" {
			claims, err := utils.ValidateToken(token)
			if err == nil {
				ctx := context.WithValue(r.Context(), "userID", claims.ID)
				ctx = context.WithValue(ctx, "userCedula", claims.Cedula)
				ctx = context.WithValue(ctx, "userName", claims.Nombre)
				ctx = context.WithValue(ctx, "userEmail", claims.Email)
				ctx = context.WithValue(ctx, "userRole", claims.Role)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func respondAuthError(w http.ResponseWriter, status int, code, message string) {
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
