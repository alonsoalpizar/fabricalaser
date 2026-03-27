// cmd/digest/main.go — one-shot WhatsApp digest runner for cron.
// Usage: ./bin/fabricalaser-digest
package main

import (
	"log"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
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

	if err := whatsapp.SendDigest(rc); err != nil {
		log.Fatalf("SendDigest: %v", err)
	}
}
