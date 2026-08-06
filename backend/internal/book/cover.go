package book

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const maxCoverBytes = 10 * 1024 * 1024

// GetBookCover fetches and optionally decodes a stored book's cover image.
func (s *Searcher) GetBookCover(ctx context.Context, src booksource.BookSource, b *Book) ([]byte, string, error) {
	if s == nil || s.fetcher == nil {
		return nil, "", fmt.Errorf("cover: dependencies unavailable")
	}
	if b == nil || strings.TrimSpace(b.CoverURL) == "" {
		return nil, "", fmt.Errorf("cover: URL is empty")
	}
	ctx, cancel := context.WithTimeout(ctx, s.sourceTimeout())
	defer cancel()

	session := s.sessions.GetOrCreateBook(src.BookSourceURL, b.BookURL)
	headers, err := evaluateSourceHeaders(ctx, s.jsVM, src, session)
	if err != nil {
		return nil, "", fmt.Errorf("cover: source headers: %w", err)
	}
	session.SetRequestHeaders(headers)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, nil, session)
	setExecutorContext(executor, src, b, nil, nil, b.CoverURL)
	spec, err := executor.BuildContext(ctx, b.CoverURL, "", 1, src.BookSourceURL)
	if err != nil {
		return nil, "", fmt.Errorf("cover: URL: %w", err)
	}
	if spec.Method != "" && spec.Method != http.MethodGet {
		return nil, "", fmt.Errorf("cover: unsupported method %s", spec.Method)
	}
	if spec.WebView {
		return nil, "", fmt.Errorf("cover: WebView requests are unsupported")
	}
	spec.Headers = sourceexec.MergeHeaders(headers, spec.Headers)
	client := fetcher.NewSessionHTTPClient(s.fetcher, session)
	response, err := client.GetBytesContext(ctx, spec.URL, spec.Headers, maxCoverBytes)
	if err != nil {
		return nil, "", fmt.Errorf("cover: fetch: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("cover: upstream status %d", response.StatusCode)
	}
	data := response.RawBody
	if len(data) == 0 {
		data = []byte(response.Body)
	}
	if script := strings.TrimSpace(src.CoverDecodeJS); script != "" {
		if s.jsVM == nil {
			return nil, "", fmt.Errorf("cover: JavaScript engine unavailable")
		}
		bindings := map[string]interface{}{
			"sourceState": session,
			"source":      sourceContext(src),
			"book":        bookContext(b, src),
		}
		value, evalErr := s.jsVM.EvalContext(ctx, src.JSLib+"\n"+script, data, spec.URL, bindings)
		if evalErr != nil {
			return nil, "", fmt.Errorf("cover: decode: %w", evalErr)
		}
		if decoded, decodeErr := analyzer.ToBytes(value); decodeErr == nil {
			data = decoded
		}
		if len(data) > maxCoverBytes {
			return nil, "", fmt.Errorf("cover: decoded response exceeds %d bytes", maxCoverBytes)
		}
	}

	contentType := http.DetectContentType(data)
	if headerType := response.Headers.Get("Content-Type"); strings.TrimSpace(src.CoverDecodeJS) == "" && headerType != "" {
		contentType = strings.TrimSpace(strings.Split(headerType, ";")[0])
	}
	return data, contentType, nil
}
