// Package epubstore owns storage-side EPUB preparation and file lifecycle.
package epubstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/otwako/novelreader/internal/epub"
)

const (
	// Final output budgets are independent of parser scratch and compressed input.
	maxSectionBytes         int64 = 64 << 20
	maxSectionTotalBytes    int64 = 2 << 30
	maxDerivativeBytes      int64 = 16 << 20
	maxDerivativeTotalBytes int64 = 1 << 30
)

// StagedPreparation owns disposable output, not a ready receipt or publication.
// The work root must remain open until Close. The caller must Close on success
// too, after consuming/moving output under its durable finalization protocol.
// Directory entries still require durable installation by that protocol.
// This object is single-owner; do not close it while consuming its files.
type StagedPreparation struct {
	Preparation epub.Preparation
	// SectionSpans contains only the byte index; no section trees are retained.
	SectionSpans []SectionSpan
	Resources    []PreparedResource
	work         *os.Root
	directory    string
}

func (s *StagedPreparation) Directory() string { return s.directory }

// Close discards only this attempt. It is safe after output has been moved and
// safe to retry after a cleanup error. No original archive belongs to this tree.
func (s *StagedPreparation) Close() error { return s.work.RemoveAll(s.directory) }

// Stage prepares an unchanged caller-owned original outside the reader mutation
// gate. work is a private disposable-work root, not the managed publication root.
// Every output path is generated here, never derived from an archive member.
// Success transfers cleanup ownership to the caller; failure returns no result
// and reports cleanup failures alongside the preparation error.
func Stage(ctx context.Context, work *os.Root, original io.ReaderAt, size int64, mode epub.ImageMode) (result *StagedPreparation, err error) {
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	s := &StagedPreparation{work: work, directory: "epub-" + rand.Text()}
	if err = work.Mkdir(s.directory, 0700); err != nil {
		return nil, fmt.Errorf("epubstore: create attempt: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.Close())
		}
	}()
	root, err := work.OpenRoot(s.directory)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, root.Close())
		if err != nil {
			result = nil
		}
	}()
	if err = root.Mkdir("images", 0700); err != nil {
		return nil, err
	}
	stream, err := root.OpenFile(sectionStreamFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, stream.Close())
		if err != nil {
			result = nil
		}
	}()
	scratch, err := root.OpenFile("scratch", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	// Close scratch before cleanup even if preparation or an output write fails.
	resources := newResourceIndex()
	var sectionBytes int64
	prepared, prepareErr := epub.Prepare(ctx, original, size, scratch, func(section epub.PreparedSection) error {
		span, err := appendSection(ctx, stream, section, sectionBytes)
		if err != nil {
			return err
		}
		for _, image := range section.Images {
			resources.add(image)
		}
		s.SectionSpans = append(s.SectionSpans, span)
		sectionBytes += span.Length
		return nil
	}, epub.ImageOptions{
		Mode: mode, MaxDerivativeBytes: maxDerivativeBytes, MaxTotalDerivativeBytes: maxDerivativeTotalBytes,
		Emit: func(ctx context.Context, _ epub.Reference, _ epub.ImageInfo, data []byte) (string, error) {
			id := rand.Text()
			err := writeDerivative(ctx, root, "images/"+id+".webp", data)
			if err == nil {
				resources.sizes[id] = int64(len(data))
			}
			return id, err
		},
	})
	if err = errors.Join(prepareErr, scratch.Close()); err != nil {
		return nil, err
	}
	if err = stream.Sync(); err != nil {
		return nil, err
	}
	if err = root.Remove("scratch"); err != nil {
		return nil, err
	}
	if prepared.Cover != nil {
		resources.add(*prepared.Cover)
	}
	for _, resource := range resources.byPath {
		s.Resources = append(s.Resources, resource)
	}
	s.Preparation = prepared
	return s, nil
}
