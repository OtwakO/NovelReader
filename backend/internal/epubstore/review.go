package epubstore

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/otwako/novelreader/internal/epub"
)

type Review struct {
	Generation           int64
	Title, Language      string
	Authors, Diagnostics []string
	ImageProcessing      epub.ImageProcessing
	TotalSections        int
	Headings             []SectionInfo
	HasMore              bool
	Sample               string
	SampleTruncated      bool
}

// Review projects saved evidence, never reparses the archive or returns private
// resource/anchor maps. Callers bound pagination. Only one section is decoded.
func (s *Store) Review(ctx context.Context, id string, generation int64, start, limit int) (Review, error) {
	current, err := s.GetImport(ctx, id)
	if err != nil {
		return Review{}, err
	}
	if current.State != Acquired || current.PreparationGeneration != generation || current.PreparationState != PreparationReady {
		return Review{}, ErrStateChanged
	}
	metadata, err := s.PreparedMetadata(ctx, id, generation)
	if err != nil {
		return Review{}, err
	}
	result := Review{Generation: generation, Title: metadata.Title, Authors: metadata.Authors, Language: metadata.Language,
		Diagnostics: metadata.Diagnostics, ImageProcessing: metadata.ImageProcessing, TotalSections: len(metadata.Sections), Headings: make([]SectionInfo, 0)}
	start = min(start, len(metadata.Sections))
	end := start + min(limit, len(metadata.Sections)-start)
	for ordinal := start; ordinal < end; ordinal++ {
		section := metadata.Sections[ordinal]
		result.Headings = append(result.Headings, SectionInfo{Index: ordinal, Title: section.Title, Main: section.Main})
	}
	result.HasMore = end < len(metadata.Sections)
	if len(result.Headings) > 0 {
		section, err := s.PreparedSection(ctx, id, generation, start)
		if err != nil {
			return Review{}, err
		}
		result.Sample, result.SampleTruncated, err = reviewSample(ctx, section.Root)
		if err != nil {
			return Review{}, err
		}
	}
	// A discard racing file reads must invalidate the response. Publication of this
	// immutable ready generation does not invalidate its review evidence.
	current, err = s.GetImport(ctx, id)
	if errors.Is(err, ErrNotFound) {
		err = ErrStateChanged
	}
	if err != nil {
		return Review{}, err
	}
	if current.State != Acquired || current.PreparationGeneration != generation {
		return Review{}, ErrStateChanged
	}
	return result, nil
}

const reviewSampleBytes = 4096

func reviewSample(ctx context.Context, root epub.Node) (string, bool, error) {
	var sample strings.Builder
	truncated := false
	appendText := func(text string) {
		end := min(len(text), reviewSampleBytes-sample.Len())
		for end > 0 && !utf8.ValidString(text[:end]) {
			end--
		}
		sample.WriteString(text[:end])
		truncated = truncated || end < len(text)
	}
	var visit func(epub.Node) error
	visit = func(node epub.Node) error {
		if truncated {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		appendText(node.Text)
		if node.Kind == "image" {
			appendText(node.Alt)
		}
		for _, child := range node.Children {
			if truncated {
				break
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		switch node.Kind {
		case "paragraph", "heading", "break", "listItem", "row", "figureCaption", "caption", "preformatted":
			appendText("\n")
		case "cell", "headerCell":
			appendText("\t")
		}
		return nil
	}
	err := visit(root)
	return strings.TrimSpace(sample.String()), truncated, err
}
