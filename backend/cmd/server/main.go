// NovelReader server — a legado-compatible book source reading platform.
package main

import (
	"fmt"
	"log"
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
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
)

func main() {
	cfg := config.Load()

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
	httpClient := fetcher.New()
	jsVM := analyzer.NewJSVM()
	cache := analyzer.NewCacheManager()

	searcher := book.NewSearcher(httpClient, jsVM, cache, sourceStore, bookStore)

	// Content processor config
	procCfg := processor.DefaultConfig()

	// Set up time function for API
	api.TimeNowMillis = func() int64 { return time.Now().UnixMilli() }

	// Create API server
	apiSrv := api.NewServer(sourceStore, bookStore, searcher, fStore, httpClient, jsVM, cache, procCfg, cfg.DataDir)

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
