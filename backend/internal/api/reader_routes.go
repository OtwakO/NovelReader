package api

func (s *readerAPI) registerRoutes() {
	// Book sources — URLs with slashes can't go in path segments, use query param
	s.mux.HandleFunc("GET /api/sources", s.handleListSources)
	s.mux.HandleFunc("POST /api/sources", s.handleImportSources)
	s.mux.HandleFunc("GET /api/sources/{id}", s.handleGetSource)
	s.mux.HandleFunc("PATCH /api/sources/{id}", s.handleUpdateSourcePreferences)
	s.mux.HandleFunc("GET /api/source-collections", s.handleListSourceCollections)
	s.mux.HandleFunc("POST /api/source-collections/upload", s.handleCreateUploadCollection)
	s.mux.HandleFunc("POST /api/source-collections/url", s.handleCreateURLCollection)
	s.mux.HandleFunc("PATCH /api/source-collections/{id}", s.handleUpdateSourceCollection)
	s.mux.HandleFunc("POST /api/source-collections/{id}/replace", s.handleReplaceUploadCollection)
	s.mux.HandleFunc("POST /api/source-collections/{id}/sync", s.handleSyncSourceCollection)
	s.mux.HandleFunc("DELETE /api/source-collections/{id}", s.handleDeleteSourceCollection)
	s.mux.HandleFunc("DELETE /api/sources", s.handleDeleteSource)
	s.mux.HandleFunc("PUT /api/sources", s.handleUpdateSource)
	s.mux.HandleFunc("GET /api/sources/{id}/interaction", s.handleSourceInteraction)
	s.mux.HandleFunc("POST /api/sources/{id}/interaction/actions", s.handleSourceInteractionAction)
	s.mux.HandleFunc("GET /api/sources/{id}/interaction/runtime-cookies", s.handleRuntimeCookieMetadata)
	s.mux.HandleFunc("POST /api/sources/{id}/interaction/runtime-cookies/reveal", s.handleRevealRuntimeCookies)
	s.mux.HandleFunc("PUT /api/sources/{id}/interaction/runtime-cookies", s.handleReplaceRuntimeCookies)
	s.mux.HandleFunc("POST /api/sources/{id}/interaction/browser", s.handleStartSourceBrowser)
	s.mux.HandleFunc("GET /api/sources/{id}/interaction/browser/{sessionID}", s.handleSourceBrowserFrame)
	s.mux.HandleFunc("POST /api/sources/{id}/interaction/browser/{sessionID}/input", s.handleSourceBrowserInput)
	s.mux.HandleFunc("DELETE /api/sources/{id}/interaction/browser/{sessionID}", s.handleCloseSourceBrowser)
	s.mux.HandleFunc("DELETE /api/sources/{id}/interaction/login", s.handleSourceInteractionResetLogin)
	s.mux.HandleFunc("DELETE /api/sources/{id}/interaction/settings", s.handleSourceInteractionResetSettings)
	s.mux.HandleFunc("DELETE /api/sources/{id}/interaction", s.handleSourceInteractionResetAll)

	// Books
	s.mux.HandleFunc("GET /api/books", s.handleListBooks)
	s.mux.HandleFunc("GET /api/books/{id}", s.handleGetBook)
	s.mux.HandleFunc("GET /api/books/{id}/cover", s.handleGetBookCover)
	s.mux.HandleFunc("GET /api/covers/{reference}", s.handleGetCoverDisplay)
	s.mux.HandleFunc("POST /api/candidate-resolutions", s.handleStartCandidateResolution)
	s.mux.HandleFunc("GET /api/candidate-resolutions/{id}", s.handleGetCandidateResolution)
	s.mux.HandleFunc("GET /api/candidate-resolutions/{id}/events", s.handleStreamCandidateResolution)
	s.mux.HandleFunc("DELETE /api/candidate-resolutions/{id}", s.handleCancelCandidateResolution)
	s.mux.HandleFunc("POST /api/candidate-resolutions/{id}/shelve", s.handleCommitCandidateResolution)
	s.mux.HandleFunc("POST /api/books/{id}/sources", s.handleMergeBookSources)
	s.mux.HandleFunc("DELETE /api/books/{id}/sources", s.handleClearBookSources)
	s.mux.HandleFunc("DELETE /api/books", s.handleDeleteBook)

	// Search
	s.mux.HandleFunc("GET /api/search/stream", s.handleSearchBatchStream)
	s.mux.HandleFunc("POST /api/search/source", s.handleSearchInstalledSource)

	// Explore
	s.mux.HandleFunc("GET /api/explore/sources", s.handleExploreSources)
	s.mux.HandleFunc("POST /api/explore/catalog", s.handleExploreCatalog)
	s.mux.HandleFunc("POST /api/explore/control", s.handleExploreControl)
	s.mux.HandleFunc("POST /api/explore/page", s.handleExplorePage)
	s.mux.HandleFunc("POST /api/explore/action", s.handleExploreAction)

	// Chapters
	s.mux.HandleFunc("GET /api/books/{id}/chapters", s.handleGetChapters)
	s.mux.HandleFunc("POST /api/books/{id}/chapters/sync", s.handleRetryChapters)
	s.mux.HandleFunc("GET /api/books/{id}/chapters/{idx}/content", s.handleGetChapterContent)
	s.mux.HandleFunc("GET /api/books/{id}/chapters/{idx}/images/{imageIdx}", s.handleGetChapterImage)

	// Progress
	s.mux.HandleFunc("PUT /api/books/{id}/progress", s.handleUpdateProgress)

	// Source switching and bookmarks
	s.mux.HandleFunc("PUT /api/books/{id}/source", s.handleSwitchSource)
	s.mux.HandleFunc("GET /api/books/{id}/bookmarks", s.handleListBookmarks)
	s.mux.HandleFunc("POST /api/books/{id}/bookmarks", s.handleAddBookmark)
	s.mux.HandleFunc("DELETE /api/books/{id}/bookmarks/{bookmarkID}", s.handleDeleteBookmark)

	// Fonts — IDs are simple UUIDs/timestamps, path-safe
	s.mux.HandleFunc("GET /api/fonts", s.handleListFonts)
	s.mux.HandleFunc("POST /api/fonts", s.handleUploadFont)
	s.mux.HandleFunc("DELETE /api/fonts/{id}", s.handleDeleteFont)
	s.mux.HandleFunc("GET /api/fonts/{id}/file", s.handleGetFontFile)
	s.mux.HandleFunc("GET /api/system/webview-status", s.handleWebViewStatus)
	s.mux.HandleFunc("GET /api/system/chinese-conversion", s.handleChineseConversionCapability)
	s.mux.HandleFunc("POST /api/system/chinese-conversion", s.handleChineseConversion)
}
