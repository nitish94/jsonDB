package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	DefaultLimit int
	MaxLimit     int
	Username     string
	Password     string
	RecordLimit  int
	JWTSecret    string
}

var GlobalConfig *Config

func Init() {
	GlobalConfig = &Config{
		Port:         getEnv("PORT", "5000"),
		DefaultLimit: getEnvInt("DEFAULT_LIMIT", 25),
		MaxLimit:     getEnvInt("MAX_LIMIT", 50),
		Username:     getEnv("USERNAME", "admin"),
		Password:     getEnv("PASSWORD", "admin"),
		RecordLimit:  getEnvInt("RECORD_LIMIT", 1000),
		JWTSecret:    getEnv("JWT_SECRET", "your-secret-key"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}