package book

import (
	"errors"
	"strings"
	"testing"
)

func TestPaginationErrorsHideOpaqueDataURLs(t *testing.T) {
	const opaqueURL = "data:;base64,eyJib29rX25hbWUiOiLlh6Hkurrkv67ku5nkvKAifQ=="
	for _, err := range []error{
		&TOCPaginationError{Operation: "parse page", FailedURL: opaqueURL, PagesFetched: 1, Err: errors.New("context deadline exceeded")},
		&ContentPaginationError{Operation: "fetch page", FailedURL: opaqueURL, PagesFetched: 1, Err: errors.New("context deadline exceeded")},
	} {
		message := err.Error()
		if strings.Contains(message, "base64") || strings.Contains(message, "eyJ") {
			t.Fatalf("opaque URL leaked in error: %s", message)
		}
		if !strings.Contains(message, "source-provided data") || !strings.Contains(message, "context deadline exceeded") {
			t.Fatalf("error lost useful context: %s", message)
		}
	}
}

func TestPaginationErrorsKeepHTTPURLsVisible(t *testing.T) {
	err := (&TOCPaginationError{Operation: "fetch page", FailedURL: "https://example.com/toc/2", PagesFetched: 1, Err: errors.New("timeout")}).Error()
	if !strings.Contains(err, "https://example.com/toc/2") {
		t.Fatalf("HTTP URL was hidden: %s", err)
	}
}
