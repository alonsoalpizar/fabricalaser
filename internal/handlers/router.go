package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/admin"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/auth"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/chat"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/config"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers/quote"
	"github.com/alonsoalpizar/fabricalaser/internal/middleware"
	"github.com/alonsoalpizar/fabricalaser/internal/whatsapp"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

const Version = "1.0.0"

func NewRouter(redisClient *redis.Client) *chi.Mux {
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
		r.Post("/solicitar-recuperacion", authHandler.SolicitarRecuperacion)
		r.Post("/reset-password", authHandler.ResetPassword)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware)
			r.Get("/me", authHandler.Me)
			r.Get("/profile", authHandler.GetProfile)
			r.Put("/profile", authHandler.UpdateProfile)
			r.Put("/change-password", authHandler.ChangePassword)
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

		// WhatsApp bitácora — sesiones paginadas + depuración + digest manual
		waAdminHandler := admin.NewWhatsappHandler(redisClient)
		r.Get("/whatsapp/sessions", waAdminHandler.GetSessions)
		r.Get("/whatsapp/sessions/{phone}/{date}", waAdminHandler.GetSessionMessages)
		r.Post("/whatsapp/purge", waAdminHandler.PurgeConversations)
		r.Post("/whatsapp/digest/send", waAdminHandler.SendDigest)
		// Legacy
		r.Get("/whatsapp/conversations", waAdminHandler.GetConversations)
		r.Get("/whatsapp/conversations/{phone}", waAdminHandler.GetConversation)
	})

	// WhatsApp webhook
	waContextProvider := whatsapp.NewWAContextProvider()
	waHandler := whatsapp.NewHandler(
		whatsapp.NewRedisAdapter(redisClient),
		whatsapp.NewPGAdapter(database.Get()),
		whatsapp.NewGeminiAdapter(waContextProvider),
		whatsapp.NewRateLimiter(redisClient),
		waContextProvider,
	)
	r.Route("/api/v1/whatsapp", func(r chi.Router) {
		r.Get("/webhook", waHandler.VerifyWebhook)
		r.Post("/webhook", waHandler.HandleMessage)
	})

	// Chat route (public - auth optional, enriches context if logged in)
	chatHandler := chat.NewHandler()
	r.Route("/api/v1/chat", func(r chi.Router) {
		r.Use(middleware.AuthOptional)
		r.Post("/", chatHandler.HandleChat)
		r.Post("/summary", chatHandler.HandleSummary)
	})

	// Quote routes (Fase 1 - Cotizador)
	quoteHandler := quote.NewHandler()

	r.Route("/api/v1/quotes", func(r chi.Router) {
		// Estimate — token interno, sin JWT (usado por el agente de WhatsApp)
		// Usa r.Post directo — r.Group+r.Use propaga al padre en chi inline mux
		r.Post("/estimate", quoteHandler.HandleEstimate)

		// GET endpoints — requieren JWT (r.With no propaga al padre)
		r.With(middleware.AuthMiddleware).Get("/my", quoteHandler.GetMyQuotes)
		r.With(middleware.AuthMiddleware).Get("/analyses", quoteHandler.GetMyAnalyses)
		r.With(middleware.AuthMiddleware).Get("/analyses/{id}/svg", quoteHandler.GetAnalysisSVG)
		r.With(middleware.AuthMiddleware).Get("/{id}", quoteHandler.GetQuote)

		// POST endpoints — requieren JWT + cuota
		r.With(middleware.AuthMiddleware, middleware.QuotaMiddleware).Post("/analyze", quoteHandler.AnalyzeSVG)
		r.With(middleware.AuthMiddleware, middleware.QuotaMiddleware).Post("/calculate", quoteHandler.CalculatePrice)
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

	// SEO files
	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "robots.txt"))
	})
	r.Get("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "sitemap.xml"))
	})
	r.Get("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "favicon.svg"))
	})
	r.Get("/logo-oficial.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "logo-oficial.svg"))
	})
	r.Get("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "logo.png"))
	})
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "favicon.ico"))
	})
	r.Get("/googleeb4aa376b55ad413.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "googleeb4aa376b55ad413.html"))
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

	// Reset password page
	r.Get("/reset-password", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "reset-password", "index.html"))
	})

	// Cotizar page (Phase 1 - requires auth via JS)
	r.Get("/cotizar", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "cotizar", "index.html"))
	})
	r.Get("/cotizar/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "cotizar", "index.html"))
	})

	// Documentation pages
	r.Get("/docs/pricing", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "docs", "pricing.html"))
	})
	r.Get("/docs/pricing/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/pricing", http.StatusMovedPermanently)
	})

	// Catálogo de Blanks
	r.Get("/catalogo", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/catalogo/", http.StatusMovedPermanently)
	})
	r.Get("/catalogo/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(webDir, "catalogo", "index.html"))
	})

	// Static assets
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(webDir, "assets")))))
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
