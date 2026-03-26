package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/config"
	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

// ConnectRedis inicializa el cliente Redis y verifica la conexión.
func ConnectRedis() (*redis.Client, error) {
	cfg := config.Get()

	rdb = redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
		DB:   cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: failed to connect to %s db %d: %w", cfg.RedisAddr, cfg.RedisDB, err)
	}

	log.Printf("Redis connected: %s db=%d", cfg.RedisAddr, cfg.RedisDB)
	return rdb, nil
}

// GetRedis retorna el cliente Redis global.
func GetRedis() *redis.Client {
	return rdb
}
