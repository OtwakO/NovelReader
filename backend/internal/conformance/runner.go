// Raw-source conformance runner for request expansion, transport, and search parsing.
package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fingerprint"
	"github.com/otwako/novelreader/internal/sourceexec"
)

const bodySampleLimit = 1000

// SourceIdentity prevents duplicate source names from selecting the wrong raw entry.
type SourceIdentity struct {
	Index  int    `json:"index"`
	URL    string `json:"bookSourceUrl"`
	SHA256 string `json:"sha256"`
}

// RequestRecord stores the exact expanded request, with sensitive header values redacted.
type RequestRecord struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Charset string            `json:"charset,omitempty"`
	Retry   int               `json:"retry,omitempty"`
	WebView bool              `json:"webView,omitempty"`
}

// ResponseRecord stores transport output without retaining an unbounded body.
type ResponseRecord struct {
	StatusCode int    `json:"statusCode"`
	FinalURL   string `json:"finalUrl,omitempty"`
	Transport  string `json:"transport,omitempty"`
	BodySample string `json:"bodySample,omitempty"`
}

// Record is one independently reproducible raw-source search observation.
type Record struct {
	Identity       SourceIdentity      `json:"identity"`
	SourceName     string              `json:"sourceName"`
	RawSource      json.RawMessage     `json:"rawSource"`
	RuleField      string              `json:"ruleField"`
	Request        *RequestRecord      `json:"request,omitempty"`
	Response       *ResponseRecord     `json:"response,omitempty"`
	Extracted      []book.SearchResult `json:"extracted,omitempty"`
	Classification string              `json:"classification"`
	Error          string              `json:"error,omitempty"`
}

// Options controls whether the runner matches production fingerprint transport.
type Options struct {
	Timeout     time.Duration
	Fingerprint bool
}

// RunSearch executes selected raw sources and records request, response, and parser output.
func RunSearch(ctx context.Context, raw []byte, indices []int, query string, timeout time.Duration) ([]Record, error) {
	return RunSearchWithOptions(ctx, raw, indices, query, Options{Timeout: timeout})
}

// RunSearchWithOptions executes selected raw sources with explicit transport policy.
func RunSearchWithOptions(ctx context.Context, raw []byte, indices []int, query string, options Options) ([]Record, error) {
	items, err := rawItems(raw)
	if err != nil {
		return nil, err
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	selected, err := selectIndices(len(items), indices)
	if err != nil {
		return nil, err
	}

	jsVM := analyzer.NewJSVM()
	var jsHTTP fetcher.HTTPClient = fetcher.NewInsecureStateless(timeout)
	if options.Fingerprint {
		fingerprintClient, fingerprintErr := fingerprint.New(fingerprint.Config{
			Timeout:            minDuration(timeout, 5*time.Second),
			InsecureSkipVerify: true,
		}, jsHTTP)
		if fingerprintErr == nil {
			jsHTTP = fingerprintClient
		}
	}
	jsVM.SetFetcher(jsHTTP)
	parser := book.NewSearcher(fetcher.NewInsecure(timeout), jsVM, nil, nil, nil)
	records := make([]Record, 0, len(selected))
	for _, index := range selected {
		record := Record{
			Identity:  SourceIdentity{Index: index, URL: sourceURL(items[index])},
			RawSource: append(json.RawMessage(nil), items[index]...),
			RuleField: "ruleSearch",
		}
		hash := sha256.Sum256(items[index])
		record.Identity.SHA256 = hex.EncodeToString(hash[:])

		var src booksource.BookSource
		if err := json.Unmarshal(items[index], &src); err != nil {
			record.Classification = "source_parse_failure"
			record.Error = err.Error()
			records = append(records, record)
			continue
		}
		record.SourceName = src.BookSourceName
		if src.BookSourceType != 0 {
			record.Classification = "unsupported_webview"
			record.Error = "source type requires browser-backed execution"
			records = append(records, record)
			continue
		}

		sourceCtx, cancel := context.WithTimeout(ctx, timeout)
		session := sourceexec.NewSourceSession()
		normalTransport := sourceexec.NewHTTPTransportForSession(fetcher.NewInsecure(timeout), session)
		var transport sourceexec.Transport = normalTransport
		if options.Fingerprint {
			if fingerprintTransport, fingerprintErr := fingerprint.NewTransport(fingerprint.Config{
				Timeout:            minDuration(timeout, 5*time.Second),
				InsecureSkipVerify: true,
			}, normalTransport, session); fingerprintErr == nil {
				transport = fingerprintTransport
			}
		}
		executor := sourceexec.NewExecutorWithSession(jsVM, transport, session)
		spec, buildErr := executor.BuildContext(sourceCtx, src.SearchURL, query, 1, src.BookSourceURL)
		if buildErr != nil {
			record.Classification = "js_or_request_build_failure"
			record.Error = buildErr.Error()
			cancel()
			records = append(records, record)
			continue
		}
		mergeHeaders(spec.Headers, parseHeaders(src.Header))
		record.Request = &RequestRecord{
			URL: spec.URL, Method: spec.Method, Body: spec.Body,
			Headers: redactHeaders(spec.Headers), Charset: spec.Charset,
			Retry: spec.Retry, WebView: spec.WebView,
		}
		if spec.WebView {
			record.Classification = "unsupported_webview"
			cancel()
			records = append(records, record)
			continue
		}
		response, requestErr := transport.Do(sourceCtx, spec)
		cancel()
		if requestErr != nil {
			record.Classification = classifyTransportError(requestErr)
			record.Error = requestErr.Error()
			records = append(records, record)
			continue
		}
		record.Response = &ResponseRecord{
			StatusCode: response.StatusCode,
			FinalURL:   response.FinalURL,
			Transport:  response.Transport,
			BodySample: sample(response.Body),
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			record.Classification = "http_or_waf_failure"
			records = append(records, record)
			continue
		}
		results, parseErr := parser.ParseSearchResultWithState(src, response.Body, session)
		if parseErr != nil {
			record.Classification = "rule_mismatch"
			record.Error = parseErr.Error()
		} else if len(results) == 0 {
			record.Classification = "legitimate_zero_results"
		} else {
			record.Classification = "success"
			record.Extracted = results
		}
		records = append(records, record)
	}
	return records, nil
}

func rawItems(raw []byte) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
		return items, nil
	}
	var single json.RawMessage
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, fmt.Errorf("conformance: invalid source JSON: %w", err)
	}
	return []json.RawMessage{single}, nil
}

func selectIndices(total int, indices []int) ([]int, error) {
	if len(indices) == 0 {
		indices = make([]int, total)
		for i := range indices {
			indices[i] = i
		}
	}
	for _, index := range indices {
		if index < 0 || index >= total {
			return nil, fmt.Errorf("conformance: source index %d outside [0,%d)", index, total)
		}
	}
	return indices, nil
}

func sourceURL(raw json.RawMessage) string {
	var source struct {
		URL string `json:"bookSourceUrl"`
	}
	_ = json.Unmarshal(raw, &source)
	return source.URL
}

func parseHeaders(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var headers map[string]string
	if json.Unmarshal([]byte(raw), &headers) == nil {
		return headers
	}
	var encoded string
	if json.Unmarshal([]byte(raw), &encoded) == nil {
		_ = json.Unmarshal([]byte(encoded), &headers)
	}
	return headers
}

func mergeHeaders(target, source map[string]string) {
	for key, value := range source {
		if _, exists := target[key]; !exists {
			target[key] = value
		}
	}
}

func redactHeaders(headers map[string]string) map[string]string {
	redacted := make(map[string]string, len(headers))
	for key, value := range headers {
		if strings.EqualFold(key, "cookie") || strings.EqualFold(key, "authorization") {
			redacted[key] = "[redacted]"
		} else {
			redacted[key] = value
		}
	}
	return redacted
}

func sample(body string) string {
	if len(body) <= bodySampleLimit {
		return body
	}
	return body[:bodySampleLimit]
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func classifyTransportError(err error) string {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "context deadline") || strings.Contains(message, "timeout") {
		return "transport_timeout"
	}
	if strings.Contains(message, "no such host") || strings.Contains(message, "dns") {
		return "transport_dns_failure"
	}
	return "transport_failure"
}
