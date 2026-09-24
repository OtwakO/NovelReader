// Source switching validates and atomically promotes an existing alternate source.
package book

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/otwako/novelreader/internal/library"
	"unicode"
)

var (
	ErrSourceNotAlternate  = errors.New("book: source is not an alternate")
	ErrInvalidSourceSwitch = errors.New("book: invalid source switch")
)

// MigrateChapterIndex matches a normalized title before using the nearest clamped raw index.
func MigrateChapterIndex(chapters []Chapter, currentTitle string, currentIndex int) (int, string) {
	title := normalizeChapterTitle(currentTitle)
	if chapter, matched := matchChapterTitle(chapters, title); matched {
		return chapter.Index, "title"
	}
	if len(chapters) == 0 {
		return -1, ""
	}
	clamped := max(0, min(currentIndex, len(chapters)-1))
	bestIndex, bestDistance := -1, len(chapters)+1
	for _, chapter := range chapters {
		if chapter.IsVolume {
			continue
		}
		distance := chapter.Index - clamped
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance || distance == bestDistance && chapter.Index < bestIndex {
			bestIndex, bestDistance = chapter.Index, distance
		}
	}
	if bestIndex < 0 {
		return -1, ""
	}
	return bestIndex, "index"
}

func matchChapterTitle(chapters []Chapter, normalizedTitle string) (Chapter, bool) {
	if normalizedTitle != "" {
		for _, chapter := range chapters {
			if !chapter.IsVolume && normalizeChapterTitle(chapter.Title) == normalizedTitle {
				return chapter, true
			}
		}
	}
	return Chapter{}, false
}

func normalizeChapterTitle(title string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return -1
	}, strings.ToLower(title))
}

// SwitchSource commits a provider-resolved interpretation and reading mapping together.
func (s *Store) SwitchSource(bookID string, expected library.Revision, target Book, chapters []Chapter, chapterIndex int, position float64) error {
	if target.SourceID == "" || target.SourceURL == "" || target.BookURL == "" || chapterIndex < 0 || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 1 {
		return ErrInvalidSourceSwitch
	}
	readable := false
	for _, chapter := range chapters {
		if chapter.Index == chapterIndex && !chapter.IsVolume {
			readable = true
			break
		}
	}
	if !readable {
		return ErrInvalidSourceSwitch
	}
	s.mergeMu.Lock()
	defer s.mergeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ctx := context.Background()
	if _, err := tx.Exec(`UPDATE books SET source_id=source_id WHERE id=?`, bookID); err != nil {
		return err
	}
	current, err := readBookTx(ctx, tx, bookID)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrBookNotFound
	}
	if current.libraryItem().Revision() != expected {
		return ErrBookStateChanged
	}
	state, err := bindingStateFromBook(current).promote(target.SourceID, target.BookURL)
	if err != nil {
		return err
	}
	bindingJSON, err := encodeBindingState(state)
	if err != nil {
		return fmt.Errorf("switch source: encode binding state: %w", err)
	}
	if _, err := tx.Exec(`UPDATE books SET source_id=?, source_url=?, book_url=?, toc_url=?, origin=?, variable_map=?, alternate_sources=? WHERE id=?`,
		target.SourceID, target.SourceURL, target.BookURL, target.TocURL, target.Origin, target.VariableMap, bindingJSON, bookID); err != nil {
		return err
	}
	item := current.libraryItem()
	item.LastChapter, item.UpdateTime, item.WordCount = target.LastChapter, target.UpdateTime, target.WordCount
	item.UpdatedAt = time.Now().UnixMilli()
	if err := library.UpdateMetadataTx(ctx, tx, item); err != nil {
		return err
	}
	if err := library.ReplaceInterpretationTx(ctx, tx, bookID, expected, len(chapters), library.Location{ChapterIndex: chapterIndex, Position: position, ChapterTitle: chapterTitleAt(chapters, chapterIndex)}); err != nil {
		return err
	}
	// Source promotion deliberately assigns IDs from the new catalog's indexes.
	replacement := make([]Chapter, len(chapters))
	for i, chapter := range chapters {
		chapter.ID = fmt.Sprintf("%s_%d", bookID, chapter.Index)
		replacement[i] = chapter
	}
	if err := replaceChaptersTx(tx, bookID, replacement, false); err != nil {
		return err
	}
	marks, err := library.BookmarksTx(ctx, tx, bookID)
	if err != nil {
		return err
	}
	for _, mark := range marks {
		chapter, matched := matchChapterTitle(chapters, normalizeChapterTitle(mark.ChapterTitle))
		mark.Orphaned = !matched
		if matched {
			mark.ChapterIndex, mark.ChapterTitle, mark.ContentRevision = chapter.Index, chapter.Title, expected.Content+1
		}
		if err := library.MapBookmarkTx(ctx, tx, mark); err != nil {
			return err
		}
	}
	return tx.Commit()
}
