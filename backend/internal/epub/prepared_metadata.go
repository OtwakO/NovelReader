package epub

import (
	"context"
	"errors"
)

var ErrPreparedMetadata = errors.New("epub: invalid preparation metadata")

// ValidatePreparedMetadata checks an already-decoded summary. Storage must still
// bound JSON decoding and match cover/counters to its resource registry. Section
// readability, placeholder agreement and anchor existence require the node walk.
func ValidatePreparedMetadata(ctx context.Context, p Preparation, mode ImageMode, sectionCount int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(p.Sections) == 0 || len(p.Sections) != sectionCount || len(p.Sections) > maxTargetSections {
		return ErrPreparedMetadata
	}
	mainCount, hasNonPlaceholderMain, titleBytes := 0, false, 0
	for _, section := range p.Sections {
		if err := ctx.Err(); err != nil {
			return err
		}
		titleBytes += len(section.Title)
		if titleBytes > maxPreparationTitleBytes {
			return ErrLimit
		}
		if section.Main {
			mainCount++
			hasNonPlaceholderMain = hasNonPlaceholderMain || !section.CoverPlaceholder
		}
	}
	if !hasNonPlaceholderMain || mainCount > maxSpineItems {
		return ErrPreparedMetadata
	}
	processing := p.ImageProcessing
	if processing.Mode != mode || processing.DerivativeCount < 0 || processing.DerivativeBytes < 0 {
		return ErrImagePolicy
	}
	switch mode {
	case OriginalImages:
		if processing.Profile != "" || processing.Backend != "" || processing.DerivativeCount != 0 || processing.DerivativeBytes != 0 {
			return ErrImagePolicy
		}
	case OptimizedImages:
		if processing.Profile != optimizedImageProfile {
			return ErrImagePolicy
		}
		if processing.DerivativeCount == 0 {
			if processing.Backend != "" || processing.DerivativeBytes != 0 {
				return ErrImagePolicy
			}
		} else if (processing.Backend != "native" && processing.Backend != "portable") || processing.DerivativeBytes == 0 {
			return ErrImagePolicy
		}
	default:
		return ErrImagePolicy
	}
	if p.Cover != nil {
		if err := ValidatePreparedImage(*p.Cover, mode); err != nil {
			return err
		}
	}
	nav := p.Navigation
	if len(nav.Entries) == 0 {
		return ErrPreparedMetadata
	}
	if nav.Source == "spine" {
		if len(nav.Entries) != mainCount {
			return ErrPreparedMetadata
		}
		entryIndex := 0
		for ordinal, section := range p.Sections {
			if !section.Main {
				continue
			}
			entry := nav.Entries[entryIndex]
			entryIndex++
			if entry.Label == "" || entry.Unavailable || len(entry.Children) != 0 || entry.Target == nil || *entry.Target != (SectionTarget{Section: ordinal}) {
				return ErrPreparedMetadata
			}
		}
	} else if nav.Source != "nav" && nav.Source != "ncx" {
		return ErrPreparedMetadata
	}
	count := 0
	var visit func([]ResolvedNavigationEntry, int) error
	visit = func(entries []ResolvedNavigationEntry, depth int) error {
		if len(entries) == 0 {
			return nil
		}
		if depth > maxXMLDepth {
			return ErrLimit
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			count++
			if count > maxNavigationEntries {
				return ErrLimit
			}
			if entry.Target != nil && (entry.Unavailable || entry.Target.Section < 0 || entry.Target.Section >= sectionCount) {
				return ErrPreparedMetadata
			}
			if err := visit(entry.Children, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(nav.Entries, 1)
}
