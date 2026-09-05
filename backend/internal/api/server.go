// Package api provides the HTTP REST API for NovelReader.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/auth"
	backupservice "github.com/otwako/novelreader/internal/backup"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/candidate"
	"github.com/otwako/novelreader/internal/chineseconv"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/processor"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceinteraction"
)

// Server owns process services and the authentication/backup boundary.
// Reader endpoints live in runtime-owned readerAPI handlers.
type Server struct {
	mux                 *http.ServeMux
	auth                *auth.HTTPHandler
	runtimes            *readerRuntimeManager
	services            *readerServices
	standalone          *readerAPI
	health              interface{ PingContext(context.Context) error }
	collectionScheduler *sourceCollectionScheduler
	backups             *backupservice.Service
}

func (s *Server) Mux() *http.ServeMux { return s.mux }

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	if s.services != nil && s.services.candidateOperations != nil {
		s.services.candidateOperations.Close()
	}
	if s.standalone != nil && s.standalone.catalogs != nil {
		s.standalone.catalogs.Close()
	}
	if s.backups != nil {
		if err := s.backups.Close(); err != nil {
			return err
		}
	}
	if s.collectionScheduler != nil {
		s.collectionScheduler.Close()
	}
	if s.services != nil && s.services.chineseConversion != nil {
		if err := s.services.chineseConversion.Close(); err != nil {
			return err
		}
	}
	if s.runtimes == nil {
		return nil
	}
	return s.runtimes.Close()
}

// NewServer binds one standalone reader. The signature is retained for existing
// callers; source execution state is owned by the supplied Searcher.
func NewServer(sourceStore *booksource.Store, bookStore *book.Store, searcher *book.Searcher, fontStore *fontstore.Store, fetcher *fetcher.Client, jsVM *analyzer.JSVM, cache *analyzer.CacheManager, processorCfg processor.Config, dataDir string, db *sql.DB) *Server {
	services := &readerServices{fetcher: fetcher, processorCfg: processorCfg,
		candidateOperations: candidate.NewManager(candidate.DefaultPolicy()),
		coverReferenceKey:   mustNewCoverReferenceKey(), collectionLoader: booksource.NewRemoteLoader()}
	runtime := &readerRuntime{db: db, sourceStore: sourceStore, bookStore: bookStore, searcher: searcher, fontStore: fontStore}
	if bookStore != nil && sourceStore != nil && searcher != nil {
		runtime.catalogs = book.NewCatalogs(bookStore, sourceStore, searcher)
	}
	runtime.api = newReaderAPI(runtime, services)
	s := &Server{mux: runtime.api.mux, services: services, standalone: runtime.api}
	if db != nil {
		s.health = db
	}
	s.mux.HandleFunc("GET /api/healthz", s.handleHealth)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("server: panic recovered", "path", r.URL.Path, "method", r.Method, "panic", fmt.Sprintf("%v", rec))
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
	}()
	s.mux.ServeHTTP(w, r)
}

// NewAuthenticatedServer creates the production Reader Data boundary.
func NewAuthenticatedServer(authHandler *auth.HTTPHandler, readers *readerstore.Manager, dataRoot string, rootSearcher *book.Searcher, jsVM *analyzer.JSVM, limits book.SearcherLimits, processorCfg processor.Config, health interface{ PingContext(context.Context) error }, browser sourceinteraction.Browser, webViewProbe interface{ Probe(context.Context) error }, conversion chineseconv.Service) (*Server, error) {
	services := &readerServices{fetcher: rootSearcher.SharedFetcher(), processorCfg: processorCfg, auth: authHandler,
		webViewProbe: webViewProbe, chineseConversion: conversion,
		candidateOperations: candidate.NewManager(candidate.DefaultPolicy()),
		coverReferenceKey:   mustNewCoverReferenceKey(), collectionLoader: booksource.NewRemoteLoader()}
	s := &Server{mux: http.NewServeMux(), auth: authHandler, health: health, services: services}
	s.runtimes = newReaderRuntimeManager(readers, rootSearcher, jsVM, browser, limits, 32, limits.SessionTTL, services)
	services.runtimes = s.runtimes
	backups, err := backupservice.NewService(readers, dataRoot, s.runtimes.quiesce, s.runtimes.resume)
	if err != nil {
		_ = s.runtimes.Close()
		return nil, fmt.Errorf("initialize backup service: %w", err)
	}
	s.backups = backups
	s.collectionScheduler = newSourceCollectionScheduler(s.runtimes, services.collectionLoader, authHandler.ListActiveReaderIDs)
	s.collectionScheduler.Start()
	authHandler.ConfigureDeletionQuiescer(readers, s.runtimes.quiesce)
	s.registerAuthenticatedRoutes()
	return s, nil
}

func (s *Server) registerAuthenticatedRoutes() {
	s.mux.HandleFunc("GET /api/healthz", s.handleHealth)
	s.mux.Handle("GET /api/backups/export", s.auth.RequireBackupScope(auth.BackupExport, http.HandlerFunc(s.handleBackupExport)))
	s.mux.Handle("POST /api/backups/restores", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handlePrepareBackupRestore)))
	s.mux.Handle("GET /api/backups/restores/{id}", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handleGetBackupRestore)))
	s.mux.Handle("DELETE /api/backups/restores/{id}", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handleCancelBackupRestore)))
	s.mux.Handle("POST /api/backups/restores/{id}/commit", s.auth.RequireBackupScope(auth.BackupRestore, http.HandlerFunc(s.handleCommitBackupRestore)))
	s.mux.Handle("/api/", s.auth.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		runtime, release, err := s.runtimes.acquire(r.Context(), account.ID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "reader storage unavailable")
			return
		}
		defer release()
		runtime.api.ServeHTTP(w, r)
	})))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
