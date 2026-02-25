package main

import (
	"log"
	"net/http"
	"os"

	"github.com/alonsoalpizar/fabricalaser/internal/config"
	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/handlers"
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

	// Setup router
	router := handlers.NewRouter()

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
