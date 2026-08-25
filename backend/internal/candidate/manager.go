// Package candidate resolves transient book candidates into verified readable books.
package candidate

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/book"
	"github.com/otwako/novelreader/internal/booksource"
)

const (
	defaultConcurrency      = 5
	defaultStageTimeout     = 15 * time.Second
	defaultOperationTimeout = 3 * time.Minute
	defaultRetention        = 15 * time.Minute
	maxOperationsPerOwner   = 8
)

type SourceStore interface {
	GetByID(string) (*booksource.BookSource, error)
}
type BookStore interface {
	AddOrMergeBookWithChapters(*book.Book, []book.Chapter) (*book.Book, bool, error)
}

type Runtime struct {
	Sources  SourceStore
	Books    BookStore
	Searcher *book.Searcher
	Release  func()
}

type Input struct {
	Name             string           `json:"name"`
	Author           string           `json:"author,omitempty"`
	CoverURL         string           `json:"coverUrl,omitempty"`
	Intro            string           `json:"intro,omitempty"`
	Kind             string           `json:"kind,omitempty"`
	LastChapter      string           `json:"lastChapter,omitempty"`
	UpdateTime       string           `json:"updateTime,omitempty"`
	WordCount        string           `json:"wordCount,omitempty"`
	SourceName       string           `json:"sourceName,omitempty"`
	SourceURL        string           `json:"sourceUrl"`
	BookURL          string           `json:"bookUrl"`
	AlternateSources []book.AltSource `json:"alternateSources,omitempty"`
	ShelveBookID     string           `json:"shelveBookId,omitempty"`
}

type State string

const (
	StateRunning   State = "running"
	StateVerified  State = "verified"
	StateCommitted State = "committed"
	StateExhausted State = "exhausted"
	StateCancelled State = "cancelled"
	StateFailed    State = "failed"
)

type Stage string

const (
	StageBookInfo Stage = "book_info"
	StageTOC      Stage = "toc"
	StageContent  Stage = "content"
)

type Attempt struct {
	SourceName string `json:"sourceName,omitempty"`
	SourceURL  string `json:"sourceUrl"`
	BookURL    string `json:"bookUrl"`
	Stage      Stage  `json:"stage,omitempty"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
}

type Selection struct {
	RequestedSourceURL string `json:"requestedSourceUrl"`
	SelectedSourceURL  string `json:"selectedSourceUrl"`
	SelectedSourceName string `json:"selectedSourceName,omitempty"`
	UsedFallback       bool   `json:"usedFallback"`
}

type Preview struct {
	Book      book.PreviewBook `json:"book"`
	Chapters  []book.Chapter   `json:"chapters"`
	Selection Selection        `json:"selection"`
}

type Snapshot struct {
	ID              string     `json:"id"`
	State           State      `json:"state"`
	Known           int        `json:"known"`
	Completed       int        `json:"completed"`
	Active          int        `json:"active"`
	Attempts        []Attempt  `json:"attempts"`
	Preview         *Preview   `json:"preview,omitempty"`
	StoredBook      *book.Book `json:"storedBook,omitempty"`
	Created         bool       `json:"created,omitempty"`
	CommitPending   bool       `json:"commitPending,omitempty"`
	AutomaticCommit bool       `json:"automaticCommit,omitempty"`
	Message         string     `json:"message,omitempty"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type Policy struct {
	Concurrency      int
	StageTimeout     time.Duration
	OperationTimeout time.Duration
	Retention        time.Duration
}

func DefaultPolicy() Policy {
	return Policy{defaultConcurrency, defaultStageTimeout, defaultOperationTimeout, defaultRetention}
}

var (
	ErrOperationNotFound = errors.New("candidate operation not found")
	errNotVerified       = errors.New("candidate is not verified")
	ErrInvalidBookID     = errors.New("book id is required")
)

type Manager struct {
	mu         sync.Mutex
	operations map[string]*operation
	policy     Policy
	closed     bool
}

type operation struct {
	mu           sync.Mutex
	id           string
	ownerID      string
	input        Input
	runtime      Runtime
	policy       Policy
	ctx          context.Context
	cancel       context.CancelFunc
	snapshot     Snapshot
	resolved     *resolved
	subscribers  map[chan Snapshot]struct{}
	terminalAt   time.Time
	commitID     string
	commitResult *book.Book
	releaseOnce  sync.Once
	done         chan struct{}
	commitMu     sync.Mutex
}

type binding struct{ sourceURL, bookURL, sourceName string }
type resolved struct {
	book      *book.Book
	chapters  []book.Chapter
	selection Selection
}
type attemptResult struct {
	index    int
	resolved *resolved
	stage    Stage
	reason   string
}

func NewManager(policy Policy) *Manager {
	if policy.Concurrency < 1 {
		policy.Concurrency = defaultConcurrency
	}
	if policy.StageTimeout <= 0 {
		policy.StageTimeout = defaultStageTimeout
	}
	if policy.OperationTimeout <= 0 {
		policy.OperationTimeout = defaultOperationTimeout
	}
	if policy.Retention <= 0 {
		policy.Retention = defaultRetention
	}
	return &Manager{operations: make(map[string]*operation), policy: policy}
}

func (m *Manager) Start(ownerID string, input Input, runtime Runtime) (Snapshot, error) {
	if strings.TrimSpace(ownerID) == "" || runtime.Sources == nil || runtime.Books == nil || runtime.Searcher == nil {
		return Snapshot{}, errors.New("candidate runtime is incomplete")
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.SourceURL) == "" || strings.TrimSpace(input.BookURL) == "" {
		return Snapshot{}, errors.New("candidate name, sourceUrl, and bookUrl are required")
	}
	id, err := randomID()
	if err != nil {
		return Snapshot{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.policy.OperationTimeout)
	queue := bindings(input)
	attempts := make([]Attempt, len(queue))
	for i, binding := range queue {
		attempts[i] = Attempt{SourceName: binding.sourceName, SourceURL: binding.sourceURL, BookURL: binding.bookURL, State: "queued"}
	}
	op := &operation{id: id, ownerID: ownerID, input: input, runtime: runtime, policy: m.policy, ctx: ctx, cancel: cancel, subscribers: make(map[chan Snapshot]struct{}), commitID: strings.TrimSpace(input.ShelveBookID), done: make(chan struct{})}
	op.snapshot = Snapshot{ID: id, State: StateRunning, Known: len(queue), Attempts: attempts, AutomaticCommit: op.commitID != "", UpdatedAt: time.Now()}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancel()
		release(runtime)
		return Snapshot{}, errors.New("candidate manager is closed")
	}
	stopped := m.pruneLocked(ownerID)
	m.operations[id] = op
	m.mu.Unlock()
	for _, old := range stopped {
		old.stop()
	}
	go op.run()
	return op.current(), nil
}

func (m *Manager) Get(ownerID, id string) (Snapshot, bool) {
	op, ok := m.operation(ownerID, id)
	if !ok {
		return Snapshot{}, false
	}
	return op.current(), true
}

func (m *Manager) Subscribe(ownerID, id string) (<-chan Snapshot, func(), bool) {
	op, ok := m.operation(ownerID, id)
	if !ok {
		return nil, nil, false
	}
	ch := make(chan Snapshot, 1)
	op.mu.Lock()
	op.subscribers[ch] = struct{}{}
	ch <- cloneSnapshot(op.snapshot)
	op.mu.Unlock()
	return ch, func() {
		op.mu.Lock()
		if _, ok := op.subscribers[ch]; ok {
			delete(op.subscribers, ch)
			close(ch)
		}
		op.mu.Unlock()
	}, true
}

func (m *Manager) Cancel(ownerID, id string) bool {
	op, ok := m.operation(ownerID, id)
	if !ok {
		return false
	}
	snapshot := op.current()
	if snapshot.State == StateVerified {
		op.finish(StateCancelled, "candidate resolution was cancelled")
		op.cancel()
		select {
		case <-op.done:
			op.release()
		default:
		}
		return true
	}
	if snapshot.State == StateRunning {
		op.finish(StateCancelled, "candidate resolution was cancelled")
		op.cancel()
	}
	return true
}

func (m *Manager) Commit(ownerID, id, bookID string) (Snapshot, error) {
	op, ok := m.operation(ownerID, id)
	if !ok {
		return Snapshot{}, ErrOperationNotFound
	}
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		bookID = op.commitID
	}
	snapshot, err := op.commit(bookID)
	if err == nil {
		select {
		case <-op.done:
			op.release()
		default:
		}
	}
	return snapshot, err
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	ops := make([]*operation, 0, len(m.operations))
	for _, op := range m.operations {
		ops = append(ops, op)
	}
	m.mu.Unlock()
	for _, op := range ops {
		op.stop()
	}
}

func (m *Manager) operation(ownerID, id string) (*operation, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op, ok := m.operations[id]
	return op, ok && op.ownerID == ownerID
}
func (m *Manager) pruneLocked(ownerID string) []*operation {
	now := time.Now()
	stopped := make([]*operation, 0)
	ownerCount := 0
	oldestID := ""
	var oldest time.Time
	for id, op := range m.operations {
		snap, terminalAt := op.currentWithTerminal()
		if !terminalAt.IsZero() && now.Sub(terminalAt) > m.policy.Retention {
			delete(m.operations, id)
			stopped = append(stopped, op)
			continue
		}
		if op.ownerID == ownerID {
			ownerCount++
			if oldestID == "" || snap.UpdatedAt.Before(oldest) {
				oldestID, oldest = id, snap.UpdatedAt
			}
		}
	}
	if ownerCount >= maxOperationsPerOwner && oldestID != "" {
		old := m.operations[oldestID]
		delete(m.operations, oldestID)
		stopped = append(stopped, old)
	}
	return stopped
}

func randomID() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("candidate operation id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func release(runtime Runtime) {
	if runtime.Release != nil {
		runtime.Release()
	}
}
