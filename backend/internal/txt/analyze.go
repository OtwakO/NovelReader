package txt

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
)

// Analyze reads an original from byte zero without modifying or retaining its
// contents. Automatic candidates share one bounded decoding pass. Any failure
// returns no partial index; persistence, admission and job lifetime are callers' work.
func Analyze(ctx context.Context, original io.Reader, options Options) (Analysis, error) {
	if err := ctx.Err(); err != nil {
		return Analysis{}, err
	}
	rule, err := options.headingRule()
	if err != nil {
		return Analysis{}, err
	}
	reader := bufio.NewReaderSize(original, encodingSampleBytes)
	enc, start, err := resolveEncoding(reader, options.Encoding)
	if err != nil {
		return Analysis{}, err
	}
	presets := []Preset{options.Preset}
	if options.Preset == "" {
		presets = []Preset{ChineseChapters, EnglishChapters}
	}
	builders := make([]indexBuilder, len(presets))
	for i, preset := range presets {
		selectedRule := rule
		if options.Preset == "" {
			selectedRule = headingRules[preset]
		}
		builders[i] = indexBuilder{preset: preset, rule: selectedRule, start: start, title: "Introduction", part: 1}
	}
	lines := lineReader{decoder: newDecoder(ctx, reader, enc, start), offset: start}
	readable := false
	for {
		line, err := lines.next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// A valid sample is only a provisional choice. If later bytes
			// disprove BOM-less automatic UTF-8, request the encoding again.
			if options.Encoding == "" && start == 0 && errors.Is(err, errInvalidUTF8) {
				return Analysis{}, fmt.Errorf("%w: %v", ErrEncodingRequired, err)
			}
			return Analysis{}, err
		}
		readable = readable || len(bytes.TrimSpace(line.text)) != 0
		for i := range builders {
			if err := builders[i].append(line); err != nil {
				return Analysis{}, err
			}
		}
	}
	if !readable {
		return Analysis{}, fmt.Errorf("txt: no readable text")
	}
	best, matching := 0, 0
	for i := range builders {
		if err := builders[i].emit(lines.offset, false); err != nil {
			return Analysis{}, err
		}
		if builders[i].headings > 0 {
			matching++
		}
		if builders[i].headings > builders[best].headings {
			best = i
		}
	}
	selected := builders[best]
	result := Analysis{Encoding: enc, Preset: selected.preset, ParserVersion: parserVersion, Sections: selected.sections}
	switch {
	case selected.headings == 0:
		result.Preset = GeneratedSections
		for i := range result.Sections {
			result.Sections[i].Title = fmt.Sprintf("Section %d", i+1)
			result.Sections[i].Generated = true
		}
		if options.Preset != GeneratedSections {
			result.ReviewReasons = append(result.ReviewReasons, NoHeadings)
		}
	case selected.headings == 1:
		result.ReviewReasons = append(result.ReviewReasons, FewHeadings)
	}
	if matching > 1 {
		result.ReviewReasons = append(result.ReviewReasons, AmbiguousHeadings)
	}
	if err := ctx.Err(); err != nil {
		return Analysis{}, err
	}
	return result, nil
}

// Keep offsets/counts, not section bodies. A remembered paragraph boundary lets
// us prefer paragraphs once the hard bound is reached without splitting normal
// chapters eagerly just because they passed the smaller target size.
type indexBuilder struct {
	preset        Preset
	rule          *regexp.Regexp
	sections      []Section
	headings      int
	start         int64
	title         string
	part, size    int
	hasText       bool
	paragraphEnd  int64
	paragraphSize int
}

func (b *indexBuilder) append(line textLine) error {
	trimmed := bytes.TrimSpace(line.text)
	if b.rule != nil && line.complete && !line.continued && len(trimmed) > 0 && len(trimmed) <= maxHeadingBytes && matchesWholeHeading(b.rule, trimmed) {
		if b.hasText {
			if err := b.emit(line.start, false); err != nil {
				return err
			}
		}
		b.title, b.part = string(trimmed), 1
		b.headings++
	}
	if b.size+len(line.text) > maxSectionBytes || line.end-b.start > maxSectionSourceBytes {
		if b.paragraphSize >= targetSectionBytes {
			remaining := b.size - b.paragraphSize
			if err := b.emit(b.paragraphEnd, true); err != nil {
				return err
			}
			b.size, b.hasText = remaining, remaining > 0
		}
		if b.size+len(line.text) > maxSectionBytes || line.end-b.start > maxSectionSourceBytes {
			if err := b.emit(line.start, true); err != nil {
				return err
			}
		}
	}
	b.size += len(line.text)
	b.hasText = b.hasText || len(trimmed) != 0
	if len(trimmed) == 0 && line.complete {
		b.paragraphEnd, b.paragraphSize = line.end, b.size
	}
	return nil
}

func (b *indexBuilder) emit(end int64, generated bool) error {
	if end == b.start {
		return nil
	}
	if len(b.sections) >= maxSections {
		return fmt.Errorf("txt: index exceeds %d sections", maxSections)
	}
	title := b.title
	if generated || b.part > 1 {
		title = fmt.Sprintf("%s (part %d)", title, b.part)
	}
	b.sections = append(b.sections, Section{Title: title, Start: b.start, End: end, Generated: generated || b.part > 1})
	b.start, b.size, b.hasText = end, 0, false
	b.paragraphEnd, b.paragraphSize = 0, 0
	b.part++
	return nil
}
