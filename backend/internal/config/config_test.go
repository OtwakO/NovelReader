package config

import (
	"testing"
	"time"
)

func TestLoadUsesSmallContainerCapacityDefaults(t *testing.T) {
	for _, key := range []string{
		"SEARCH_CONCURRENCY", "GLOBAL_SEARCH_CONCURRENCY", "JS_POOL_SIZE",
		"MAX_WORKFLOW_SESSIONS", "SESSION_TTL_MINUTES",
	} {
		t.Setenv(key, "")
	}
	cfg := Load()
	if cfg.SearchConcurrency != 16 || cfg.GlobalSearchConcurrency != 32 || cfg.JSPoolSize != 4 {
		t.Fatalf("execution limits=%d/%d/%d", cfg.SearchConcurrency, cfg.GlobalSearchConcurrency, cfg.JSPoolSize)
	}
	if cfg.MaxSessions != 1024 || cfg.SessionTTL != 30*time.Minute {
		t.Fatalf("session limits=%d/%s", cfg.MaxSessions, cfg.SessionTTL)
	}
}

func TestLoadAcceptsCapacityOverridesAndRejectsNonPositiveValues(t *testing.T) {
	t.Setenv("SEARCH_CONCURRENCY", "24")
	t.Setenv("GLOBAL_SEARCH_CONCURRENCY", "0")
	t.Setenv("JS_POOL_SIZE", "8")
	t.Setenv("MAX_WORKFLOW_SESSIONS", "2048")
	t.Setenv("SESSION_TTL_MINUTES", "60")
	cfg := Load()
	if cfg.SearchConcurrency != 24 || cfg.GlobalSearchConcurrency != 32 || cfg.JSPoolSize != 8 {
		t.Fatalf("execution limits=%d/%d/%d", cfg.SearchConcurrency, cfg.GlobalSearchConcurrency, cfg.JSPoolSize)
	}
	if cfg.MaxSessions != 2048 || cfg.SessionTTL != time.Hour {
		t.Fatalf("session limits=%d/%s", cfg.MaxSessions, cfg.SessionTTL)
	}
}
