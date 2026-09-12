// Package library owns origin-neutral shelf metadata and reading state.
package library

import "errors"

const (
	BookSource = "booksource"
	TXT        = "txt"
)

var (
	ErrNotFound        = errors.New("library: item not found")
	ErrStateChanged    = errors.New("library: reading state changed")
	ErrInvalidProgress = errors.New("library: invalid progress")
)

// Item is shared shelf state, not a provider's catalog or acquisition context.
// ChapterIndex is meaningful only within ContentRevision.
type Item struct {
	ID                  string  `json:"id"`
	Provider            string  `json:"provider"`
	Name                string  `json:"name"`
	Author              string  `json:"author"`
	CoverURL            string  `json:"coverUrl"`
	Intro               string  `json:"intro"`
	Kind                string  `json:"kind"`
	LastChapter         string  `json:"lastChapter"`
	UpdateTime          string  `json:"updateTime"`
	WordCount           string  `json:"wordCount"`
	DurChapterIndex     int     `json:"durChapterIndex"`
	DurChapterPos       float64 `json:"durChapterPos"`
	TotalChapterNum     int     `json:"totalChapterNum"`
	CurrentChapterTitle string  `json:"currentChapterTitle"`
	ContentRevision     int64   `json:"contentRevision"`
	StateVersion        int64   `json:"stateVersion"`
	CreatedAt           int64   `json:"createdAt"`
	UpdatedAt           int64   `json:"updatedAt"`
}

type Revision struct {
	Content int64
	State   int64
}

func (i Item) Revision() Revision { return Revision{Content: i.ContentRevision, State: i.StateVersion} }
