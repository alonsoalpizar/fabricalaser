package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/alonsoalpizar/fabricalaser/internal/agent/llm"
	"github.com/alonsoalpizar/fabricalaser/internal/agent/prompts"
	"github.com/alonsoalpizar/fabricalaser/internal/config"
	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/alonsoalpizar/fabricalaser/internal/whatsapp"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()

	log.Printf("FabricaLaser API v%s", handlers.Version)
	log.Printf("Environment: %s", cfg.Environment)

	// Connect to database
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Verify connection
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Database connection established")

	// Connect to Redis
	redisClient, err := database.ConnectRedis()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Start WhatsApp digest email scheduler (every 4 hours)
	whatsapp.StartDigestScheduler(redisClient)

	// LLM factory — único cliente activo compartido por todos los handlers
	// (chat web, WhatsApp, Telegram, admin chat). Permite hot reload de proveedor
	// desde el admin UI sin reiniciar el servicio.
	appSecret := os.Getenv("FABRICALASER_APP_SECRET")
	if appSecret == "" {
		appSecret = os.Getenv("APP_SECRET")
	}
	if appSecret == "" {
		log.Fatalf("APP_SECRET (o FABRICALASER_APP_SECRET) requerido para inicializar LLM factory")
	}
	sysConfigRepo := repository.NewSystemConfigRepository()
	llmFactory, err := llm.NewFactory(sysConfigRepo, appSecret)
	if err != nil {
		log.Fatalf("LLM factory: %v", err)
	}

	// Prompts provider — sirve los system prompts de los 3 agentes (chat web,
	// WhatsApp/Telegram, admin chat) desde DB con fallback hardcoded. Hot reload
	// via pub/sub Redis: al guardar un prompt desde /admin/prompts.html, el
	// servicio invalida su cache local en segundos sin reiniciar.
	promptRepo := repository.NewAgentPromptRepository()
	promptProvider := prompts.New(promptRepo, redisClient, prompts.Fallbacks())
	if err := promptProvider.Seed(); err != nil {
		// No fatal — Get() caerá al fallback hardcoded si DB tiene bodies vacíos.
		log.Printf("prompts seed: %v (usando fallbacks)", err)
	}
	promptProvider.Warmup() // precarga los 5 bodies en cache; first request no espera DB
	go promptProvider.Subscribe(context.Background())

	// Setup router
	router := handlers.NewRouter(redisClient, llmFactory, promptProvider)

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	log.Printf("Health check: http://localhost%s/api/v1/health", addr)
	log.Printf("Auth endpoints: http://localhost%s/api/v1/auth/*", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func init() {
	// Ensure we're in the right directory
	if _, err := os.Stat("/opt/FabricaLaser"); os.IsNotExist(err) {
		log.Println("Warning: /opt/FabricaLaser directory not found")
	}
}
