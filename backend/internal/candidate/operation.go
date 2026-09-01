package candidate

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
)

func (op *operation) run() {
	defer close(op.done)
	queue := bindings(op.input)
	results := make(chan attemptResult, op.policy.Concurrency)
	next, active := 0, 0
	var winner *resolved
	completed := make([]bool, len(queue))
	successes := make([]*resolved, len(queue))
	var stoppingState State
	var stoppingMessage string
	ctxDone := op.ctx.Done()
	launch := func(index int) {
		active++
		op.update(func(s *Snapshot) { s.Active++; s.Attempts[index].State = "running" })
		go func() { results <- op.validate(index, queue[index]) }()
	}
	if len(queue) == 0 {
		op.finish(StateFailed, "candidate has no source bindings")
		op.release()
		return
	}
	for next < len(queue) && active < op.policy.Concurrency {
		launch(next)
		next++
	}
	for active > 0 {
		select {
		case <-ctxDone:
			stoppingState = StateCancelled
			stoppingMessage = "candidate resolution was cancelled"
			if errors.Is(op.ctx.Err(), context.DeadlineExceeded) {
				stoppingState = StateExhausted
				stoppingMessage = "candidate resolution reached its operation deadline"
				op.finish(stoppingState, stoppingMessage)
			}
			ctxDone = nil
		case result := <-results:
			active--
			completed[result.index] = true
			successes[result.index] = result.resolved
			drainingAfterWinner := winner != nil
			op.update(func(s *Snapshot) {
				if s.Active > 0 {
					s.Active--
				}
				s.Completed++
				if drainingAfterWinner {
					s.Attempts[result.index].State = "skipped"
					s.Attempts[result.index].Stage = result.stage
					s.Attempts[result.index].Reason = ""
				} else if result.resolved == nil {
					s.Attempts[result.index].State = "failed"
					s.Attempts[result.index].Stage = result.stage
					s.Attempts[result.index].Reason = result.reason
				} else {
					s.Attempts[result.index].State = "ready"
					s.Attempts[result.index].Stage = StageBookInfo
				}
			})
			selected := -1
			for index := range completed {
				if !completed[index] {
					break
				}
				if successes[index] != nil {
					selected = index
					break
				}
			}
			if selected >= 0 && winner == nil && stoppingState == "" {
				winner = successes[selected]
				preview := previewFromResolved(winner)
				op.update(func(s *Snapshot) {
					op.resolved = winner
					for i := range s.Attempts {
						if i == selected {
							s.Attempts[i].State = "verified"
							continue
						}
						if s.Attempts[i].State == "queued" || s.Attempts[i].State == "ready" {
							s.Attempts[i].State = "skipped"
						}
					}
					s.State = StateVerified
					s.Preview = &preview
					s.Message = "book metadata verified"
				})
				if op.commitID != "" {
					if _, err := op.commit(op.commitID); err != nil {
						op.update(func(s *Snapshot) {
							s.State = StateFailed
							s.CommitPending = true
							s.Message = "verified candidate could not be added to the shelf"
						})
						op.markTerminal()
					}
				}
				op.cancel()
				ctxDone = nil
			}
			if winner == nil && stoppingState == "" && next < len(queue) {
				launch(next)
				next++
			}
			if stoppingState != "" && active == 0 {
				if op.current().State == StateRunning {
					op.finish(stoppingState, stoppingMessage)
				}
				op.release()
				return
			}
			if winner != nil && active == 0 {
				snapshot := op.current()
				if snapshot.State == StateVerified || snapshot.CommitPending {
					go op.expirePending()
				} else {
					op.release()
				}
				return
			}
		}
	}
	if op.current().State == StateRunning {
		op.finish(StateExhausted, "no known source produced usable book metadata")
	}
	op.release()
}

func (op *operation) validate(index int, b binding) attemptResult {
	result := attemptResult{index: index, stage: StageBookInfo}
	src, err := op.runtime.Sources.GetByID(b.sourceID)
	if err != nil || src == nil {
		result.reason = "book source not found"
		return result
	}
	b.sourceGroup = src.BookSourceGroup
	b.capabilities = booksource.CapabilityTags(*src)
	candidate := inputBook(op.input, b, *src)
	if candidate, err = op.bookInfo(candidate, *src, index); err != nil {
		result.reason = err.Error()
		return result
	}
	candidate.Name = op.input.Name
	if strings.TrimSpace(op.input.Author) != "" {
		candidate.Author = op.input.Author
	}
	candidate.AlternateSources = alternates(bindings(op.input), b)
	result.resolved = &resolved{candidate, Selection{op.input.SourceID, b.sourceID, op.input.SourceURL, b.sourceURL, candidate.Origin, b.sourceURL != op.input.SourceURL || b.bookURL != op.input.BookURL}}
	return result
}

func (op *operation) bookInfo(candidate *book.Book, src booksource.BookSource, index int) (*book.Book, error) {
	op.stage(index, StageBookInfo)
	ctx, cancel := context.WithTimeout(op.ctx, op.policy.StageTimeout)
	defer cancel()
	return op.runtime.Searcher.GetBookInfoForBookContext(ctx, src, candidate, candidate.BookURL)
}
func (op *operation) stage(index int, stage Stage) {
	op.update(func(s *Snapshot) { s.Attempts[index].Stage = stage })
}

func (op *operation) commit(bookID string) (Snapshot, error) {
	op.commitMu.Lock()
	defer op.commitMu.Unlock()
	if bookID == "" {
		return op.current(), ErrInvalidBookID
	}
	op.mu.Lock()
	if op.commitResult != nil {
		snap := cloneSnapshot(op.snapshot)
		op.mu.Unlock()
		return snap, nil
	}
	resolved := op.resolved
	op.mu.Unlock()
	if resolved == nil {
		return op.current(), errNotVerified
	}
	candidate := *resolved.book
	candidate.ID = bookID
	stored, created, err := op.runtime.Books.AddOrMergeBook(&candidate)
	if err != nil {
		return op.current(), err
	}
	op.mu.Lock()
	op.commitResult = stored
	op.snapshot.State = StateCommitted
	op.snapshot.StoredBook = stored
	op.snapshot.Created = created
	op.snapshot.CommitPending = false
	op.snapshot.Message = "book added to shelf"
	op.snapshot.UpdatedAt = time.Now()
	op.broadcastLocked()
	op.mu.Unlock()
	op.markTerminal()
	return op.current(), nil
}

func (op *operation) expirePending() {
	timer := time.NewTimer(op.policy.Retention)
	defer timer.Stop()
	<-timer.C
	op.mu.Lock()
	if (op.snapshot.State == StateVerified || op.snapshot.CommitPending) && op.commitResult == nil {
		op.snapshot.State = StateFailed
		op.snapshot.CommitPending = false
		op.snapshot.Message = "verified candidate expired before it was added to the shelf"
		op.snapshot.UpdatedAt = time.Now()
		op.broadcastLocked()
		if op.terminalAt.IsZero() {
			op.terminalAt = time.Now()
		}
		op.mu.Unlock()
		op.release()
		return
	}
	op.mu.Unlock()
}

func (op *operation) finish(state State, message string) {
	op.update(func(s *Snapshot) { s.State = state; s.Message = message; s.Active = 0 })
	op.markTerminal()
}
func (op *operation) markTerminal() {
	op.mu.Lock()
	if op.terminalAt.IsZero() {
		op.terminalAt = time.Now()
	}
	op.mu.Unlock()
}
func (op *operation) release() { op.releaseOnce.Do(func() { release(op.runtime) }) }
func (op *operation) stop() {
	op.cancel()
	<-op.done
	op.release()
}
func (op *operation) current() Snapshot {
	snapshot, _ := op.currentWithTerminal()
	return snapshot
}

func (op *operation) currentWithTerminal() (Snapshot, time.Time) {
	op.mu.Lock()
	defer op.mu.Unlock()
	return cloneSnapshot(op.snapshot), op.terminalAt
}
func (op *operation) update(change func(*Snapshot)) {
	op.mu.Lock()
	change(&op.snapshot)
	op.snapshot.UpdatedAt = time.Now()
	op.broadcastLocked()
	op.mu.Unlock()
}
func (op *operation) broadcastLocked() {
	for ch := range op.subscribers {
		snap := cloneSnapshot(op.snapshot)
		select {
		case ch <- snap:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- snap:
			default:
			}
		}
	}
}

func cloneSnapshot(value Snapshot) Snapshot {
	value.Attempts = append([]Attempt(nil), value.Attempts...)
	if value.Preview != nil {
		p := *value.Preview
		p.Book.AlternateSources = append([]book.AltSource(nil), value.Preview.Book.AlternateSources...)
		value.Preview = &p
	}
	if value.StoredBook != nil {
		b := *value.StoredBook
		b.AlternateSources = append([]book.AltSource(nil), value.StoredBook.AlternateSources...)
		value.StoredBook = &b
	}
	return value
}
func previewFromResolved(value *resolved) Preview {
	return Preview{previewBook(value.book), value.selection}
}
func previewBook(value *book.Book) book.PreviewBook {
	return book.PreviewBook{Name: value.Name, Author: value.Author, CoverURL: value.CoverURL, Intro: value.Intro, Kind: value.Kind, SourceID: value.SourceID, LastChapter: value.LastChapter, UpdateTime: value.UpdateTime, WordCount: value.WordCount, Origin: value.Origin, SourceURL: value.SourceURL, BookURL: value.BookURL, TocURL: value.TocURL, AlternateSources: value.AlternateSources}
}
func inputBook(in Input, b binding, src booksource.BookSource) *book.Book {
	name := sourceName(src, b.sourceName)
	return &book.Book{Name: in.Name, Author: in.Author, CoverURL: in.CoverURL, Intro: in.Intro, Kind: in.Kind, LastChapter: in.LastChapter, UpdateTime: in.UpdateTime, WordCount: in.WordCount, Origin: name, SourceID: b.sourceID, SourceURL: b.sourceURL, BookURL: b.bookURL, ActiveSource: &book.AltSource{SourceID: b.sourceID, SourceURL: b.sourceURL, BookURL: b.bookURL, SourceName: name, SourceGroup: b.sourceGroup, Capabilities: append([]string(nil), b.capabilities...)}}
}
func sourceName(src booksource.BookSource, fallback string) string {
	if strings.TrimSpace(src.BookSourceName) != "" {
		return src.BookSourceName
	}
	return fallback
}
func bindings(in Input) []binding {
	result := make([]binding, 0, 1+len(in.AlternateSources))
	seen := map[string]bool{}
	add := func(b binding) {
		b.sourceID = strings.TrimSpace(b.sourceID)
		b.sourceURL = strings.TrimSpace(b.sourceURL)
		b.bookURL = strings.TrimSpace(b.bookURL)
		key := b.sourceID + "\x00" + b.bookURL
		if b.sourceID != "" && b.bookURL != "" && !seen[key] {
			seen[key] = true
			result = append(result, b)
		}
	}
	add(binding{sourceID: in.SourceID, sourceURL: in.SourceURL, bookURL: in.BookURL, sourceName: in.SourceName, sourceGroup: in.SourceGroup, capabilities: in.Capabilities})
	for _, a := range in.AlternateSources {
		add(binding{sourceID: a.SourceID, sourceURL: a.SourceURL, bookURL: a.BookURL, sourceName: a.SourceName, sourceGroup: a.SourceGroup, capabilities: a.Capabilities})
	}
	return result
}
func alternates(all []binding, winner binding) []book.AltSource {
	result := make([]book.AltSource, 0, len(all)-1)
	for _, b := range all {
		if b.sourceID == winner.sourceID && b.bookURL == winner.bookURL {
			continue
		}
		result = append(result, book.AltSource{SourceID: b.sourceID, SourceURL: b.sourceURL, BookURL: b.bookURL, SourceName: b.sourceName, SourceGroup: b.sourceGroup, Capabilities: b.capabilities})
	}
	return result
}
