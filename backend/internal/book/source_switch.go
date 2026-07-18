// Source switching validates and atomically promotes an existing alternate source.
package book

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
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

// SwitchSource atomically replaces active crawl state after the caller validates the target TOC.
func (s *Store) SwitchSource(bookID string, expectedVersion int64, target Book, chapters []Chapter, chapterIndex int, position float64) error {
	if target.SourceURL == "" || target.BookURL == "" || chapterIndex < 0 || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 1 {
		return ErrInvalidSourceSwitch
	}
	readableProgress := false
	for _, chapter := range chapters {
		if chapter.Index == chapterIndex && !chapter.IsVolume {
			readableProgress = true
			break
		}
	}
	if !readableProgress {
		return ErrInvalidSourceSwitch
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	current, err := scanBookFromScanner(tx.QueryRow(`SELECT `+bookColumns+` FROM books WHERE id = ?`, bookID))
	if err != nil {
		return err
	}
	if current.StateVersion != expectedVersion {
		return ErrBookStateChanged
	}

	alternates := make([]AltSource, 0, len(current.AlternateSources))
	found := false
	for _, alternate := range current.AlternateSources {
		if alternate.SourceURL == target.SourceURL && alternate.BookURL == target.BookURL {
			found = true
			alternates = append(alternates, AltSource{SourceURL: current.SourceURL, BookURL: current.BookURL, SourceName: current.Origin})
			continue
		}
		alternates = append(alternates, alternate)
	}
	if !found {
		return ErrSourceNotAlternate
	}
	alternateJSON, err := json.Marshal(alternates)
	if err != nil {
		return fmt.Errorf("switch source: encode alternates: %w", err)
	}

	if _, err := tx.Exec(`UPDATE books SET source_url = ?, book_url = ?, toc_url = ?, origin = ?, variable_map = ?,
		last_chapter = ?, update_time = ?, word_count = ?, dur_chapter_index = ?, dur_chapter_pos = ?,
		total_chapter_num = ?, state_version = state_version + 1, alternate_sources = ?, updated_at = ? WHERE id = ?`,
		target.SourceURL, target.BookURL, target.TocURL, target.Origin, target.VariableMap,
		target.LastChapter, target.UpdateTime, target.WordCount, chapterIndex, position,
		len(chapters), string(alternateJSON), time.Now().UnixMilli(), bookID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chapters WHERE book_id = ?`, bookID); err != nil {
		return err
	}
	for _, chapter := range chapters {
		chapter.BookID = bookID
		chapter.ID = fmt.Sprintf("%s_%d", bookID, chapter.Index)
		if _, err := tx.Exec(`INSERT INTO chapters (id, book_id, idx, title, url, is_vip, is_volume, is_pay, base_url, tag, word_count, cached) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			chapter.ID, chapter.BookID, chapter.Index, chapter.Title, chapter.URL, boolToInt(chapter.IsVip), boolToInt(chapter.IsVolume), boolToInt(chapter.IsPay), chapter.BaseURL, chapter.Tag, chapter.WordCount, boolToInt(chapter.Cached)); err != nil {
			return err
		}
	}
	marks, err := tx.Query(`SELECT id, chapter_title FROM bookmarks WHERE book_id = ?`, bookID)
	if err != nil {
		return err
	}
	type bookmarkTitle struct{ id, title string }
	bookmarkTitles := make([]bookmarkTitle, 0)
	for marks.Next() {
		var mark bookmarkTitle
		if err := marks.Scan(&mark.id, &mark.title); err != nil {
			marks.Close()
			return err
		}
		bookmarkTitles = append(bookmarkTitles, mark)
	}
	if err := marks.Err(); err != nil {
		marks.Close()
		return err
	}
	if err := marks.Close(); err != nil {
		return err
	}
	for _, mark := range bookmarkTitles {
		chapter, matched := matchChapterTitle(chapters, normalizeChapterTitle(mark.title))
		if matched {
			if _, err := tx.Exec(`UPDATE bookmarks SET chapter_index = ?, chapter_title = ?, orphaned = 0 WHERE id = ?`, chapter.Index, chapter.Title, mark.id); err != nil {
				return err
			}
		} else if _, err := tx.Exec(`UPDATE bookmarks SET orphaned = 1 WHERE id = ?`, mark.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
