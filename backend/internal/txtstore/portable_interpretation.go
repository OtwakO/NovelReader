package txtstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/otwako/novelreader/internal/library"
	"github.com/otwako/novelreader/internal/txt"
)

// Validate saved structure, never rebuild it or execute stored patterns. The
// caller's snapshot includes both the library revision and interpretation roles.
func validatePortableInterpretations(ctx context.Context, tx *sql.Tx, value Receipt, generation int64, item library.Item) error {
	rows, err := tx.QueryContext(ctx, `SELECT generation,role,state,base_content_revision,encoding,preset,parser_version,review_reasons FROM txt_interpretations WHERE file_id=?`, value.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	active, candidate := 0, 0
	var activeGeneration, candidateGeneration int64
	for rows.Next() {
		var version, base int64
		var role, encoding, preset, reasons string
		var state State
		var parser int
		if err := rows.Scan(&version, &role, &state, &base, &encoding, &preset, &parser, &reasons); err != nil {
			return err
		}
		if version <= 0 || version > generation {
			return fmt.Errorf("invalid interpretation generation")
		}
		switch role {
		case "active":
			active++
			activeGeneration = version
			if (state != Ready && state != NeedsReview) || value.LibraryID == "" || base >= item.ContentRevision {
				return fmt.Errorf("active interpretation lacks a completed publication")
			}
		case "candidate":
			candidate++
			candidateGeneration = version
			if version != generation {
				return fmt.Errorf("candidate is not the latest allocated generation")
			}
			if (value.LibraryID == "" && base != 0) || (value.LibraryID != "" && base != item.ContentRevision) {
				return fmt.Errorf("candidate base revision differs from publication")
			}
		default:
			return fmt.Errorf("invalid interpretation role")
		}
		complete := state == Ready || state == NeedsReview
		if complete {
			// Options.Validate is deliberately not called with a stored preset/pattern:
			// future custom patterns are analysis input, not portable validation authority.
			if encoding == "" || (txt.Options{Encoding: txt.Encoding(encoding)}).Validate() != nil || parser <= 0 {
				return fmt.Errorf("invalid saved decoding metadata")
			}
			switch txt.Preset(preset) {
			case txt.ChineseChapters, txt.EnglishChapters, txt.GeneratedSections, "custom":
			default:
				return fmt.Errorf("invalid saved heading method")
			}
			var diagnostics []txt.ReviewReason
			if err := json.Unmarshal([]byte(reasons), &diagnostics); err != nil {
				return fmt.Errorf("invalid review diagnostics: %w", err)
			}
			if (state == NeedsReview) != (len(diagnostics) > 0) {
				return fmt.Errorf("review state differs from diagnostics")
			}
		} else if state != queued && state != Analyzing && state != AnalysisFailed {
			return fmt.Errorf("invalid candidate state")
		}
		count, err := validatePortableSections(ctx, tx, value, version, encoding, complete)
		if err != nil {
			return err
		}
		if role == "active" && count != item.TotalChapterNum {
			return fmt.Errorf("active section count differs from publication")
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if value.State != acquired {
		if active != 0 || candidate != 0 {
			return fmt.Errorf("unfinished acquisition/removal retains an interpretation")
		}
	} else if value.LibraryID == "" {
		if active != 0 || candidate != 1 {
			return fmt.Errorf("unpublished acquisition requires exactly one candidate")
		}
	} else if active != 1 || candidate > 1 || (candidate == 1 && candidateGeneration <= activeGeneration) {
		return fmt.Errorf("publication requires one active interpretation and at most one candidate")
	}
	return nil
}

func validatePortableSections(ctx context.Context, tx *sql.Tx, value Receipt, generation int64, encoding string, complete bool) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT idx,start_byte,end_byte FROM txt_sections WHERE file_id=? AND generation=? ORDER BY idx`, value.ID, generation)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	var previousEnd int64
	for rows.Next() {
		var index int
		var start, end int64
		if err := rows.Scan(&index, &start, &end); err != nil {
			return 0, err
		}
		if !complete {
			return 0, fmt.Errorf("unfinished candidate retains a partial index")
		}
		if count == 0 {
			validStart := start == 0
			switch txt.Encoding(encoding) {
			case txt.UTF8:
				validStart = start == 0 || start == 3
			case txt.UTF16LE, txt.UTF16BE:
				validStart = start == 2
			}
			if !validStart {
				return 0, fmt.Errorf("invalid first section offset")
			}
		} else if start != previousEnd {
			return 0, fmt.Errorf("section ranges do not partition the original")
		}
		if err := txt.ValidateSectionRange(start, end); err != nil {
			return 0, err
		}
		if index != count || end > value.Size {
			return 0, fmt.Errorf("invalid indexed section range")
		}
		previousEnd = end
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if complete && (count == 0 || previousEnd != value.Size) {
		return 0, fmt.Errorf("completed index does not cover the original")
	}
	return count, nil
}
