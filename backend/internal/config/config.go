// config holds server configuration loaded from env or config file.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port         int
	DatabasePath string
	DataDir      string
	ReadTimeout  time.Duration
	CORSOrigin   string
}

func Load() *Config {
	return &Config{
		Port:         getEnvInt("PORT", 8888),
		DatabasePath: getEnv("DATABASE_PATH", "./data/novelreader.db"),
		DataDir:      getEnv("DATA_DIR", "./data"),
		ReadTimeout:  time.Duration(getEnvInt("READ_TIMEOUT_SECONDS", 30)) * time.Second,
		CORSOrigin:   getEnv("CORS_ORIGIN", "*"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
