// NovelReader server — a legado-compatible book source reading platform.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/api"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/config"
	"github.com/otwako/novelreader/internal/database"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fingerprint"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/webview"
)

func main() {
	cfg := config.Load()

	// Enable debug logging via DEBUG=1 env var
	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "1" {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))
	if logLevel == slog.LevelDebug {
		slog.Info("Debug logging enabled (to disable: unset DEBUG)")
	}

	// Open database
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	// Init stores
	sourceStore := booksource.NewStore(db)
	if err := sourceStore.Init(); err != nil {
		log.Fatalf("booksource store: %v", err)
	}

	bookStore := book.NewStore(db)
	if err := bookStore.Init(); err != nil {
		log.Fatalf("book store: %v", err)
	}

	fStore := fontstore.NewStore(db, cfg.DataDir)
	if err := fStore.Init(); err != nil {
		log.Fatalf("font store: %v", err)
	}

	// Engine components
	// ponytail: use insecure TLS for content fetcher too — same reason as search:
	// many Chinese novel sites have self-signed or expired TLS certs. Search already
	// uses InsecureSkipVerify; content/TOC/chapter fetches must match.
	httpContent := fetcher.NewInsecure(15 * time.Second) // normal fallback and content
	fingerprintConfig := fingerprint.Config{
		Timeout:            15 * time.Second,
		Profile:            os.Getenv("TLS_CLIENT_PROFILE"),
		InsecureSkipVerify: true,
	}
	jsFallback := fetcher.NewInsecureStateless(15 * time.Second)
	jsHTTP, err := fingerprint.New(fingerprintConfig, jsFallback)
	if err != nil {
		log.Fatalf("fingerprint transport: %v", err)
	}
	jsVM := analyzer.NewJSVMWithPoolSize(cfg.JSPoolSize)
	jsVM.SetFetcher(jsHTTP)
	cache := analyzer.NewCacheManager()

	limits := book.DefaultSearcherLimits()
	limits.ConcurrentPerSearch = cfg.SearchConcurrency
	limits.ConcurrentGlobal = cfg.GlobalSearchConcurrency
	limits.MaxSessions = cfg.MaxSessions
	limits.SessionTTL = cfg.SessionTTL
	searcher := book.NewSearcherWithLimits(httpContent, jsVM, cache, sourceStore, bookStore, limits)
	regularFingerprintConfig := fingerprintConfig
	regularFingerprintConfig.Timeout = 5 * time.Second // leave room for normal fallback within per-source timeout
	searcher.SetTransportFactory(func(client *fetcher.Client, session *sourceexec.SourceSession) sourceexec.Transport {
		normal := sourceexec.NewHTTPTransportForSession(client, session)
		transport, transportErr := fingerprint.NewTransport(regularFingerprintConfig, normal, session)
		if transportErr != nil {
			log.Printf("fingerprint source transport unavailable: %v", transportErr)
			return normal
		}
		return transport
	})
	if cfg.WebViewEndpoint != "" {
		browserClient, browserErr := webview.NewClient(webview.Config{Endpoint: cfg.WebViewEndpoint})
		if browserErr != nil {
			log.Fatalf("webview transport: %v", browserErr)
		}
		searcher.SetWebViewTransportFactory(browserClient.ForSession)
		slog.Info("headless WebView transport enabled", "endpoint", cfg.WebViewEndpoint)
	}

	// Content processor config
	procCfg := processor.DefaultConfig()

	// Set up time function for API
	api.TimeNowMillis = func() int64 { return time.Now().UnixMilli() }

	// Create API server
	apiSrv := api.NewServer(sourceStore, bookStore, searcher, fStore, httpContent, jsVM, cache, procCfg, cfg.DataDir, db)

	// Serve frontend static files from the project frontend dist
	staticDir := "../frontend/dist"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		staticDir = "./frontend/dist"
	}
	log.Printf("Serving static files from: %s", staticDir)
	fs := http.FileServer(http.Dir(staticDir))
	apiSrv.ServeStatic(apiSrv.Mux(), staticDir, fs)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("NovelReader server starting on %s", addr)
	log.Printf("Database: %s", cfg.DatabasePath)
	slog.Info("capacity limits",
		"searchConcurrency", cfg.SearchConcurrency,
		"globalSearchConcurrency", cfg.GlobalSearchConcurrency,
		"jsPoolSize", cfg.JSPoolSize,
		"maxWorkflowSessions", cfg.MaxSessions,
		"sessionTTL", cfg.SessionTTL,
	)

	srv := &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(cfg.CORSOrigin, apiSrv),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: 0, // no write timeout for long reads
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// corsMiddleware adds CORS headers for frontend access.
func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
