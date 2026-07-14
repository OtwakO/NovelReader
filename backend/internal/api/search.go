// Batched SSE search transport; legacy all-source search remains in server.go.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
)

func (s *Server) handleSearchBatchStream(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing query param q")
		return
	}
	batchSize, err := positiveQueryInt(r, "batchSize")
	if err != nil || batchSize > book.MaxSearchBatchSize {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("batchSize must be between 1 and %d", book.MaxSearchBatchSize))
		return
	}
	concurrency := 0
	if _, supplied := r.URL.Query()["concurrency"]; supplied {
		concurrency, err = positiveQueryInt(r, "concurrency")
		if err != nil {
			writeError(w, http.StatusBadRequest, "concurrency must be a positive integer")
			return
		}
	}

	plan, err := s.searcher.PrepareSearchBatch(book.SearchBatchOptions{
		Cursor: r.URL.Query().Get("cursor"), Limit: batchSize, Concurrency: concurrency,
	})
	if errors.Is(err, book.ErrStaleSearchCursor) {
		slog.Info("search batch rejected", "reason", "sources_changed")
		writeSearchSSE(w, map[string]interface{}{
			"type": "stale", "code": "sources_changed", "restartRequired": true,
			"message": "Eligible sources changed; restart the search.",
		})
		return
	}
	if errors.Is(err, book.ErrInvalidSearchBatch) || errors.Is(err, book.ErrInvalidSearchCursor) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare search batch")
		return
	}

	started := time.Now()
	slog.Info("search batch starting",
		"revision", plan.Revision[:8], "offset", plan.Offset,
		"batch_size", plan.SourcesInBatch, "eligible", plan.Eligible,
		"requested_concurrency", plan.RequestedConcurrency,
		"effective_concurrency", plan.EffectiveConcurrency)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	startSSE(w)
	write := func(event map[string]interface{}) {
		payload, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}
	write(map[string]interface{}{
		"type": "start", "revision": plan.Revision, "retryCursor": plan.RetryCursor,
		"offset": plan.Offset, "requestedBatchSize": plan.RequestedBatchSize,
		"sourcesInBatch": plan.SourcesInBatch, "eligible": plan.Eligible,
		"requestedConcurrency": plan.RequestedConcurrency, "effectiveConcurrency": plan.EffectiveConcurrency,
	})

	batchChecked := 0
	batchResults := 0
	sourceFailures := 0
	err = s.searcher.SearchBatch(r.Context(), query, plan, func(source booksource.BookSource, results []book.SearchResult, sourceErr error) {
		batchChecked++
		event := map[string]interface{}{
			"sourceId": source.BookSourceURL, "source": source.BookSourceName,
			"batchChecked": batchChecked, "checked": plan.Offset + batchChecked, "eligible": plan.Eligible,
		}
		if sourceErr != nil {
			sourceFailures++
			event["type"] = "source_error"
			event["message"] = sourceErr.Error()
		} else {
			batchResults += len(results)
			event["type"] = "results"
			event["data"] = results
		}
		write(event)
	})

	complete := err == nil
	done := map[string]interface{}{
		"type": "done", "complete": complete,
		"checked": plan.Offset + batchChecked, "eligible": plan.Eligible,
		"batchResults": batchResults, "sourceFailures": sourceFailures,
		"hasMore": plan.HasMore || !complete, "retryCursor": plan.RetryCursor,
	}
	if complete {
		done["nextCursor"] = plan.NextCursor
	} else if !errors.Is(err, context.Canceled) {
		done["error"] = true
	}
	write(done)
	slog.Info("search batch finished",
		"revision", plan.Revision[:8], "offset", plan.Offset,
		"completed", batchChecked, "failures", sourceFailures,
		"duration", time.Since(started), "cancelled", errors.Is(err, context.Canceled))
}

func positiveQueryInt(r *http.Request, name string) (int, error) {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || value < 1 {
		return 0, errors.New("invalid positive integer")
	}
	return value, nil
}

func startSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}

func writeSearchSSE(w http.ResponseWriter, event map[string]interface{}) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	startSSE(w)
	payload, _ := json.Marshal(event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}
