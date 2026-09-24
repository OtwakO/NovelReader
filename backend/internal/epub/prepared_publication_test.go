package epub

import (
	"context"
	"errors"
	"testing"
)

func publicationCheckFixture() (Preparation, []PreparedSection) {
	metadata := Preparation{Sections: []PreparedSectionInfo{{Main: true}, {}}, ImageProcessing: ImageProcessing{Mode: OriginalImages}, Navigation: ResolvedNavigation{Source: "nav", Entries: []ResolvedNavigationEntry{{Children: []ResolvedNavigationEntry{{Target: &SectionTarget{Section: 1, Anchor: "a1"}}}}}}}
	sections := []PreparedSection{
		{Root: Node{Kind: "group", ID: "a1", Children: []Node{{Kind: "text", Text: "main"}, {Kind: "link", Target: &SectionTarget{Section: 1, Anchor: "a1"}}}}},
		{Ordinal: 1, Root: Node{Kind: "group", ID: "a1", Role: "footnote", Children: []Node{{Kind: "text", Text: "note"}, {Kind: "link", Target: &SectionTarget{Section: 0, Anchor: "a1"}}}}},
	}
	return metadata, sections
}

func TestPreparedPublicationTargets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Preparation, []PreparedSection)
		valid  bool
	}{
		{"forward auxiliary and return", func(*Preparation, []PreparedSection) {}, true},
		{"missing link anchor", func(_ *Preparation, s []PreparedSection) { s[0].Root.Children[1].Target.Anchor = "missing" }, false},
		{"missing section", func(_ *Preparation, s []PreparedSection) { s[0].Root.Children[1].Target.Section = 2 }, false},
		{"missing TOC anchor", func(p *Preparation, _ []PreparedSection) {
			p.Navigation.Entries[0].Children[0].Target.Anchor = "missing"
		}, false},
		{"summary placeholder mismatch", func(p *Preparation, _ []PreparedSection) { p.Sections[1].CoverPlaceholder = true }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata, sections := publicationCheckFixture()
			tc.change(&metadata, sections)
			check := NewPreparedPublicationCheck()
			for _, section := range sections {
				if err := check.Add(t.Context(), section); err != nil {
					t.Fatal(err)
				}
			}
			err := check.Finish(t.Context(), metadata)
			if (err == nil) != tc.valid {
				t.Fatal("publication validation", err)
			}
		})
	}
	metadata, sections := publicationCheckFixture()
	check := NewPreparedPublicationCheck()
	if err := check.Add(t.Context(), sections[1]); err == nil {
		t.Fatal("out-of-order section accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := check.Finish(ctx, metadata); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
