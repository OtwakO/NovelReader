package epub

import (
	"context"
	"errors"
	"fmt"
)

type sectionWork struct {
	path           string
	main, required bool
}

type preparationWork struct {
	archive        *archive
	packageInfo    Package
	stage          *preparationStage
	images         *imageResolver
	targets        *targetIndex
	items          map[string]Item
	seen           map[string]bool
	covers         map[string]bool
	coverPageImage coverPageImage
	queue          []sectionWork
	result         Preparation
}

func newPreparationWork(a *archive, p Package, stage *preparationStage) *preparationWork {
	w := &preparationWork{archive: a, packageInfo: p, stage: stage, images: newImageResolver(a, p.Items), targets: newTargetIndex(), items: map[string]Item{}, seen: map[string]bool{}, covers: map[string]bool{}, result: Preparation{Title: p.Title, Authors: p.Authors, Language: p.Language}}
	ids := make(map[string]Item, len(p.Items))
	for _, item := range p.Items {
		ids[item.ID] = item
		w.items[item.Reference.Path] = item
	}
	for _, spine := range p.Spine {
		path := ids[spine.ContentID].Reference.Path
		w.queue = append(w.queue, sectionWork{path: path, main: spine.Linear, required: true})
		w.seen[path] = true
	}
	for _, ref := range p.CoverPages {
		w.covers[ref.Path] = true
	}
	for _, ref := range p.Navigation.CoverPages {
		w.covers[ref.Path] = true
	}
	for _, code := range p.Diagnostics {
		w.result.warn(code)
	}
	return w
}

// Only existing, declared XHTML can enter the auxiliary queue. Marking it before
// traversal bounds discovery by manifest size and terminates cyclic note links.
// All spine slots were seeded first, preserving duplicates and linearity.
func (w *preparationWork) enqueue(ref Reference) {
	item, exists := w.items[ref.Path]
	if w.seen[ref.Path] || !exists || item.MediaType != "application/xhtml+xml" || w.archive.files[ref.Path] == nil {
		return
	}
	w.seen[ref.Path] = true
	w.queue = append(w.queue, sectionWork{path: ref.Path})
}

func (w *preparationWork) discoverNavigation(ctx context.Context, entries []NavigationEntry) error {
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Target != nil && !entry.Unavailable {
			w.enqueue(*entry.Target)
		}
		if err := w.discoverNavigation(ctx, entry.Children); err != nil {
			return err
		}
	}
	return nil
}

func (w *preparationWork) discoverLinks(ctx context.Context, node Node, refs map[string]Reference) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if node.Link != "" {
		w.enqueue(refs[node.Link])
	}
	for _, child := range node.Children {
		if err := w.discoverLinks(ctx, child, refs); err != nil {
			return err
		}
	}
	return nil
}

func (w *preparationWork) normalize(ctx context.Context) error {
	if err := w.discoverNavigation(ctx, w.packageInfo.Navigation.Entries); err != nil {
		return err
	}
	readableMain, titleBytes := false, 0
	for cursor := 0; cursor < len(w.queue); cursor++ {
		job := w.queue[cursor]
		data, err := w.archive.readSection(ctx, job.path)
		if err != nil {
			return err
		} // ZIP corruption, I/O and limits are never optional.
		section, err := normalizeSection(ctx, data, job.path)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if job.required || !optionalSectionError(err) {
				return err
			}
			if w.covers[job.path] {
				w.coverPageImage.unavailable = true
			}
			w.result.warn("auxiliary_content_unavailable")
			continue
		}
		if section.Cover || w.covers[job.path] {
			w.coverPageImage.observe(section, w.items)
		}
		readable, err := w.images.resolveSection(ctx, &section)
		if err != nil {
			return err
		}
		placeholder := !readable && job.required && (section.Cover || w.covers[job.path])
		if !readable && !placeholder {
			if job.required {
				return fmt.Errorf("%w: unreadable spine section", ErrUnsupported)
			}
			w.result.warn("auxiliary_content_unavailable")
			continue
		}
		if placeholder {
			w.result.warn("cover_placeholder")
			section.Diagnostics = append(section.Diagnostics, "cover_placeholder")
		}
		if job.main && readable {
			readableMain = true
		}
		titleBytes += len(section.Title)
		if titleBytes > maxPreparationTitleBytes {
			return ErrLimit
		}
		if _, err := w.targets.add(ctx, job.path, section.Anchors); err != nil {
			return err
		}
		if err := w.discoverLinks(ctx, section.Root, section.Links); err != nil {
			return err
		}
		section.Anchors = nil // The index owns the copied lookup; no need to stage it too.
		if err := w.stage.write(ctx, section); err != nil {
			return err
		}
		w.result.Sections = append(w.result.Sections, PreparedSectionInfo{Title: section.Title, Main: job.main, CoverPlaceholder: placeholder})
	}
	if !readableMain {
		return fmt.Errorf("%w: no readable main content", ErrUnsupported)
	}
	return ctx.Err()
}

// Only semantic failures of optional documents may become unavailable content.
// Limits and cancellation must survive even when wrapped as package errors.
func optionalSectionError(err error) bool {
	return !errors.Is(err, ErrLimit) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) &&
		(errors.Is(err, ErrPackage) || errors.Is(err, ErrUnsupported))
}
