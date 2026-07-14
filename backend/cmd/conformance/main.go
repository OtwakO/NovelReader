// CLI for reproducible raw booksource request and search-rule verification.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/conformance"
)

func main() {
	input := flag.String("sources", "", "raw booksource JSON file")
	indicesText := flag.String("indices", "", "comma-separated raw JSON indices; empty means all")
	query := flag.String("query", "凡人修仙传", "search query")
	timeout := flag.Duration("timeout", 10*time.Second, "per-source timeout")
	healthURL := flag.String("health-url", "", "optional server health endpoint; abort on failure")
	webViewEndpoint := flag.String("webview-endpoint", "", "optional Patchright worker endpoint for webView requests")
	bookURL := flag.String("book-url", "", "optional book URL; runs detail, TOC, and first-chapter workflow for one index")
	flag.Parse()
	if *input == "" {
		flag.Usage()
		os.Exit(2)
	}

	raw, err := os.ReadFile(*input)
	if err != nil {
		fatal(err)
	}
	indices, err := parseIndices(*indicesText)
	if err != nil {
		fatal(err)
	}
	options := conformance.Options{
		Timeout: *timeout, Fingerprint: true, HealthURL: *healthURL, WebViewEndpoint: *webViewEndpoint,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if *bookURL != "" {
		if len(indices) != 1 {
			fatal(fmt.Errorf("-book-url requires exactly one index in -indices"))
		}
		record, err := conformance.RunWorkflowWithOptions(context.Background(), raw, indices[0], *bookURL, options)
		if err != nil {
			fatal(err)
		}
		if err := encoder.Encode(record); err != nil {
			fatal(err)
		}
		return
	}
	records, err := conformance.RunSearchWithOptions(context.Background(), raw, indices, *query, options)
	if err != nil {
		fatal(err)
	}
	if err := encoder.Encode(records); err != nil {
		fatal(err)
	}
}

func parseIndices(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	indices := make([]int, 0, len(parts))
	for _, part := range parts {
		index, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || index < 0 {
			return nil, fmt.Errorf("invalid source index %q", part)
		}
		indices = append(indices, index)
	}
	return indices, nil
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
