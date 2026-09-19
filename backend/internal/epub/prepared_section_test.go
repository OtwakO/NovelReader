package epub

import (
	"context"
	"errors"
	"testing"
)

func preparedSectionFixture() PreparedSection {
	return PreparedSection{Root: Node{Kind: "group", ID: "a1", Children: []Node{
		{Kind: "text", Text: "Readable text"},
		{Kind: "link", Target: &SectionTarget{Section: 0, Anchor: "a1"}},
		{Kind: "image", Image: "r1", Width: 2, Height: 3},
	}}, Images: map[string]PreparedImage{"r1": {Reference: Reference{Path: "image.png"}, Info: ImageInfo{MediaType: "image/png", Width: 2, Height: 3}}}}
}

func TestPreparedSectionSemantics(t *testing.T) {
	if err := NewPreparedPublicationCheck().Add(t.Context(), preparedSectionFixture()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*PreparedSection)
	}{
		{"unknown kind", func(s *PreparedSection) { s.Root.Kind = "script" }},
		{"duplicate anchor", func(s *PreparedSection) { s.Root.Children[0].ID = "a1" }},
		{"unresolved link", func(s *PreparedSection) { s.Root.Children[1].Link = "l1" }},
		{"two link actions", func(s *PreparedSection) { s.Root.Children[1].URL = "https://example.invalid" }},
		{"unsafe URL", func(s *PreparedSection) { s.Root.Children[1] = Node{Kind: "link", URL: "javascript:alert(1)"} }},
		{"missing image", func(s *PreparedSection) { delete(s.Images, "r1") }},
		{"false image dimensions", func(s *PreparedSection) { s.Root.Children[2].Width++ }},
		{"false placeholder", func(s *PreparedSection) { s.CoverPlaceholder = true }},
		{"unreadable prose", func(s *PreparedSection) { s.Root = Node{Kind: "group"} }},
		{"invalid heading", func(s *PreparedSection) { s.Root.Kind = "heading"; s.Root.Level = 7 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := preparedSectionFixture()
			tc.change(&s)
			if err := NewPreparedPublicationCheck().Add(t.Context(), s); err == nil {
				t.Fatal("invalid semantics accepted")
			}
		})
	}
	placeholder := PreparedSection{CoverPlaceholder: true, Root: Node{Kind: "group", Children: []Node{{Kind: "unsupported"}}}}
	if err := NewPreparedPublicationCheck().Add(t.Context(), placeholder); err != nil {
		t.Fatal("valid unavailable cover", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := NewPreparedPublicationCheck().Add(ctx, placeholder); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPreparedSectionAcceptsNormalizedSemantics(t *testing.T) {
	data := []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body dir="rtl" xml:lang="ar"><h2 id="title">Title</h2><aside epub:type="footnote"><p><ruby>字<rt>zi</rt></ruby><strong>bold</strong></p></aside><ol start="-2"><li>item</li></ol><table><tr><td colspan="2" rowspan="3">cell</td></tr></table><a href="https://example.invalid/path?q=1#note">external</a><a href="#title">local</a><a href="#missing">unavailable</a><svg xmlns="http://www.w3.org/2000/svg"><text>fallback</text></svg></body></html>`)
	normalized, err := NormalizeSection(t.Context(), data, "chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	targets := newTargetIndex()
	if _, err = targets.add(t.Context(), "chapter.xhtml", normalized.Anchors); err != nil {
		t.Fatal(err)
	}
	if err = targets.resolveSection(t.Context(), 0, &normalized); err != nil {
		t.Fatal(err)
	}
	if err = NewPreparedPublicationCheck().Add(t.Context(), PreparedSection{Root: normalized.Root}); err != nil {
		t.Fatal(err)
	}
	// Bounds apply even to structurally valid decoded trees.
	root := Node{Kind: "text", Text: "deep"}
	for i := 0; i < maxXMLDepth; i++ {
		root = Node{Kind: "group", Children: []Node{root}}
	}
	if err = NewPreparedPublicationCheck().Add(t.Context(), PreparedSection{Root: root}); !errors.Is(err, ErrLimit) {
		t.Fatal("depth", err)
	}
}
