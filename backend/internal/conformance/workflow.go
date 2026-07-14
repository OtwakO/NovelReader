// Reproducible detail, TOC, and chapter-content conformance workflow.
package conformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
	"github.com/otwako/novelreader/internal/fingerprint"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/webview"
)

// WorkflowRecord captures the small, bounded output of one book workflow.
type WorkflowRecord struct {
	Identity       SourceIdentity `json:"identity"`
	SourceName     string         `json:"sourceName"`
	BookURL        string         `json:"bookUrl"`
	Detail         *book.Book     `json:"detail,omitempty"`
	ChapterCount   int            `json:"chapterCount,omitempty"`
	FirstChapter   *book.Chapter  `json:"firstChapter,omitempty"`
	ContentTitle   string         `json:"contentTitle,omitempty"`
	ContentSample  string         `json:"contentSample,omitempty"`
	Classification string         `json:"classification"`
	Stage          string         `json:"stage,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// RunWorkflowWithOptions executes detail, TOC, and first-chapter content for one source.
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

	chapters, err := searcher.GetChapterList(src, bookURL, detail.TocURL)
	if err != nil {
		record.Stage, record.Classification, record.Error = "toc", "toc_failure", err.Error()
		return record, nil
	}
	record.ChapterCount = len(chapters)
	for i := range chapters {
		if chapters[i].URL != "" && !chapters[i].IsVolume {
			record.FirstChapter = &chapters[i]
			break
		}
	}
	if record.FirstChapter == nil {
		record.Stage, record.Classification = "toc", "toc_empty"
		return record, nil
	}

	content, title, err := searcher.GetChapterContent(src, record.FirstChapter.URL)
	if err != nil {
		record.Stage, record.Classification, record.Error = "content", "content_failure", err.Error()
		return record, nil
	}
	record.ContentTitle = title
	record.ContentSample = sample(content)
	record.Stage, record.Classification = "content", "success"
	return record, nil
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
