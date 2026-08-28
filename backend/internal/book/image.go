package book

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const maxImageBytes = 10 * 1024 * 1024

var ErrUnsupportedImageDecoder = errors.New("image: Android bitmap decoder is unsupported")

// GetChapterImage fetches and decodes one server-indexed image from a stored chapter.
func (s *Searcher) GetChapterImage(ctx context.Context, src booksource.BookSource, b *Book, chapter *Chapter, imageURL string) ([]byte, string, error) {
	if chapter == nil || strings.TrimSpace(chapter.URL) == "" {
		return nil, "", fmt.Errorf("image: chapter is required")
	}
	script := strings.TrimSpace(parseRuleJSON(src.RuleContent)["imageDecode"])
	if usesAndroidBitmapDecoder(script) {
		return nil, "", ErrUnsupportedImageDecoder
	}
	return s.getStoredImage(ctx, src, b, chapter, imageURL, script, false, "image")
}

func (s *Searcher) getStoredImage(ctx context.Context, src booksource.BookSource, b *Book, chapter *Chapter, rawURL, script string, preserveOnNonBytes bool, label string) ([]byte, string, error) {
	if s == nil || s.fetcher == nil {
		return nil, "", fmt.Errorf("%s: dependencies unavailable", label)
	}
	if b == nil || strings.TrimSpace(rawURL) == "" {
		return nil, "", fmt.Errorf("%s: URL is empty", label)
	}
	ctx, cancel := context.WithTimeout(ctx, s.sourceTimeout())
	defer cancel()

	session := s.sessions.GetOrCreateBook(src.ID, b.BookURL)
	headers, err := evaluateSourceHeaders(ctx, s.jsVM, src, session)
	if err != nil {
		return nil, "", fmt.Errorf("%s: source headers: %w", label, err)
	}
	session.SetRequestHeaders(headers)
	executor := sourceexec.NewExecutorWithSession(s.jsVM, nil, session)
	contextURL := rawURL
	resolutionBaseURL := src.BookSourceURL
	if chapter != nil {
		contextURL = chapter.URL
		resolutionBaseURL = chapter.URL
	}
	setExecutorContext(executor, src, b, chapter, nil, contextURL)
	spec, err := executor.BuildContext(ctx, rawURL, "", 1, resolutionBaseURL)
	if err != nil {
		return nil, "", fmt.Errorf("%s: URL: %w", label, err)
	}
	if spec.Method != "" && spec.Method != http.MethodGet {
		return nil, "", fmt.Errorf("%s: unsupported method %s", label, spec.Method)
	}
	if spec.WebView {
		return nil, "", fmt.Errorf("%s: WebView requests are unsupported", label)
	}
	spec.Headers = sourceexec.MergeHeaders(headers, spec.Headers)
	client := fetcher.NewSessionHTTPClient(s.fetcher, session)
	response, err := client.GetBytesContext(ctx, spec.URL, spec.Headers, maxImageBytes)
	if err != nil {
		return nil, "", fmt.Errorf("%s: fetch: %w", label, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("%s: upstream status %d", label, response.StatusCode)
	}
	data := response.RawBody
	if len(data) == 0 {
		data = []byte(response.Body)
	}
	if script != "" {
		if s.jsVM == nil {
			return nil, "", fmt.Errorf("%s: JavaScript engine unavailable", label)
		}
		bindings := map[string]interface{}{
			"sourceState": session,
			"source":      sourceContext(src),
			"book":        bookContext(b, src),
		}
		if chapter != nil {
			bindings["chapter"] = chapterContext(b, chapter, chapter.URL)
			bindings["src"] = spec.URL
		}
		value, evalErr := s.jsVM.EvalContext(ctx, decodeScript(src.JSLib, script), data, spec.URL, bindings)
		if evalErr != nil {
			return nil, "", fmt.Errorf("%s: decode: %w", label, evalErr)
		}
		decoded, decodeErr := analyzer.ToBytes(value)
		if decodeErr != nil {
			if !preserveOnNonBytes {
				return nil, "", fmt.Errorf("%s: decode did not return bytes: %w", label, decodeErr)
			}
		} else {
			data = decoded
		}
		if len(data) > maxImageBytes {
			return nil, "", fmt.Errorf("%s: decoded response exceeds %d bytes", label, maxImageBytes)
		}
	}

	contentType := http.DetectContentType(data)
	if headerType := response.Headers.Get("Content-Type"); script == "" && headerType != "" {
		contentType = strings.TrimSpace(strings.Split(headerType, ";")[0])
	}
	return data, contentType, nil
}

func decodeScript(jsLib, script string) string {
	trimmed := strings.TrimSpace(jsLib)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return script
	}
	if trimmed == "" {
		return script
	}
	return jsLib + "\n" + script
}

func usesAndroidBitmapDecoder(script string) bool {
	lower := strings.ToLower(script)
	return strings.Contains(lower, "android.graphics") || strings.Contains(lower, "bitmapfactory") || strings.Contains(lower, "bitmap.createbitmap")
}
