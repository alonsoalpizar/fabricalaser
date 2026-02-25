package middleware

import (
	"net/http"
)

// RoleMiddleware checks if the user has the required role
func RoleMiddleware(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := r.Context().Value("userRole")
			if userRole == nil {
				respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "No autenticado")
				return
			}

			if userRole.(string) != requiredRole {
				respondAuthError(w, http.StatusForbidden, "FORBIDDEN", "No tiene permisos para esta acción")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnly is a convenience middleware for admin-only routes
func AdminOnly(next http.Handler) http.Handler {
	return RoleMiddleware("admin")(next)
}
