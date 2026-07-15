// Reproducible detail, TOC, and chapter-content conformance workflow.
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
	"github.com/otwako/novelreader/internal/webview"
)

// ChapterCheck captures one bounded first/middle/last content probe.
type ChapterCheck struct {
	Position      string       `json:"position"`
	Chapter       book.Chapter `json:"chapter"`
	ContentTitle  string       `json:"contentTitle,omitempty"`
	ContentSample string       `json:"contentSample,omitempty"`
}

// WorkflowRecord captures the small, bounded output of one book workflow.
type WorkflowRecord struct {
	Identity       SourceIdentity `json:"identity"`
	SourceName     string         `json:"sourceName"`
	BookURL        string         `json:"bookUrl"`
	Detail         *book.Book     `json:"detail,omitempty"`
	ChapterCount   int            `json:"chapterCount,omitempty"`
	ChapterChecks  []ChapterCheck `json:"chapterChecks,omitempty"`
	FirstChapter   *book.Chapter  `json:"firstChapter,omitempty"` // retained for CLI compatibility
	ContentTitle   string         `json:"contentTitle,omitempty"`
	ContentSample  string         `json:"contentSample,omitempty"`
	Classification string         `json:"classification"`
	Stage          string         `json:"stage,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// RunWorkflowWithOptions executes detail, TOC, and first/middle/last chapter content for one source.
func RunWorkflowWithOptions(ctx context.Context, raw []byte, index int, bookURL string, options Options) (WorkflowRecord, error) {
	items, err := rawItems(raw)
	if err != nil {
		return WorkflowRecord{}, err
	}
	if index < 0 || index >= len(items) {
		return WorkflowRecord{}, fmt.Errorf("conformance: source index %d outside [0,%d)", index, len(items))
	}
	if bookURL == "" {
		return WorkflowRecord{}, fmt.Errorf("conformance: workflow book URL is required")
	}
	if err := ctx.Err(); err != nil {
		return WorkflowRecord{}, err
	}

	var src booksource.BookSource
	if err := json.Unmarshal(items[index], &src); err != nil {
		return WorkflowRecord{}, fmt.Errorf("conformance: source %d: %w", index, err)
	}
	hash := sha256.Sum256(items[index])
	record := WorkflowRecord{
		Identity: SourceIdentity{
			Index: index, URL: src.BookSourceURL, SHA256: hex.EncodeToString(hash[:]),
		},
		SourceName: src.BookSourceName,
		BookURL:    bookURL,
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	searcher, err := newWorkflowSearcher(timeout, options)
	if err != nil {
		return record, err
	}

	detail, err := searcher.GetBookInfo(src, bookURL)
	if err != nil {
		record.Stage, record.Classification, record.Error = "detail", "detail_failure", err.Error()
		return record, nil
	}
	record.Detail = detail

	chapters, err := searcher.GetChapterListForBook(src, detail, detail.TocURL)
	if err != nil {
		record.Stage, record.Classification, record.Error = "toc", "toc_failure", err.Error()
		return record, nil
	}
	record.ChapterCount = len(chapters)
	checks := chapterCheckIndexes(chapters)
	if len(checks) == 0 {
		record.Stage, record.Classification = "toc", "toc_empty"
		return record, nil
	}

	for _, check := range checks {
		chapter := &chapters[check.index]
		if record.FirstChapter == nil {
			record.FirstChapter = chapter
		}
		var next *book.Chapter
		if check.index+1 < len(chapters) {
			next = &chapters[check.index+1]
		}
		content, title, err := searcher.GetChapterContentForBook(src, detail, chapter, next)
		if err != nil {
			record.Stage, record.Classification, record.Error = "content_"+check.position, "content_failure", err.Error()
			return record, nil
		}
		if strings.TrimSpace(content) == "" {
			record.Stage, record.Classification, record.Error = "content_"+check.position, "content_empty", "chapter content is empty"
			return record, nil
		}
		result := ChapterCheck{Position: check.position, Chapter: *chapter, ContentTitle: title, ContentSample: sample(content)}
		record.ChapterChecks = append(record.ChapterChecks, result)
		if check.position == "first" {
			record.ContentTitle = title
			record.ContentSample = result.ContentSample
		}
	}
	record.Stage, record.Classification = "content", "success"
	return record, nil
}

type chapterCheckIndex struct {
	position string
	index    int
}

func chapterCheckIndexes(chapters []book.Chapter) []chapterCheckIndex {
	readable := make([]int, 0, len(chapters))
	for i := range chapters {
		if chapters[i].URL != "" && !chapters[i].IsVolume {
			readable = append(readable, i)
		}
	}
	if len(readable) == 0 {
		return nil
	}
	candidates := []chapterCheckIndex{
		{position: "first", index: readable[0]},
		{position: "middle", index: readable[len(readable)/2]},
		{position: "last", index: readable[len(readable)-1]},
	}
	checks := make([]chapterCheckIndex, 0, 3)
	seen := make(map[int]bool, 3)
	for _, candidate := range candidates {
		if !seen[candidate.index] {
			seen[candidate.index] = true
			checks = append(checks, candidate)
		}
	}
	return checks
}

func newWorkflowSearcher(timeout time.Duration, options Options) (*book.Searcher, error) {
	jsVM := analyzer.NewJSVM()
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(timeout), jsVM, nil, nil, nil)
	searcher.SetWorkflowTimeout(timeout)
	searcher.SetTransportFactory(func(client *fetcher.Client, session *sourceexec.SourceSession) sourceexec.Transport {
		normal := sourceexec.NewHTTPTransportForSession(client, session)
		if !options.Fingerprint {
			return normal
		}
		transport, err := fingerprint.NewTransport(fingerprint.Config{
			Timeout:            minDuration(timeout, 5*time.Second),
			InsecureSkipVerify: true,
		}, normal, session)
		if err != nil {
			return normal
		}
		return transport
	})
	if options.WebViewEndpoint == "" {
		return searcher, nil
	}
	client, err := webview.NewClient(webview.Config{Endpoint: options.WebViewEndpoint, Timeout: timeout})
	if err != nil {
		return nil, err
	}
	searcher.SetWebViewTransportFactory(client.ForSession)
	return searcher, nil
}
