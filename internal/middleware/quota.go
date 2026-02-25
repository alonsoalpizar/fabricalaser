package middleware

import (
	"net/http"

	"github.com/alonsoalpizar/fabricalaser/internal/repository"
)

// QuotaMiddleware checks if the user has remaining quote quota
func QuotaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("userID")
		if userID == nil {
			respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "No autenticado")
			return
		}

		userRepo := repository.NewUserRepository()
		user, err := userRepo.FindByID(userID.(uint))
		if err != nil {
			respondAuthError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "Usuario no encontrado")
			return
		}

		if !user.CanQuote() {
			respondAuthError(w, http.StatusForbidden, "QUOTA_EXCEEDED", "Ha alcanzado el límite de cotizaciones. Contacte al administrador para extender su cuota.")
			return
		}

		next.ServeHTTP(w, r)
	})
}
