package epub

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func targetSection(t *testing.T, path, body string) Section {
	t.Helper()
	s, err := NormalizeSection(context.Background(), []byte(xhtml(body)), path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSectionTargetsAndNavigation(t *testing.T) {
	ctx := context.Background()
	main := targetSection(t, "main.xhtml", `<h1 id="private-start">Start</h1><h2 id="private-second">Second</h2><p><a href="#private-second">Here</a><a epub:type="noteref" href="notes.xhtml#private-note%2520one">Note</a><a href="notes.xhtml">Notes start</a><a href="notes.xhtml#missing">Broken</a><a href="notes.xhtml#private-duplicate">Ambiguous</a><a href="picture.png">Not a section</a><a href="https://example.invalid/">Outside</a></p>`)
	notes := targetSection(t, "notes.xhtml", `<aside epub:type="footnote" id="private-note%20one">Note text <a href="main.xhtml#private-start">Return</a></aside><p id="private-duplicate">One</p><p id="private-duplicate">Two</p>`)
	index := newTargetIndex()
	for _, entry := range []struct {
		path    string
		section *Section
	}{{"main.xhtml", &main}, {"notes.xhtml", &notes}} {
		if _, err := index.add(ctx, entry.path, entry.section.Anchors); err != nil {
			t.Fatal(err)
		}
	}
	// Forward and backward bindings use the same completed metadata index; note
	// registration does not add anything to a main reading sequence.
	if err := index.resolveSection(ctx, 0, &main); err != nil {
		t.Fatal(err)
	}
	if err := index.resolveSection(ctx, 1, &notes); err != nil {
		t.Fatal(err)
	}
	links := nodesOfKind(main.Root, "link")
	want := []SectionTarget{{0, main.Anchors["private-second"]}, {1, notes.Anchors["private-note%20one"]}, {1, ""}}
	for i, target := range want {
		if links[i].Target == nil || *links[i].Target != target || links[i].Link != "" || links[i].Unavailable {
			t.Fatalf("link %d: %+v", i, links[i])
		}
	}
	for _, node := range links[3:6] {
		if node.Target != nil || !node.Unavailable || node.Link != "" || len(node.Children) == 0 {
			t.Fatalf("unsafe/missing broken link: %+v", node)
		}
	}
	if links[6].URL != "https://example.invalid/" || links[6].Unavailable || len(main.Links) != 0 || !slices.Contains(main.Diagnostics, "link_unavailable") {
		t.Fatal("external action or binding cleanup")
	}
	back := nodesOfKind(notes.Root, "link")[0].Target
	if back == nil || *back != (SectionTarget{0, main.Anchors["private-start"]}) {
		t.Fatalf("note return: %+v", back)
	}

	nav, err := index.resolveNavigation(ctx, Navigation{Source: "nav", Entries: []NavigationEntry{{Label: "Part", Children: []NavigationEntry{
		{Label: "Second", Target: &Reference{Path: "main.xhtml", Fragment: "private-second"}},
		{Label: "First", Target: &Reference{Path: "main.xhtml", Fragment: "private-start"}},
		{Label: "Note", Target: &Reference{Path: "notes.xhtml", Fragment: "private-note%20one"}},
		{Label: "Missing", Target: &Reference{Path: "notes.xhtml", Fragment: "missing"}},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	group := nav.Entries[0]
	if group.Target != nil || group.Unavailable || len(group.Children) != 4 || nav.Source != "nav" {
		t.Fatalf("hierarchy: %+v", nav)
	}
	if *group.Children[0].Target != want[0] || group.Children[1].Target.Anchor != main.Anchors["private-start"] || *group.Children[2].Target != want[1] || !group.Children[3].Unavailable || group.Children[3].Target != nil {
		t.Fatalf("navigation targets: %+v", group.Children)
	}
	if !slices.Contains(nav.Diagnostics, "navigation_target_unavailable") {
		t.Fatal("missing navigation diagnostic")
	}
	encoded, err := json.Marshal([]any{main.Root, notes.Root, nav})
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"main.xhtml", "notes.xhtml", "picture.png", "private-"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("source reference leaked: %s", private)
		}
	}
}

func TestRepeatedResourceTargetsDoNotGuess(t *testing.T) {
	ctx := context.Background()
	index := newTargetIndex()
	anchors := map[string]string{"heading": "a1"}
	for range 2 {
		if _, err := index.add(ctx, "repeat.xhtml", anchors); err != nil {
			t.Fatal(err)
		}
	}
	// Ownership is copied: staging/reusing a caller's map cannot change targets.
	anchors["heading"] = "changed"
	ref := Reference{Path: "repeat.xhtml", Fragment: "heading"}
	for ordinal := range 2 {
		got := index.resolve(ordinal, ref)
		if got == nil || *got != (SectionTarget{ordinal, "a1"}) {
			t.Fatalf("self occurrence: %+v", got)
		}
	}
	if index.resolve(-1, ref) != nil {
		t.Fatal("ambiguous publication target chose an occurrence")
	}
	if _, err := index.add(ctx, "other.xhtml", nil); err != nil {
		t.Fatal(err)
	}
	if index.resolve(2, ref) != nil {
		t.Fatal("ambiguous cross-document target chose an occurrence")
	}
}

func TestTargetIndexLimitsAndCancellation(t *testing.T) {
	ctx := context.Background()
	// Seed near each budget, then consume its final slot through real registration
	// rather than allocating hundreds of thousands of fixture anchors.
	for _, setup := range []func(*targetIndex){
		func(index *targetIndex) { index.anchorCount = maxTargetAnchors - 1 },
		func(index *targetIndex) { index.bytes = maxTargetBytes - len("main.xhtml") - len("id") - len("a1") },
		func(index *targetIndex) { index.sections = make([]indexedTargetSection, maxTargetSections-1) },
	} {
		index := newTargetIndex()
		setup(index)
		if _, err := index.add(ctx, "main.xhtml", map[string]string{"id": "a1"}); err != nil {
			t.Fatalf("final available slot: %v", err)
		}
		before := len(index.sections)
		if _, err := index.add(ctx, "other.xhtml", map[string]string{"id": "a1"}); !errors.Is(err, ErrLimit) {
			t.Fatalf("index bounds: %v", err)
		}
		if len(index.sections) != before || len(index.paths) != 1 {
			t.Fatal("failed registration mutated index")
		}
	}
	index := newTargetIndex()
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := index.add(ctx, "main.xhtml", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled registration: %v", err)
	}
	section := Section{Root: Node{Kind: "group"}}
	if err := index.resolveSection(ctx, 0, &section); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled binding: %v", err)
	}
	if _, err := index.resolveNavigation(ctx, Navigation{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled empty navigation: %v", err)
	}
}
