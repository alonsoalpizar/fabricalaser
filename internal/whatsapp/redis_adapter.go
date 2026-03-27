package whatsapp

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisAdapter struct {
	client *redis.Client
}

// NewRedisAdapter crea un RedisClient que implementa la interfaz usando go-redis/v9.
func NewRedisAdapter(client *redis.Client) RedisClient {
	return &redisAdapter{client: client}
}

func (r *redisAdapter) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, ttl).Result()
}

func (r *redisAdapter) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// Clave no encontrada es estado válido — retornar error genérico para que el caller lo maneje
		return "", err
	}
	return val, err
}

func (r *redisAdapter) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *redisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}
