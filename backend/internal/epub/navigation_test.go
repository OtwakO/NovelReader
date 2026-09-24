package epub

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func navigationFixture(version, body string) []fixtureEntry {
	entries := packageFixture(version)
	name := "Book/nav/toc.xhtml"
	if version == "2.0" {
		name = "Book/nav/toc.ncx"
	}
	return append(entries, fixtureEntry{name, body})
}

func navDocument(body string) string {
	return `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><nav epub:type="toc"><ol>` + body + `</ol></nav></body></html>`
}

func TestNavigationHierarchy(t *testing.T) {
	for _, version := range []string{"2.0", "3.0"} {
		t.Run(version, func(t *testing.T) {
			body := navDocument(`<li><a href="../text/notes.xhtml#note">Notes</a><ol><li><a href="../text/first%20chapter.xhtml#part%202">Part <em>two</em><br/>end</a></li><li><a href="../text/first%20chapter.xhtml#start">Start</a></li></ol></li>`)
			if version == "2.0" {
				body = `<!DOCTYPE ncx SYSTEM "https://example.invalid/never-fetch.dtd"><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><navMap><navPoint playOrder="9"><navLabel><text>Notes</text></navLabel><content src="../text/notes.xhtml#note"/><navPoint playOrder="1"><navLabel><text>Part two end</text></navLabel><content src="../text/first%20chapter.xhtml#part%202"/></navPoint><navPoint><navLabel><text>Start</text></navLabel><content src="../text/first%20chapter.xhtml#start"/></navPoint></navPoint></navMap></ncx>`
			}
			entries := navigationFixture(version, body)
			// EPUB 3 uses the declared nav, EPUB 2 the spine's NCX; never
			// merge their different hierarchies or parse the other document.
			other := "Book/nav/toc.ncx"
			if version == "2.0" {
				other = "Book/nav/toc.xhtml"
			}
			entries = append(entries, fixtureEntry{other, "<malformed"})
			data := fixtureArchive(t, entries)
			p, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			n := p.Navigation
			if len(n.Diagnostics) != 0 || len(n.Entries) != 1 || len(n.Entries[0].Children) != 2 {
				t.Fatalf("navigation: %+v", n)
			}
			entry := n.Entries[0]
			if entry.Target.Path != "Book/text/notes.xhtml" || entry.Target.Fragment != "note" || entry.Children[0].Label != "Part two end" || entry.Children[0].Target.Fragment != "part 2" || entry.Children[1].Target.Fragment != "start" {
				t.Fatalf("targets: %+v", entry)
			}
			if p.Spine[0].ID != "chapter" || !p.Spine[0].Linear || p.Spine[1].Linear {
				t.Fatal("navigation changed spine")
			}
		})
	}
}

func TestNavigationHeadingsAndUnavailableTargets(t *testing.T) {
	body := navDocument(`<li><span>Part <img alt="one"/></span><ol><li><a href="../text/notes.xhtml#note">Note</a></li></ol></li>`)
	for _, href := range []string{"https://example.invalid/x", "../../../outside", "../missing.xhtml", "../package.opf", ""} {
		body = strings.Replace(body, `</ol></nav>`, `<li><a href="`+href+`">Unavailable</a></li></ol></nav>`, 1)
	}
	data := fixtureArchive(t, navigationFixture("3.0", body))
	p, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	n := p.Navigation
	if len(n.Entries) != 6 || n.Entries[0].Label != "Part one" || n.Entries[0].Target != nil || n.Entries[0].Unavailable || len(n.Diagnostics) != 1 || n.Diagnostics[0] != "navigation_target_unavailable" {
		t.Fatalf("navigation: %+v", n)
	}
	for _, entry := range n.Entries[1:] {
		if !entry.Unavailable || entry.Target != nil {
			t.Fatalf("unsafe target: %+v", entry)
		}
	}
}

func TestNavigationDiagnosticsAndFatalLimits(t *testing.T) {
	for _, tc := range []struct {
		name, body, diagnostic string
		fatal                  error
	}{
		{name: "malformed", body: `<html>`, diagnostic: "navigation_invalid"},
		{name: "wrong namespace", body: `<html><nav type="toc"><ol/></nav></html>`, diagnostic: "navigation_invalid"},
		{name: "empty", body: navDocument(""), diagnostic: "navigation_empty"},
		{name: "base override", body: navDocument(`<li xml:base="https://example.invalid/"><a href="x">X</a></li>`), diagnostic: "navigation_invalid"},
		{name: "duplicate toc", body: strings.Replace(navDocument(""), `</body>`, `<nav xmlns:e="http://www.idpf.org/2007/ops" e:type="toc"><ol/></nav></body>`, 1), diagnostic: "navigation_invalid"},
		{name: "depth", body: navDocument(strings.Repeat(`<div>`, maxXMLDepth) + strings.Repeat(`</div>`, maxXMLDepth)), fatal: ErrLimit},
		{name: "entries", body: navDocument(strings.Repeat(`<li><a href="../text/notes.xhtml">Note</a></li>`, maxNavigationEntries+1)), fatal: ErrLimit},
		{name: "bytes", body: strings.Repeat("x", int(maxMetadataBytes)+1), fatal: ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := fixtureArchive(t, navigationFixture("3.0", tc.body))
			p, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
			if tc.fatal != nil {
				if !errors.Is(err, tc.fatal) {
					t.Fatalf("got %v, want %v", err, tc.fatal)
				}
				return
			}
			if err != nil || !slices.Contains(p.Navigation.Diagnostics, tc.diagnostic) {
				t.Fatalf("error=%v navigation=%+v", err, p.Navigation)
			}
		})
	}
	data := fixtureArchive(t, packageFixture("3.0"))
	p, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil || !slices.Contains(p.Navigation.Diagnostics, "navigation_missing") {
		t.Fatalf("missing: %v %+v", err, p.Navigation)
	}
}

func TestNavigationCancellationAndReadBudget(t *testing.T) {
	data := fixtureArchive(t, navigationFixture("3.0", navDocument(`<li><a href="../text/notes.xhtml">Note</a></li>`)))
	a, err := openArchive(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := inspectNavigation(ctx, a, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	a.readBytes = maxMetadataReadBytes - 1
	if _, err := inspectNavigation(context.Background(), a, p); !errors.Is(err, ErrLimit) {
		t.Fatalf("budget: %v", err)
	}
}
