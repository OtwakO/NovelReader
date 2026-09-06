package candidate

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

type sourceMap map[string]booksource.BookSource

func (s sourceMap) GetByID(id string) (*booksource.BookSource, error) {
	value, ok := s[id]
	if !ok {
		return nil, nil
	}
	return &value, nil
}
func (s sourceMap) ListEnabled() ([]booksource.BookSource, error) {
	values := make([]booksource.BookSource, 0, len(s))
	for _, value := range s {
		values = append(values, value)
	}
	return values, nil
}

type memoryBooks struct {
	mu     sync.Mutex
	stored *book.Book
	calls  int
	err    error
}

func (s *memoryBooks) AddOrMergeBook(value *book.Book) (*book.Book, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, false, s.err
	}
	if s.stored != nil {
		return s.stored, false, nil
	}
	copyBook := *value
	s.stored = &copyBook
	return s.stored, true, nil
}

func TestOperationPrefersPrimaryMetadataAndCommitsWithoutRecrawl(t *testing.T) {
	inputs := make([]bindingFixture, 0, 9)
	sources := sourceMap{}
	for index := 0; index < 9; index++ {
		delay := time.Duration(0)
		if index == 0 {
			delay = 80 * time.Millisecond
		}
		fixture := newBindingFixture(t, credibleText(fmt.Sprintf("source-%d", index)), delay)
		fixture.source.RuleBookInfo = `{"name":".name@text","tocUrl":".toc@href","intro":"@js:if (java.get('binding') !== source.bookSourceUrl) throw new Error('wrong binding variables'); java.put('checked', 'yes')"}`
		inputs = append(inputs, fixture)
		sources[fixture.server.URL] = fixture.source
	}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	books := &memoryBooks{}
	var released atomic.Int32
	input := Input{Name: "Fixture Novel", Author: "Fixture Author", SourceURL: inputs[0].server.URL, BookURL: inputs[0].server.URL + "/book", LastChapter: "Primary provider hint", ShelveBookID: "stored"}
	input.VariableMap = fmt.Sprintf(`{"binding":%q}`, inputs[0].server.URL)
	for _, fixture := range inputs[1:] {
		input.AlternateSources = append(input.AlternateSources, book.AltSource{SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book", VariableMap: fmt.Sprintf(`{"binding":%q}`, fixture.server.URL)})
	}
	manager := NewManager(Policy{Concurrency: 5, StageTimeout: 300 * time.Millisecond, OperationTimeout: 2 * time.Second, Retention: time.Minute})
	snapshot, err := manager.Start("reader", input, Runtime{Sources: sources, Books: books, Searcher: searcher, Release: func() { released.Add(1) }})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	sawWaitingMetadata := false
	for time.Now().Before(deadline) {
		current, _ := manager.Get("reader", snapshot.ID)
		if current.Attempts[0].State == "running" && current.Attempts[1].State == "ready" {
			if current.State != StateRunning || current.Preview != nil {
				t.Fatalf("lower-priority metadata selected early: %+v", current)
			}
			sawWaitingMetadata = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !sawWaitingMetadata {
		t.Fatal("lower-priority metadata did not finish before the preferred source")
	}
	final := waitForState(t, manager, "reader", snapshot.ID, StateCommitted)
	if final.StoredBook == nil || final.StoredBook.SourceURL != inputs[0].server.URL {
		t.Fatalf("final=%+v", final)
	}
	if final.StoredBook.Intro != "yes" || final.StoredBook.VariableMap != fmt.Sprintf(`{"binding":%q,"checked":"yes"}`, inputs[0].server.URL) {
		t.Fatalf("candidate lost updated book variables: %+v", final.StoredBook)
	}
	for _, alternate := range final.StoredBook.AlternateSources {
		if alternate.VariableMap != fmt.Sprintf(`{"binding":%q}`, alternate.SourceURL) {
			t.Fatalf("candidate lost alternate variables: %+v", alternate)
		}
	}
	if final.Attempts[0].VariableMap != input.VariableMap {
		t.Fatal("attempt must retain the input snapshot, not later crawl mutations")
	}
	if final.StoredBook.ActiveSource == nil || final.StoredBook.ActiveSource.LastChapter != "Primary provider hint" {
		t.Fatalf("active binding display snapshot=%+v", final.StoredBook.ActiveSource)
	}
	if final.Known != 9 {
		t.Fatalf("known=%d", final.Known)
	}
	if final.Attempts[0].State != "verified" || final.Attempts[1].State != "skipped" {
		t.Fatalf("primary metadata was not selected in stable order: attempts=%+v", final.Attempts)
	}
	if books.calls != 1 {
		t.Fatalf("commit calls=%d", books.calls)
	}
	deadline = time.Now().Add(time.Second)
	for released.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if released.Load() != 1 {
		t.Fatalf("runtime releases=%d", released.Load())
	}
}

func TestAutomaticCommitDoesNotWaitForLosingAttemptDrain(t *testing.T) {
	winner := newBindingFixture(t, credibleText("winner"), 0)
	loser := newBindingFixture(t, credibleText("loser"), 500*time.Millisecond)
	sources := sourceMap{winner.server.URL: winner.source, loser.server.URL: loser.source}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	manager := NewManager(Policy{Concurrency: 2, StageTimeout: time.Second, OperationTimeout: 2 * time.Second, Retention: time.Minute})
	startedAt := time.Now()
	started, err := manager.Start("reader", Input{
		Name: "Fixture Novel", SourceURL: winner.server.URL, BookURL: winner.server.URL + "/book", ShelveBookID: "stored",
		AlternateSources: []book.AltSource{{SourceURL: loser.server.URL, BookURL: loser.server.URL + "/book"}},
	}, Runtime{Sources: sources, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	final := waitForState(t, manager, "reader", started.ID, StateCommitted)
	if elapsed := time.Since(startedAt); elapsed >= 250*time.Millisecond {
		t.Fatalf("automatic commit waited for losing attempt drain: %s", elapsed)
	}
	if final.StoredBook == nil {
		t.Fatalf("committed snapshot=%+v", final)
	}
}

func TestAutomaticCommitFailureBecomesTerminalFailed(t *testing.T) {
	winner := newBindingFixture(t, credibleText("winner"), 0)
	sources := sourceMap{winner.server.URL: winner.source}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: 2 * time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: winner.server.URL, BookURL: winner.server.URL + "/book", ShelveBookID: "stored"}, Runtime{
		Sources: sources, Books: &memoryBooks{err: errors.New("storage unavailable")}, Searcher: searcher,
	})
	if err != nil {
		t.Fatal(err)
	}
	final := waitForState(t, manager, "reader", started.ID, StateFailed)
	if final.Message != "verified candidate could not be added to the shelf" || final.Preview == nil || !final.CommitPending {
		t.Fatalf("failed snapshot=%+v", final)
	}
}

func TestAutomaticCommitFailureRetriesWithoutRecrawl(t *testing.T) {
	winner := newBindingFixture(t, credibleText("winner"), 0)
	sources := sourceMap{winner.server.URL: winner.source}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	books := &memoryBooks{err: errors.New("storage unavailable")}
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: 2 * time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: winner.server.URL, BookURL: winner.server.URL + "/book", ShelveBookID: "stored"}, Runtime{Sources: sources, Books: books, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	failed := waitForState(t, manager, "reader", started.ID, StateFailed)
	if !failed.CommitPending {
		t.Fatalf("failed snapshot=%+v", failed)
	}
	books.mu.Lock()
	books.err = nil
	books.mu.Unlock()
	committed, err := manager.Commit("reader", started.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if committed.State != StateCommitted || committed.CommitPending || books.calls != 2 {
		t.Fatalf("retry snapshot=%+v calls=%d", committed, books.calls)
	}
}

func TestWinnerMarksActiveLosingAttemptsSkippedAfterCancellation(t *testing.T) {
	winner := newBindingFixture(t, credibleText("winner"), 0)
	slowerHealthy := newBindingFixture(t, credibleText("healthy loser"), 300*time.Millisecond)
	sources := sourceMap{winner.server.URL: winner.source, slowerHealthy.server.URL: slowerHealthy.source}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	manager := NewManager(Policy{Concurrency: 2, StageTimeout: time.Second, OperationTimeout: 2 * time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{
		Name: "Fixture Novel", SourceURL: winner.server.URL, BookURL: winner.server.URL + "/book", ShelveBookID: "stored",
		AlternateSources: []book.AltSource{{SourceURL: slowerHealthy.server.URL, BookURL: slowerHealthy.server.URL + "/book"}},
	}, Runtime{Sources: sources, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	waitForState(t, manager, "reader", started.ID, StateCommitted)
	select {
	case <-manager.operations[started.ID].done:
	case <-time.After(time.Second):
		t.Fatal("operation did not finish draining")
	}
	final, ok := manager.Get("reader", started.ID)
	if !ok {
		t.Fatal("operation disappeared")
	}
	if final.Attempts[0].State != "verified" || final.Attempts[1].State != "skipped" {
		t.Fatalf("winner and cancelled healthy attempt=%+v", final.Attempts)
	}
	if final.Attempts[1].Reason != "" {
		t.Fatalf("skipped attempt retained failure reason %q", final.Attempts[1].Reason)
	}
}

func TestWinnerMarksUntouchedSourcesSkipped(t *testing.T) {
	inputs := make([]bindingFixture, 0, 7)
	sources := sourceMap{}
	for index := 0; index < 7; index++ {
		delay := 100 * time.Millisecond
		content := "compatibility notice"
		if index == 0 {
			delay = 0
			content = credibleText("winner")
		}
		fixture := newBindingFixture(t, content, delay)
		inputs = append(inputs, fixture)
		sources[fixture.server.URL] = fixture.source
	}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	input := Input{Name: "Fixture Novel", SourceURL: inputs[0].server.URL, BookURL: inputs[0].server.URL + "/book", ShelveBookID: "stored"}
	for _, fixture := range inputs[1:] {
		input.AlternateSources = append(input.AlternateSources, book.AltSource{SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"})
	}
	manager := NewManager(Policy{Concurrency: 5, StageTimeout: time.Second, OperationTimeout: 2 * time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", input, Runtime{Sources: sources, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	final := waitForState(t, manager, "reader", started.ID, StateCommitted)
	if final.Attempts[0].State != "verified" || final.Attempts[5].State != "skipped" || final.Attempts[6].State != "skipped" {
		t.Fatalf("winner and skipped attempts=%+v", final.Attempts)
	}
}

func TestStartReturnsCompleteQueuedAttemptSnapshot(t *testing.T) {
	fixture := newBindingFixture(t, credibleText("slow"), 100*time.Millisecond)
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sourceMap{fixture.server.URL: fixture.source}, nil)
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"}, Runtime{Sources: sourceMap{fixture.server.URL: fixture.source}, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Cancel("reader", started.ID)
	if len(started.Attempts) != 1 || started.Attempts[0].State != "queued" || started.Attempts[0].SourceURL != fixture.server.URL {
		t.Fatalf("initial attempts=%+v", started.Attempts)
	}
	if started.AutomaticCommit {
		t.Fatal("detail operation unexpectedly marked for automatic commit")
	}
}

func TestStartMarksAutomaticShelfIntentInEverySnapshot(t *testing.T) {
	fixture := newBindingFixture(t, credibleText("automatic"), 100*time.Millisecond)
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sourceMap{fixture.server.URL: fixture.source}, nil)
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book", ShelveBookID: "stored"}, Runtime{Sources: sourceMap{fixture.server.URL: fixture.source}, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	if !started.AutomaticCommit {
		t.Fatalf("initial snapshot=%+v", started)
	}
	final := waitForState(t, manager, "reader", started.ID, StateCommitted)
	if !final.AutomaticCommit {
		t.Fatalf("committed snapshot=%+v", final)
	}
}

func TestAdmissionDoesNotEvaluateLargeTOC(t *testing.T) {
	fixture := newBindingFixture(t, credibleText("unused"), 0)
	fixture.source.RuleToc = `{"chapterList":"<js>while(true){result.push({text:'Chapter',href:'/chapter'})}</js>","chapterName":"text","chapterUrl":"href"}`
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: 100 * time.Millisecond, OperationTimeout: time.Second, Retention: time.Minute})
	defer manager.Close()
	input := Input{Name: "Book", Author: "Author", SourceID: fixture.source.ID, SourceURL: fixture.source.BookSourceURL, BookURL: fixture.server.URL + "/book"}
	started := time.Now()
	sources := sourceMap{fixture.source.ID: fixture.source}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	snapshot, err := manager.Start("reader", input, Runtime{Sources: sources, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	snapshot = waitForState(t, manager, "reader", snapshot.ID, StateVerified)
	if time.Since(started) > 500*time.Millisecond || snapshot.Attempts[0].Stage != StageBookInfo {
		t.Fatalf("elapsed=%s snapshot=%+v", time.Since(started), snapshot)
	}
}

func TestDefaultPolicyStartsFiveSourcesImmediately(t *testing.T) {
	inputs := make([]bindingFixture, 0, 6)
	sources := sourceMap{}
	for range 6 {
		fixture := newBindingFixture(t, credibleText("slow"), 100*time.Millisecond)
		inputs = append(inputs, fixture)
		sources[fixture.server.URL] = fixture.source
	}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	input := Input{Name: "Fixture Novel", SourceURL: inputs[0].server.URL, BookURL: inputs[0].server.URL + "/book"}
	for _, fixture := range inputs[1:] {
		input.AlternateSources = append(input.AlternateSources, book.AltSource{SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"})
	}
	manager := NewManager(DefaultPolicy())
	started, err := manager.Start("reader", input, Runtime{Sources: sources, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Cancel("reader", started.ID)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, _ := manager.Get("reader", started.ID)
		if snapshot.Active == 5 {
			if queued := countAttempts(snapshot.Attempts, "queued"); queued != 1 {
				t.Fatalf("queued=%d attempts=%+v", queued, snapshot.Attempts)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	snapshot, _ := manager.Get("reader", started.ID)
	t.Fatalf("expected five active attempts, snapshot=%+v", snapshot)
}

func TestFailedAttemptImmediatelyRefillsFifthWorkerSlot(t *testing.T) {
	inputs := make([]bindingFixture, 0, 6)
	sources := sourceMap{}
	for index := 0; index < 6; index++ {
		delay := 300 * time.Millisecond
		content := credibleText(fmt.Sprintf("slow-%d", index))
		if index == 0 {
			delay = 0
		}
		fixture := newBindingFixture(t, content, delay)
		inputs = append(inputs, fixture)
		sources[fixture.server.URL] = fixture.source
	}
	delete(sources, inputs[0].server.URL)
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	input := Input{Name: "Fixture Novel", SourceID: inputs[0].server.URL, SourceURL: inputs[0].server.URL, BookURL: inputs[0].server.URL + "/book"}
	for _, fixture := range inputs[1:] {
		input.AlternateSources = append(input.AlternateSources, book.AltSource{SourceID: fixture.server.URL, SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"})
	}
	manager := NewManager(Policy{Concurrency: 5, StageTimeout: 2 * time.Second, OperationTimeout: 3 * time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", input, Runtime{Sources: sources, Books: &memoryBooks{}, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Cancel("reader", started.ID)
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		snapshot, _ := manager.Get("reader", started.ID)
		if snapshot.Attempts[0].State == "failed" && snapshot.Attempts[5].State == "running" {
			if snapshot.Active != 5 || snapshot.Completed != 1 {
				t.Fatalf("refill snapshot=%+v", snapshot)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	snapshot, _ := manager.Get("reader", started.ID)
	t.Fatalf("sixth source did not immediately refill failed slot: %+v", snapshot)
}

func countAttempts(attempts []Attempt, state string) int {
	count := 0
	for _, attempt := range attempts {
		if attempt.State == state {
			count++
		}
	}
	return count
}

func TestCloseWaitsForCancelledAttemptBeforeReleasingRuntime(t *testing.T) {
	startedRequest := make(chan struct{})
	var startedOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedOnce.Do(func() { close(startedRequest) })
		<-r.Context().Done()
	}))
	defer server.Close()
	source := booksource.BookSource{ID: server.URL, BookSourceName: "Blocking", BookSourceURL: server.URL, Enabled: true, SearchURL: server.URL, RuleBookInfo: `{"name":".name@text"}`}
	sources := sourceMap{server.URL: source}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	released := make(chan struct{})
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: server.URL, BookURL: server.URL + "/book"}, Runtime{Sources: sources, Books: &memoryBooks{}, Searcher: searcher, Release: func() { close(released) }})
	if err != nil {
		t.Fatal(err)
	}
	<-startedRequest
	if !manager.Cancel("reader", started.ID) {
		t.Fatal("cancel failed")
	}
	manager.Close()
	select {
	case <-released:
	default:
		t.Fatal("manager returned before releasing the drained runtime")
	}
}

func TestCancelReleasesRuntimeAfterActiveAttemptStops(t *testing.T) {
	fixture := newBindingFixture(t, credibleText("cancel"), 80*time.Millisecond)
	sources := sourceMap{fixture.server.URL: fixture.source}
	books := &memoryBooks{}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: time.Second, Retention: time.Minute})
	type releaseState struct {
		active int
		state  State
	}
	released := make(chan releaseState, 1)
	operationID := ""
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"}, Runtime{Sources: sources, Books: books, Searcher: searcher, Release: func() {
		snapshot, _ := manager.Get("reader", operationID)
		released <- releaseState{snapshot.Active, snapshot.State}
	}})
	if err != nil {
		t.Fatal(err)
	}
	operationID = started.ID
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, _ := manager.Get("reader", started.ID)
		if snapshot.Active == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !manager.Cancel("reader", started.ID) {
		t.Fatal("cancel failed")
	}
	cancelled, ok := manager.Get("reader", started.ID)
	if !ok || cancelled.State != StateCancelled || cancelled.Active != 0 {
		t.Fatalf("cancel acknowledgement=%+v", cancelled)
	}
	select {
	case state := <-released:
		if state.active != 0 || state.state != StateCancelled {
			t.Fatalf("released with active=%d state=%s", state.active, state.state)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime was not released after cancellation drained")
	}
}

func TestOperationSubscriptionImmediatelyReceivesCurrentSnapshot(t *testing.T) {
	fixture := newBindingFixture(t, credibleText("slow"), 20*time.Millisecond)
	sources := sourceMap{fixture.server.URL: fixture.source}
	books := &memoryBooks{}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"}, Runtime{Sources: sources, Books: books, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	updates, unsubscribe, ok := manager.Subscribe("reader", started.ID)
	if !ok {
		t.Fatal("subscribe failed")
	}
	defer unsubscribe()
	select {
	case snapshot := <-updates:
		if snapshot.ID != started.ID {
			t.Fatalf("snapshot=%+v", snapshot)
		}
	case <-time.After(time.Second):
		t.Fatal("initial snapshot not delivered")
	}
}

func TestCommitIsIdempotentAcrossConcurrentRetries(t *testing.T) {
	fixture := newBindingFixture(t, credibleText("commit"), 0)
	sources := sourceMap{fixture.server.URL: fixture.source}
	books := &memoryBooks{}
	searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
	manager := NewManager(Policy{Concurrency: 1, StageTimeout: time.Second, OperationTimeout: time.Second, Retention: time.Minute})
	started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"}, Runtime{Sources: sources, Books: books, Searcher: searcher})
	if err != nil {
		t.Fatal(err)
	}
	waitForState(t, manager, "reader", started.ID, StateVerified)
	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			if _, err := manager.Commit("reader", started.ID, "stored"); err != nil {
				t.Errorf("commit: %v", err)
			}
		}()
	}
	wg.Wait()
	if books.calls != 1 {
		t.Fatalf("commit calls=%d", books.calls)
	}
}

func waitForState(t *testing.T, manager *Manager, owner, id string, want State) Snapshot {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := manager.Get(owner, id)
		if !ok {
			t.Fatal("operation disappeared")
		}
		if snapshot.State == want {
			return snapshot
		}
		if snapshot.State == StateFailed || snapshot.State == StateExhausted || snapshot.State == StateCancelled {
			t.Fatalf("terminal state=%s message=%s", snapshot.State, snapshot.Message)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("operation did not reach %s", want)
	return Snapshot{}
}

type bindingFixture struct {
	server *httptest.Server
	source booksource.BookSource
}

func newBindingFixture(t *testing.T, content string, delay time.Duration) bindingFixture {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		switch r.URL.Path {
		case "/book":
			fmt.Fprint(w, `<h1 class="name">Renamed Novel</h1><span class="author">Renamed Author</span><a class="toc" href="/toc">TOC</a>`)
		case "/toc":
			fmt.Fprint(w, `<a class="chapter" href="/chapter">Chapter</a>`)
		case "/chapter":
			fmt.Fprintf(w, `<article class="content">%s</article>`, content)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	source := booksource.BookSource{ID: server.URL, BookSourceName: "Fixture", BookSourceURL: server.URL, Enabled: true, SearchURL: server.URL, RuleBookInfo: `{"name":".name@text","author":".author@text","tocUrl":".toc@href"}`, RuleToc: `{"chapterList":".chapter","chapterName":"text","chapterUrl":"href"}`, RuleContent: `{"content":".content@text"}`}
	return bindingFixture{server, source}
}
func credibleText(label string) string {
	return fmt.Sprintf("%s chapter prose contains enough meaningful narrative text to verify that this source returned real readable content rather than an empty page, login prompt, access denial, or browser compatibility notice.", label)
}

// Cancellation and expiry must revoke write eligibility, not just release the runtime.
func TestTerminalCandidateCannotCommit(t *testing.T) {
	for _, terminal := range []string{"cancelled", "expired"} {
		t.Run(terminal, func(t *testing.T) {
			fixture := newBindingFixture(t, credibleText("terminal"), 0)
			sources := sourceMap{fixture.server.URL: fixture.source}
			books := &memoryBooks{}
			searcher := book.NewSearcher(fetcher.NewInsecureStateless(time.Second), analyzer.NewJSVM(), analyzer.NewCacheManager(), sources, nil)
			policy := DefaultPolicy()
			if terminal == "expired" {
				policy.Retention = 50 * time.Millisecond
			}
			manager := NewManager(policy)
			defer manager.Close()
			released := make(chan struct{})
			started, err := manager.Start("reader", Input{Name: "Fixture Novel", SourceURL: fixture.server.URL, BookURL: fixture.server.URL + "/book"}, Runtime{Sources: sources, Books: books, Searcher: searcher, Release: func() { close(released) }})
			if err != nil {
				t.Fatal(err)
			}
			if terminal == "cancelled" {
				waitForState(t, manager, "reader", started.ID, StateVerified)
				manager.Cancel("reader", started.ID)
			}
			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("terminal operation retained runtime")
			}
			if _, err := manager.Commit("reader", started.ID, "must-not-exist"); err == nil {
				t.Fatal("terminal candidate accepted a new commit")
			}
			if books.calls != 0 {
				t.Fatalf("terminal candidate wrote %d times", books.calls)
			}
		})
	}
}
