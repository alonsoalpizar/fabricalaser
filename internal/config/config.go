package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	Environment string

	// Database
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string

	// JWT
	JWTSecret string

	// Redis
	RedisAddr string
	RedisDB   int

	// Uploads
	UploadDir   string
	MaxFileSize int64
}

var cfg *Config

func Load() *Config {
	if cfg != nil {
		return cfg
	}

	redisDB, _ := strconv.Atoi(getEnv("FABRICALASER_REDIS_DB", "3"))
	maxFileSize, _ := strconv.ParseInt(getEnv("FABRICALASER_MAX_FILE_SIZE", "10485760"), 10, 64)

	cfg = &Config{
		Port:        getEnv("FABRICALASER_PORT", "8083"),
		Environment: getEnv("FABRICALASER_ENV", "development"),

		DBHost:     getEnv("FABRICALASER_DB_HOST", "localhost"),
		DBPort:     getEnv("FABRICALASER_DB_PORT", "5432"),
		DBName:     getEnv("FABRICALASER_DB_NAME", "fabricalaser"),
		DBUser:     getEnv("FABRICALASER_DB_USER", "fabricalaser"),
		DBPassword: getEnv("FABRICALASER_DB_PASSWORD", ""),

		JWTSecret: getEnv("FABRICALASER_JWT_SECRET", ""),

		RedisAddr: getEnv("FABRICALASER_REDIS_ADDR", "localhost:6379"),
		RedisDB:   redisDB,

		UploadDir:   getEnv("FABRICALASER_UPLOAD_DIR", "/opt/FabricaLaser/uploads"),
		MaxFileSize: maxFileSize,
	}

	return cfg
}

func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}
