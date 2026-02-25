package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/handlers/admin"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/auth"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/config"
	"github.com/alonsoalpizar/fabricalaser/internal/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

const Version = "1.0.0"

func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(60 * time.Second))
	r.Use(middleware.CORS)

	// Health check
	r.Get("/api/v1/health", healthHandler)

	// Auth routes (public)
	authHandler := auth.NewAuthHandler()
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/verificar-cedula", authHandler.VerificarCedula)
		r.Post("/login", authHandler.Login)
		r.Post("/registro", authHandler.Registro)
		r.Post("/establecer-password", authHandler.EstablecerPassword)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware)
			r.Get("/me", authHandler.Me)
			r.Get("/profile", authHandler.GetProfile)
			r.Put("/profile", authHandler.UpdateProfile)
		})
	})

	// Config routes (public - for quoter)
	configHandler := config.NewConfigHandler()
	r.Route("/api/v1/config", func(r chi.Router) {
		r.Get("/", configHandler.GetAll)                        // All config in one call
		r.Get("/technologies", configHandler.GetTechnologies)   // Technologies list
		r.Get("/materials", configHandler.GetMaterials)         // Materials list
		r.Get("/engrave-types", configHandler.GetEngraveTypes)  // Engrave types list
		r.Get("/tech-rates", configHandler.GetTechRates)        // Tech rates with technology
		r.Get("/volume-discounts", configHandler.GetVolumeDiscounts)
		r.Get("/price-references", configHandler.GetPriceReferences)
	})

	// Admin routes (protected)
	adminHandler := admin.NewAdminHandler()
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)
		r.Use(middleware.AdminOnly)

		// Technologies CRUD
		r.Post("/technologies", adminHandler.CreateTechnology)
		r.Put("/technologies/{id}", adminHandler.UpdateTechnology)
		r.Delete("/technologies/{id}", adminHandler.DeleteTechnology)

		// Materials CRUD
		r.Post("/materials", adminHandler.CreateMaterial)
		r.Put("/materials/{id}", adminHandler.UpdateMaterial)
		r.Delete("/materials/{id}", adminHandler.DeleteMaterial)

		// Engrave Types CRUD
		r.Post("/engrave-types", adminHandler.CreateEngraveType)
		r.Put("/engrave-types/{id}", adminHandler.UpdateEngraveType)
		r.Delete("/engrave-types/{id}", adminHandler.DeleteEngraveType)

		// Tech Rates (update only - created via seed)
		r.Put("/tech-rates/{id}", adminHandler.UpdateTechRate)

		// Volume Discounts CRUD
		r.Post("/volume-discounts", adminHandler.CreateVolumeDiscount)
		r.Put("/volume-discounts/{id}", adminHandler.UpdateVolumeDiscount)
		r.Delete("/volume-discounts/{id}", adminHandler.DeleteVolumeDiscount)

		// Price References CRUD
		r.Post("/price-references", adminHandler.CreatePriceReference)
		r.Put("/price-references/{id}", adminHandler.UpdatePriceReference)
		r.Delete("/price-references/{id}", adminHandler.DeletePriceReference)

		// User management
		r.Get("/users", adminHandler.GetUsers)
		r.Put("/users/{id}/quota", adminHandler.UpdateUserQuota)
	})

	// TODO: Quote routes (Fase 1)
	// r.Route("/api/v1/quotes", func(r chi.Router) {
	//     r.Use(middleware.AuthMiddleware)
	//     r.Use(middleware.QuotaMiddleware) // For POST endpoints
	//     // Quote endpoints...
	// })

	// Static file routes
	webDir := "/opt/FabricaLaser/web"

	// Landing page
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "landing", "index.html"))
	})

	// Mi cuenta page
	r.Get("/mi-cuenta", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "mi-cuenta", "index.html"))
	})

	// Cotizar page (placeholder until Phase 1)
	r.Get("/cotizar", func(w http.ResponseWriter, r *http.Request) {
		// For now, redirect to landing
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	})

	// Static assets (if needed in future)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(webDir, "static")))))

	return r
}

// ensureDir checks if directory exists
func ensureDir(dir string) bool {
	_, err := os.Stat(dir)
	return err == nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": Version,
		"service": "fabricalaser-api",
	})
}
