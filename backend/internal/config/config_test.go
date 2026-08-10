package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReadsPublicOriginConfiguration(t *testing.T) {
	t.Setenv("PUBLIC_URL", "https://reader.example")
	t.Setenv("ADMIN_BOOTSTRAP_TOKEN", "temporary-setup-authority")
	cfg := Load()
	if cfg.PublicURL != "https://reader.example" || cfg.AdminBootstrapToken != "temporary-setup-authority" {
		t.Fatalf("origin/bootstrap config=%q/%q", cfg.PublicURL, cfg.AdminBootstrapToken)
	}
}

func TestLoadUsesSmallContainerCapacityDefaults(t *testing.T) {
	for _, key := range []string{
		"SEARCH_CONCURRENCY", "GLOBAL_SEARCH_CONCURRENCY", "JS_POOL_SIZE",
		"MAX_WORKFLOW_SESSIONS", "SESSION_TTL_MINUTES", "EXPLORE_SOURCE_TIMEOUT_SECONDS",
	} {
		t.Setenv(key, "")
	}
	cfg := Load()
	if cfg.SearchConcurrency != 16 || cfg.GlobalSearchConcurrency != 32 || cfg.JSPoolSize != 4 {
		t.Fatalf("execution limits=%d/%d/%d", cfg.SearchConcurrency, cfg.GlobalSearchConcurrency, cfg.JSPoolSize)
	}
	if cfg.MaxSessions != 1024 || cfg.SessionTTL != 30*time.Minute || cfg.ExploreSourceTimeout != 30*time.Second {
		t.Fatalf("workflow limits=%d/%s/%s", cfg.MaxSessions, cfg.SessionTTL, cfg.ExploreSourceTimeout)
	}
}

func TestLoadKeepsDefaultDatabaseInsideConfiguredDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "reader-data")
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("DATABASE_PATH", "")

	cfg := Load()
	if cfg.DatabasePath != filepath.Join(dataDir, "novelreader.db") {
		t.Fatalf("database path = %q", cfg.DatabasePath)
	}
}

func TestLoadAcceptsExplicitDatabasePath(t *testing.T) {
	t.Setenv("DATA_DIR", "/srv/novelreader")
	t.Setenv("DATABASE_PATH", "/srv/novelreader/custom.db")

	cfg := Load()
	if cfg.DatabasePath != "/srv/novelreader/custom.db" {
		t.Fatalf("database path = %q", cfg.DatabasePath)
	}
}

func TestLoadInfersDataDirFromDatabasePath(t *testing.T) {
	t.Setenv("DATA_DIR", "")
	t.Setenv("DATABASE_PATH", "/srv/novelreader/custom.db")

	cfg := Load()
	if cfg.DataDir != "/srv/novelreader" || cfg.DatabasePath != "/srv/novelreader/custom.db" {
		t.Fatalf("storage paths = %q / %q", cfg.DataDir, cfg.DatabasePath)
	}
}

func TestLoadPlacesBareDatabasePathInsideDefaultDataDir(t *testing.T) {
	t.Setenv("DATA_DIR", "")
	t.Setenv("DATABASE_PATH", "custom.db")

	cfg := Load()
	if cfg.DataDir != "./data" || cfg.DatabasePath != filepath.Join("data", "custom.db") {
		t.Fatalf("storage paths = %q / %q", cfg.DataDir, cfg.DatabasePath)
	}
}

func TestLoadAcceptsCapacityOverridesAndRejectsNonPositiveValues(t *testing.T) {
	t.Setenv("SEARCH_CONCURRENCY", "24")
	t.Setenv("GLOBAL_SEARCH_CONCURRENCY", "0")
	t.Setenv("JS_POOL_SIZE", "8")
	t.Setenv("MAX_WORKFLOW_SESSIONS", "2048")
	t.Setenv("SESSION_TTL_MINUTES", "60")
	t.Setenv("EXPLORE_SOURCE_TIMEOUT_SECONDS", "45")
	cfg := Load()
	if cfg.SearchConcurrency != 24 || cfg.GlobalSearchConcurrency != 32 || cfg.JSPoolSize != 8 {
		t.Fatalf("execution limits=%d/%d/%d", cfg.SearchConcurrency, cfg.GlobalSearchConcurrency, cfg.JSPoolSize)
	}
	if cfg.MaxSessions != 2048 || cfg.SessionTTL != time.Hour || cfg.ExploreSourceTimeout != 45*time.Second {
		t.Fatalf("workflow limits=%d/%s/%s", cfg.MaxSessions, cfg.SessionTTL, cfg.ExploreSourceTimeout)
	}
}
