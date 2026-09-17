package epub

import (
	"context"
	"fmt"
	"io"
	"slices"
)

// Preparation is internal import evidence, not publication authority or the
// version-2 HTTP contract. Ordinals index Sections; only Main sections belong to
// ordinary reading order. Diagnostics must be reviewed before publication.
type Preparation struct {
	Title       string
	Authors     []string
	Language    string
	Sections    []PreparedSectionInfo
	Navigation  ResolvedNavigation
	Diagnostics []string
}

type PreparedSectionInfo struct {
	Title            string
	Main             bool
	CoverPlaceholder bool
}

// Images is private storage evidence. Only Root is a presentation candidate;
// image keys still require revision-qualified, authorized resource routes.
type PreparedSection struct {
	Ordinal     int
	Root        Node
	Images      map[string]PreparedImage
	Diagnostics []string
}

type PreparedImage struct {
	Reference Reference
	Info      ImageInfo
}

// Prepare reuses one archive index, stages one normalized section at a time,
// then emits bound sections once forward/auxiliary targets are known. The caller
// owns an unchanged original, empty disposable scratch, and a disposable output
// attempt. It must discard ALL output on any error (including an emit error),
// and close/remove scratch on both success and failure. This function never
// chooses filesystem paths, extracts archive entries or publishes library state.
func Prepare(ctx context.Context, original io.ReaderAt, size int64, scratch io.ReadWriteSeeker, emit func(PreparedSection) error) (Preparation, error) {
	if err := ctx.Err(); err != nil {
		return Preparation{}, err
	}
	stage, err := newPreparationStage(scratch)
	if err != nil {
		return Preparation{}, err
	}
	a, err := openArchive(ctx, original, size)
	if err != nil {
		return Preparation{}, err
	}
	p, err := inspectArchive(ctx, a)
	if err != nil {
		return Preparation{}, err
	}
	work := newPreparationWork(a, p, stage)
	if err := work.normalize(ctx); err != nil {
		return Preparation{}, err
	}
	out := work.result
	out.Navigation, err = work.targets.resolveNavigation(ctx, p.Navigation)
	if err != nil {
		return Preparation{}, err
	}
	if len(out.Navigation.Entries) == 0 {
		out.Navigation.Source = "spine"
		for ordinal, section := range out.Sections {
			if section.Main {
				label := section.Title
				if label == "" {
					label = fmt.Sprintf("Section %d", ordinal+1)
				}
				out.Navigation.Entries = append(out.Navigation.Entries, ResolvedNavigationEntry{Label: label, Target: &SectionTarget{Section: ordinal}})
			}
		}
	}
	for _, code := range out.Navigation.Diagnostics {
		out.warn(code)
	}
	for ordinal := range out.Sections {
		section, err := stage.read(ctx, ordinal)
		if err != nil {
			return Preparation{}, err
		}
		if err := work.targets.resolveSection(ctx, ordinal, &section); err != nil {
			return Preparation{}, err
		}
		images := make(map[string]PreparedImage, len(section.Images))
		for key, ref := range section.Images {
			images[key] = PreparedImage{Reference: ref, Info: work.images.cache[ref.Path].info}
		}
		for _, code := range section.Diagnostics {
			out.warn(code)
		}
		if err := emit(PreparedSection{Ordinal: ordinal, Root: section.Root, Images: images, Diagnostics: section.Diagnostics}); err != nil {
			return Preparation{}, fmt.Errorf("epub: emit prepared section: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return Preparation{}, err
	}
	return out, nil
}

func (p *Preparation) warn(code string) {
	if !slices.Contains(p.Diagnostics, code) {
		p.Diagnostics = append(p.Diagnostics, code)
	}
}
