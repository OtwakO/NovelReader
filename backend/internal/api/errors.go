// API error responses classify client, storage, and upstream crawl failures.
package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/otwako/novelreader/internal/book"
)

type crawlErrorResponse struct {
	Error           string `json:"error"`
	Code            string `json:"code"`
	Workflow        string `json:"workflow"`
	Operation       string `json:"operation,omitempty"`
	PageURL         string `json:"pageUrl,omitempty"`
	FailedURL       string `json:"failedUrl,omitempty"`
	PagesFetched    int    `json:"pagesFetched,omitempty"`
	ChaptersFetched int    `json:"chaptersFetched,omitempty"`
}

func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

func writeCrawlError(w http.ResponseWriter, workflow string, err error) {
	response := crawlErrorResponse{
		Error:    err.Error(),
		Code:     workflow + "_failed",
		Workflow: workflow,
	}
	var tocErr *book.TOCPaginationError
	var contentErr *book.ContentPaginationError
	switch {
	case errors.As(err, &tocErr):
		response.Code = "toc_pagination_failed"
		response.Operation = tocErr.Operation
		response.PageURL = tocErr.PageURL
		response.FailedURL = tocErr.FailedURL
		response.PagesFetched = tocErr.PagesFetched
		response.ChaptersFetched = tocErr.ChaptersFetched
	case errors.As(err, &contentErr):
		response.Code = "content_pagination_failed"
		response.Operation = contentErr.Operation
		response.PageURL = contentErr.PageURL
		response.FailedURL = contentErr.FailedURL
		response.PagesFetched = contentErr.PagesFetched
	}
	slog.Warn("api: upstream crawl failed", "workflow", workflow, "code", response.Code, "error", err)
	writeJSON(w, http.StatusBadGateway, response)
}
