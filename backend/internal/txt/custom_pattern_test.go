package txt

import (
	"errors"
	"strings"
	"testing"
)

func TestCustomPatternMatchesWholeTrimmedHeadings(t *testing.T) {
	source := "An aside: part 8 - not a heading\n  PART 1 - Start  \r\nFirst paragraph.\n\nPart 2 - End\nLast paragraph.\n"
	result, err := Analyze(t.Context(), strings.NewReader(source), Options{Preset: CustomPattern, Pattern: `(?i)part ([0-9]+)|part ([0-9]+) - .*`})
	if err != nil {
		t.Fatal(err)
	}
	if result.Preset != CustomPattern || len(result.Sections) != 3 || len(result.ReviewReasons) != 0 {
		t.Fatalf("analysis: %+v", result)
	}
	if result.Sections[1].Title != "PART 1 - Start" || result.Sections[2].Title != "Part 2 - End" {
		t.Fatalf("captures replaced whole-line titles: %+v", result.Sections)
	}
	if result.Sections[1].Start != int64(strings.Index(source, "  PART 1")) || result.Sections[2].Start != int64(strings.Index(source, "Part 2")) {
		t.Fatalf("source boundaries: %+v", result.Sections)
	}
}

func TestCustomPatternNoMatchRetainsReviewFallback(t *testing.T) {
	source := "part 9 inside prose\n" + strings.Repeat("x", maxHeadingBytes+1) + "\n"
	result, err := Analyze(t.Context(), strings.NewReader(source), Options{Preset: CustomPattern, Pattern: `part [0-9]+|x+`})
	if err != nil {
		t.Fatal(err)
	}
	if result.Preset != GeneratedSections || len(result.ReviewReasons) != 1 || result.ReviewReasons[0] != NoHeadings {
		t.Fatalf("fallback: %+v", result)
	}
}

func TestCustomPatternValidation(t *testing.T) {
	for _, pattern := range []string{"", "(", "a)|(?:b", "a*", `(?=chapter)chapter`, `(chapter)\1`, strings.Repeat("中", MaxPatternBytes/3+1)} {
		if err := (Options{Preset: CustomPattern, Pattern: pattern}).Validate(); !errors.Is(err, ErrInvalidPattern) || !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("pattern %q: %v", pattern, err)
		}
	}
	if err := (Options{Pattern: "chapter"}).Validate(); !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("pattern without custom choice: %v", err)
	}
	if err := (Options{Preset: CustomPattern, Pattern: strings.Repeat("x", MaxPatternBytes)}).Validate(); err != nil {
		t.Fatalf("limit boundary: %v", err)
	}
}
