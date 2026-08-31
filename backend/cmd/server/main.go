// NovelReader server — a legado-compatible book source reading platform.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/api"
	"github.com/otwako/novelreader/internal/auth"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/chineseconv"
	"github.com/otwako/novelreader/internal/config"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fingerprint"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceprofile"
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

	// Classify the complete data root and validate identity storage before any reader store opens.
	systemStore, readers, err := openStores(cfg.DataDir)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer systemStore.Close()
	defer readers.Close()

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
	limits.ExploreSourceTimeout = cfg.ExploreSourceTimeout
	searcher := book.NewSearcherWithLimits(httpContent, jsVM, cache, nil, nil, limits)
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
	var browserClient *webview.Client
	if cfg.WebViewEndpoint != "" {
		var browserErr error
		browserClient, browserErr = webview.NewClient(webview.Config{Endpoint: cfg.WebViewEndpoint})
		if browserErr != nil {
			log.Fatalf("webview transport: %v", browserErr)
		}
		searcher.SetWebViewTransportFactory(browserClient.ForSession)
		slog.Info("headless WebView transport enabled", "endpoint", cfg.WebViewEndpoint)
	}

	// Content processor config
	procCfg := processor.DefaultConfig()

	// Display-only Chinese conversion is optional in ordinary local builds and required in release images.
	conversion, err := chineseconv.New()
	if err != nil {
		log.Fatalf("Chinese conversion: %v", err)
	}
	capability := conversion.Capability()
	if capability.Available {
		slog.Info("Chinese conversion enabled", "engine", capability.Engine, "version", capability.Version, "presets", capability.Presets)
	} else {
		slog.Info("Chinese conversion unavailable in this build")
	}

	// Set up time function for API
	api.TimeNowMillis = func() int64 { return time.Now().UnixMilli() }

	// Mount public authentication/setup/recovery and protect every Reader Data route.
	authHandler, err := auth.NewHTTPHandler(systemStore, auth.HTTPConfig{
		PublicURL: cfg.PublicURL, Readers: readers,
		RegistrationEnabled: cfg.RegistrationEnabled, RegistrationInviteCode: cfg.RegistrationInviteCode,
	})
	if err != nil {
		log.Fatalf("authentication HTTP: %v", err)
	}
	setupHandler, err := auth.NewSetupHTTPHandler(systemStore, readers, auth.SetupHTTPConfig{PublicURL: cfg.PublicURL, BootstrapToken: cfg.AdminBootstrapToken})
	if err != nil {
		log.Fatalf("setup HTTP: %v", err)
	}
	recoveryHandler, err := auth.NewRecoveryHTTPHandler(systemStore, readers, auth.RecoveryHTTPConfig{PublicURL: cfg.PublicURL, RecoveryToken: cfg.AdminRecoveryToken})
	if err != nil {
		log.Fatalf("recovery HTTP: %v", err)
	}
	apiSrv, err := api.NewAuthenticatedServer(authHandler, readers, cfg.DataDir, searcher, jsVM, limits, procCfg, systemStore, browserClient, browserClient, conversion)
	if err != nil {
		log.Fatalf("authenticated API: %v", err)
	}
	defer apiSrv.Close()
	rootMux := applicationMux(apiSrv, authHandler, setupHandler, recoveryHandler, cfg.AdminRecoveryToken != "")

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
	log.Printf("Data root: %s", cfg.DataDir)
	slog.Info("capacity limits",
		"searchConcurrency", cfg.SearchConcurrency,
		"globalSearchConcurrency", cfg.GlobalSearchConcurrency,
		"jsPoolSize", cfg.JSPoolSize,
		"maxWorkflowSessions", cfg.MaxSessions,
		"sessionTTL", cfg.SessionTTL,
		"exploreSourceTimeout", cfg.ExploreSourceTimeout,
	)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mustCredentialOriginMiddleware(cfg.PublicURL, rootMux),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: 0, // no write timeout for long reads
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("server: listen: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, srv, listener); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func applicationMux(apiHandler, authHandler, setupHandler, recoveryHandler http.Handler, recoveryEnabled bool) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/api/auth/", authHandler)
	mux.Handle("/api/setup", setupHandler)
	mux.Handle("/api/setup/", setupHandler)
	if recoveryEnabled {
		mux.Handle("/api/recovery", recoveryHandler)
		mux.Handle("/api/recovery/", recoveryHandler)
	} else {
		mux.HandleFunc("/api/recovery", http.NotFound)
		mux.HandleFunc("/api/recovery/", http.NotFound)
	}
	mux.Handle("/", apiHandler)
	return mux
}

func openStores(dataDir string) (*auth.Store, *readerstore.Manager, error) {
	if _, err := readerstore.PrepareRoot(dataDir); err != nil {
		return nil, nil, fmt.Errorf("data root: %w", err)
	}
	rootLock, err := readerstore.LockRoot(dataDir)
	if err != nil {
		return nil, nil, fmt.Errorf("data root: %w", err)
	}
	systemStore, err := auth.OpenSystemStore(dataDir)
	if err != nil {
		_ = rootLock.Close()
		return nil, nil, fmt.Errorf("system database: %w", err)
	}
	systemStore.HoldRootLock(rootLock)
	readers, err := readerstore.NewManager(dataDir, 32,
		booksource.ReaderSchema(), book.ReaderSchema(), fontstore.ReaderSchema(), sourceprofile.ReaderSchema())
	if err != nil {
		_ = systemStore.Close()
		return nil, nil, fmt.Errorf("reader stores: %w", err)
	}
	return systemStore, readers, nil
}

func serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func mustCredentialOriginMiddleware(publicURL string, next http.Handler) http.Handler {
	handler, err := credentialOriginMiddleware(publicURL, next)
	if err != nil {
		log.Fatalf("origin policy: %v", err)
	}
	return handler
}

func credentialOriginMiddleware(publicURL string, next http.Handler) (http.Handler, error) {
	origin, _, err := auth.ParsePublicOrigin(publicURL)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matchedOrigin, _, matches := auth.MatchRequestOrigin(origin, r)
		if matches {
			w.Header().Set("Access-Control-Allow-Origin", matchedOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		backupAutomation := strings.HasPrefix(r.URL.Path, "/api/backups/") && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
		if isUnsafeMethod(r.Method) && !matches && !backupAutomation {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodOptions {
			if !matches {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

func isUnsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}
