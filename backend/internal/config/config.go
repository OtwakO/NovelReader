// config holds server configuration loaded from env or config file.
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                    int
	DataDir                 string
	ReadTimeout             time.Duration
	PublicURL               string
	AdminBootstrapToken     string
	AdminRecoveryToken      string
	WebViewEndpoint         string
	SearchConcurrency       int
	GlobalSearchConcurrency int
	JSPoolSize              int
	MaxSessions             int
	SessionTTL              time.Duration
	ExploreSourceTimeout    time.Duration
}

func Load() *Config {
	dataDir := getEnv("DATA_DIR", "./data")
	return &Config{
		Port:                    getEnvInt("PORT", 8888),
		DataDir:                 dataDir,
		ReadTimeout:             time.Duration(getEnvInt("READ_TIMEOUT_SECONDS", 30)) * time.Second,
		PublicURL:               getEnv("PUBLIC_URL", ""),
		AdminBootstrapToken:     getEnv("ADMIN_BOOTSTRAP_TOKEN", ""),
		AdminRecoveryToken:      getEnv("ADMIN_RECOVERY_TOKEN", ""),
		WebViewEndpoint:         getEnv("WEBVIEW_ENDPOINT", ""),
		SearchConcurrency:       getEnvPositiveInt("SEARCH_CONCURRENCY", 16),
		GlobalSearchConcurrency: getEnvPositiveInt("GLOBAL_SEARCH_CONCURRENCY", 32),
		JSPoolSize:              getEnvPositiveInt("JS_POOL_SIZE", 4),
		MaxSessions:             getEnvPositiveInt("MAX_WORKFLOW_SESSIONS", 1024),
		SessionTTL:              time.Duration(getEnvPositiveInt("SESSION_TTL_MINUTES", 30)) * time.Minute,
		ExploreSourceTimeout:    time.Duration(getEnvPositiveInt("EXPLORE_SOURCE_TIMEOUT_SECONDS", 30)) * time.Second,
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

func getEnvPositiveInt(key string, fallback int) int {
	value := getEnvInt(key, fallback)
	if value < 1 {
		return fallback
	}
	return value
}
