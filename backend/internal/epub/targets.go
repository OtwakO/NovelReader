package epub

import (
	"context"
	"strings"
)

const (
	maxTargetSections = maxSpineItems + maxManifestItems
	maxTargetAnchors  = 250000
	maxTargetBytes    = 32 << 20
)

// SectionTarget is preparation evidence, not yet the version-2 reading contract.
// Its ordinal belongs to one interpretation; the reading boundary must qualify
// it with that publication's revision. An empty Anchor means document start.
type SectionTarget struct {
	Section int    `json:"section"`
	Anchor  string `json:"anchor,omitempty"`
}

// targetIndex retains only bounded path/anchor metadata, never section trees.
// Register admitted sections in their assigned order, then resolve links from
// staged sections one at a time once all forward targets have been registered.
// Reading-order membership and discovery of auxiliary documents belong to the
// preparation orchestrator, not this lookup owner.
type targetIndex struct {
	sections    []indexedTargetSection
	paths       map[string][]int
	anchorCount int
	bytes       int
}

type indexedTargetSection struct {
	path    string
	anchors map[string]string
}

func newTargetIndex() *targetIndex {
	return &targetIndex{paths: make(map[string][]int)}
}

// add copies the small lookup data so the caller can release or stage its whole
// Section. Empty anchor values preserve ambiguity reported by normalization.
// On any error the index is unchanged.
func (t *targetIndex) add(ctx context.Context, path string, anchors map[string]string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(t.sections) >= maxTargetSections || len(anchors) > maxTargetAnchors-t.anchorCount {
		return 0, ErrLimit
	}
	size := len(path)
	for original, opaque := range anchors {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		size += len(original) + len(opaque)
		if size > maxTargetBytes-t.bytes {
			return 0, ErrLimit
		}
	}
	if size > maxTargetBytes-t.bytes {
		return 0, ErrLimit
	}
	copied := make(map[string]string, len(anchors))
	for original, opaque := range anchors {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		copied[strings.Clone(original)] = strings.Clone(opaque)
	}
	path = strings.Clone(path)
	ordinal := len(t.sections)
	t.sections = append(t.sections, indexedTargetSection{path: path, anchors: copied})
	t.paths[path] = append(t.paths[path], ordinal)
	t.anchorCount += len(anchors)
	t.bytes += size
	return ordinal, nil
}

// resolve never guesses an ordinal or substitutes document start for a missing
// fragment. Manifest aliases may put the same resource in multiple spine slots:
// a self-link stays in its current occurrence, but an ambiguous cross-document
// or TOC target is unavailable rather than jumping to an arbitrary occurrence.
// Pass -1 when there is no current section (publication-wide navigation).
func (t *targetIndex) resolve(from int, ref Reference) *SectionTarget {
	ordinal := -1
	if from >= 0 && from < len(t.sections) && t.sections[from].path == ref.Path {
		ordinal = from
	} else if candidates := t.paths[ref.Path]; len(candidates) == 1 {
		ordinal = candidates[0]
	}
	if ordinal < 0 {
		return nil
	}
	anchor := ""
	if ref.Fragment != "" {
		anchor = t.sections[ordinal].anchors[ref.Fragment]
		if anchor == "" {
			return nil
		}
	}
	return &SectionTarget{Section: ordinal, Anchor: anchor}
}
