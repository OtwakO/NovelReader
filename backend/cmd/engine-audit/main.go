// engine-audit is an explicit local Search → Book Info probe, not a CI test.
// Its JSON contains private requests/responses: redirect only into ignored test-booksources/.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fingerprint"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/webview"
)

type installedSource struct{ source booksource.BookSource }

func (s installedSource) ListEnabled() ([]booksource.BookSource, error) {
	return []booksource.BookSource{s.source}, nil
}

type exchange struct {
	Stage         string
	Request       sourceexec.RequestSpec
	Response      sourceexec.Response
	Error         string
	DurationMS    int64
	BodyTruncated bool
}

type observer struct {
	sourceexec.Transport
	stage     *string
	exchanges *[]exchange
}

func (o observer) Do(ctx context.Context, request sourceexec.RequestSpec) (sourceexec.Response, error) {
	start := time.Now()
	response, err := o.Transport.Do(ctx, request)
	captured := response
	truncated := len(captured.Body) > 1<<20
	if truncated {
		captured.Body = captured.Body[:1<<20]
	}
	*o.exchanges = append(*o.exchanges, exchange{*o.stage, request, captured, errorText(err), time.Since(start).Milliseconds(), truncated})
	return response, err
}

func (o observer) CloseIdleConnections() {
	if closer, ok := o.Transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func main() {
	input := flag.String("sources", "", "private frozen JSON array")
	index := flag.Int("index", -1, "frozen array index")
	query := flag.String("query", "凡人修仙传", "search query")
	browser := flag.String("webview", "", "optional disposable worker endpoint")
	flag.Parse()
	raw, err := os.ReadFile(*input)
	if err != nil {
		log.Fatal(err)
	}
	var sources []json.RawMessage
	if err := json.Unmarshal(raw, &sources); err != nil {
		log.Fatal(err)
	}
	if *index < 0 || *index >= len(sources) {
		log.Fatal("index outside frozen array")
	}
	var source booksource.BookSource
	if err := json.Unmarshal(sources[*index], &source); err != nil {
		log.Fatal(err)
	}
	// Installed identity is runtime metadata; the source definition stays unchanged.
	source.ID = fmt.Sprintf("audit-%d", *index)

	client := fetcher.NewInsecure(15 * time.Second)
	config := fingerprint.Config{Timeout: 15 * time.Second, Profile: os.Getenv("TLS_CLIENT_PROFILE"), InsecureSkipVerify: true}
	jsHTTP, err := fingerprint.New(config, fetcher.NewInsecureStateless(15*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	vm := analyzer.NewJSVMWithPoolSize(4)
	vm.SetFetcher(jsHTTP)
	searcher := book.NewSearcher(client, vm, analyzer.NewCacheManager(), installedSource{source}, nil)
	var exchanges []exchange
	stage := "search"
	config.Timeout = 5 * time.Second
	searcher.SetTransportFactory(func(client *fetcher.Client, session *sourceexec.SourceSession) sourceexec.Transport {
		normal := sourceexec.NewHTTPTransportForSession(client, session)
		transport, err := fingerprint.NewTransport(config, normal, session)
		if err != nil {
			log.Fatal(err)
		}
		return observer{transport, &stage, &exchanges}
	})
	if *browser != "" {
		worker, err := webview.NewClient(webview.Config{Endpoint: *browser})
		if err != nil {
			log.Fatal(err)
		}
		searcher.SetWebViewTransportFactory(func(session *sourceexec.SourceSession) sourceexec.Transport {
			return observer{worker.ForSession(session), &stage, &exchanges}
		})
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	results, searchErr := searcher.SearchInstalledSource(ctx, source.ID, *query)
	cancel()
	searchMS := time.Since(start).Milliseconds()
	var detail *book.Book
	var detailErr error
	detailAttempted := false
	stage = "detail"
	for _, result := range results {
		if result.Name == "" || result.BookURL == "" {
			continue
		}
		candidate := &book.Book{VariableMap: result.VariableMap, Name: result.Name, Author: result.Author, CoverURL: result.CoverURL, Intro: result.Intro, Kind: result.Kind, LastChapter: result.LastChapter, WordCount: result.WordCount, UpdateTime: result.UpdateTime, SourceURL: source.BookSourceURL, BookURL: result.BookURL, Origin: source.BookSourceName}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		detail, detailErr = searcher.GetBookInfoForBookContext(ctx, source, candidate, result.BookURL)
		cancel()
		detailAttempted = true
		break
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{
		"index": *index, "searchError": errorText(searchErr), "results": results, "searchMS": searchMS,
		"detailAttempted": detailAttempted, "detailError": errorText(detailErr), "detail": detail,
		"exchanges": exchanges, "webviewConfigured": *browser != "",
	}); err != nil {
		log.Fatal(err)
	}
}
