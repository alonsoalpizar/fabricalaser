// cmd/digest/main.go — one-shot WhatsApp digest runner for cron.
// Usage: ./bin/fabricalaser-digest
package main

import (
	"log"
	"os"

	"github.com/alonsoalpizar/fabricalaser/internal/agent/llm"
	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/repository"
	"github.com/alonsoalpizar/fabricalaser/internal/whatsapp"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	if _, err := database.Connect(); err != nil {
		log.Fatalf("DB: %v", err)
	}
	defer database.Close()

	rc, err := database.ConnectRedis()
	if err != nil {
		log.Fatalf("Redis: %v", err)
	}

	// APP_SECRET requerido para desencriptar la API key del LLM activo desde system_config.
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

	if err := whatsapp.SendDigest(rc, llmFactory); err != nil {
		log.Fatalf("SendDigest: %v", err)
	}
}
