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
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/quote"
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
		r.Get("/compatible-options", configHandler.GetCompatibleOptions) // Compatible tech/material options
	})

	// Admin routes (protected)
	adminHandler := admin.NewAdminHandler()
	systemConfigHandler := admin.NewSystemConfigHandler()
	techMaterialSpeedHandler := admin.NewTechMaterialSpeedHandler()
	materialCostHandler := admin.NewMaterialCostHandler()
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)
		r.Use(middleware.AdminOnly)

		// Dashboard stats
		r.Get("/dashboard-stats", adminHandler.GetDashboardStats)

		// System Config CRUD
		r.Get("/system-config", systemConfigHandler.GetSystemConfigs)
		r.Get("/system-config/{id}", systemConfigHandler.GetSystemConfig)
		r.Post("/system-config", systemConfigHandler.CreateSystemConfig)
		r.Put("/system-config/{id}", systemConfigHandler.UpdateSystemConfig)
		r.Delete("/system-config/{id}", systemConfigHandler.DeleteSystemConfig)

		// Tech Material Speeds CRUD
		r.Get("/tech-material-speeds", techMaterialSpeedHandler.GetTechMaterialSpeeds)
		r.Get("/tech-material-speeds/{id}", techMaterialSpeedHandler.GetTechMaterialSpeed)
		r.Post("/tech-material-speeds", techMaterialSpeedHandler.CreateTechMaterialSpeed)
		r.Post("/tech-material-speeds/bulk", techMaterialSpeedHandler.BulkCreateTechMaterialSpeeds)
		r.Put("/tech-material-speeds/{id}", techMaterialSpeedHandler.UpdateTechMaterialSpeed)
		r.Delete("/tech-material-speeds/{id}", techMaterialSpeedHandler.DeleteTechMaterialSpeed)

		// Technologies CRUD
		r.Get("/technologies", configHandler.GetTechnologies)
		r.Post("/technologies", adminHandler.CreateTechnology)
		r.Put("/technologies/{id}", adminHandler.UpdateTechnology)
		r.Delete("/technologies/{id}", adminHandler.DeleteTechnology)

		// Materials CRUD
		r.Get("/materials", configHandler.GetMaterials)
		r.Post("/materials", adminHandler.CreateMaterial)
		r.Put("/materials/{id}", adminHandler.UpdateMaterial)
		r.Delete("/materials/{id}", adminHandler.DeleteMaterial)

		// Engrave Types CRUD
		r.Get("/engrave-types", configHandler.GetEngraveTypes)
		r.Post("/engrave-types", adminHandler.CreateEngraveType)
		r.Put("/engrave-types/{id}", adminHandler.UpdateEngraveType)
		r.Delete("/engrave-types/{id}", adminHandler.DeleteEngraveType)

		// Volume Discounts CRUD
		r.Get("/volume-discounts", configHandler.GetVolumeDiscounts)

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

		// User management (full CRUD)
		r.Get("/users", adminHandler.GetUsers)
		r.Post("/users", adminHandler.CreateUser)
		r.Put("/users/{id}", adminHandler.UpdateUser)
		r.Delete("/users/{id}", adminHandler.DeleteUser)

		// Quotes management
		r.Get("/quotes", adminHandler.GetQuotes)
		r.Get("/quotes/{id}", adminHandler.GetQuote)
		r.Put("/quotes/{id}", adminHandler.UpdateQuote)

		// Tech rates (full CRUD)
		r.Get("/tech-rates", adminHandler.GetTechRates)
		r.Post("/tech-rates", adminHandler.CreateTechRate)
		r.Delete("/tech-rates/{id}", adminHandler.DeleteTechRate)

		// Material Costs CRUD
		r.Get("/material-costs", materialCostHandler.GetMaterialCosts)
		r.Get("/material-costs/{id}", materialCostHandler.GetMaterialCost)
		r.Post("/material-costs", materialCostHandler.CreateMaterialCost)
		r.Put("/material-costs/{id}", materialCostHandler.UpdateMaterialCost)
		r.Delete("/material-costs/{id}", materialCostHandler.DeleteMaterialCost)
		r.Post("/material-costs/{id}/recalculate", materialCostHandler.RecalculateMaterialCost)
	})

	// Quote routes (Fase 1 - Cotizador)
	quoteHandler := quote.NewHandler()
	r.Route("/api/v1/quotes", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		// GET endpoints (no quota check)
		r.Get("/my", quoteHandler.GetMyQuotes)       // List user's quotes
		r.Get("/analyses", quoteHandler.GetMyAnalyses) // List user's SVG analyses
		r.Get("/{id}", quoteHandler.GetQuote)        // Get specific quote

		// POST endpoints (with quota check)
		r.Group(func(r chi.Router) {
			r.Use(middleware.QuotaMiddleware)
			r.Post("/analyze", quoteHandler.AnalyzeSVG)      // Upload and analyze SVG
			r.Post("/calculate", quoteHandler.CalculatePrice) // Calculate price for analysis
		})
	})

	// Static file routes
	webDir := "/opt/FabricaLaser/web"

	// Landing page
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "landing", "index.html"))
	})
	r.Get("/landing", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "landing", "index.html"))
	})
	r.Get("/landing/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "landing", "index.html"))
	})

	// Admin pages (redirect /admin to /admin/ for correct relative paths)
	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	r.Get("/admin/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "admin", "index.html"))
	})
	r.Handle("/admin/*", http.StripPrefix("/admin/", http.FileServer(http.Dir(filepath.Join(webDir, "admin")))))

	// Mi cuenta page
	r.Get("/mi-cuenta", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "mi-cuenta", "index.html"))
	})

	// Cotizar page (Phase 1 - requires auth via JS)
	r.Get("/cotizar", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "cotizar", "index.html"))
	})
	r.Get("/cotizar/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "cotizar", "index.html"))
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
