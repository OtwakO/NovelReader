package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"golang.org/x/text/encoding/unicode"
)

type fixtureEntry struct{ name, body string }

func fixtureArchive(t testing.TB, entries []fixtureEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, entry := range entries {
		f, err := w.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func packageFixture(version string) []fixtureEntry {
	return []fixtureEntry{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="Book/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"Book/package.opf", `<package xmlns="http://www.idpf.org/2007/opf" version="` + version + `"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title> A Novel </dc:title><dc:creator> Writer </dc:creator><dc:language>zh</dc:language></metadata><manifest><item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav/toc.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="chapter" href="text/first%20chapter.xhtml" media-type="application/xhtml+xml"/><item id="ncx" href="nav/toc.ncx" media-type="application/x-dtbncx+xml"/></manifest><spine toc="ncx"><itemref idref="chapter"/><itemref idref="notes" linear="no"/></spine></package>`},
		{"Book/text/notes.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="note">Note.</p></body></html>`},
		{"Book/text/first chapter.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>First chapter.</p></body></html>`},
	}
}

func TestInspectPackage(t *testing.T) {
	for _, version := range []string{"2.0", "3.0"} {
		t.Run(version, func(t *testing.T) {
			data := fixtureArchive(t, packageFixture(version))
			p, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if p.Title != "A Novel" || len(p.Authors) != 1 || p.Authors[0] != "Writer" || p.Language != "zh" || p.Version != version {
				t.Fatalf("metadata: %+v", p)
			}
			if len(p.Spine) != 2 || p.Spine[0].ID != "chapter" || !p.Spine[0].Linear || p.Spine[1].Linear {
				t.Fatalf("spine: %+v", p.Spine)
			}
			if p.Items[2].Reference.Path != "Book/text/first chapter.xhtml" || p.NCXID != "ncx" || p.Items[1].Properties[0] != "nav" {
				t.Fatalf("inventory: %+v", p.Items)
			}
		})
	}
}

func TestInspectXMLAndPackageFailures(t *testing.T) {
	for _, tc := range []struct {
		name, old, replacement string
		want                   error
	}{
		{"duplicate ID", `id="notes"`, `id="chapter"`, ErrPackage},
		{"missing spine ID", `idref="chapter"`, `idref="absent"`, ErrPackage},
		{"bad linear", `linear="no"`, `linear="maybe"`, ErrPackage},
		{"external resource", `text/notes.xhtml`, `https://example.invalid/notes.xhtml`, ErrReference},
		{"escaping resource", `text/notes.xhtml`, `../../outside.xhtml`, ErrReference},
		{"second XML root", `</package>`, `</package><other/>`, ErrPackage},
		{"unknown entity", `A Novel`, `&notDeclared;`, ErrPackage},
		{"deep XML", `A Novel`, strings.Repeat("<span>", maxXMLDepth) + "text" + strings.Repeat("</span>", maxXMLDepth), ErrLimit},
		{"token budget", `A Novel`, strings.Repeat("<span/>", maxXMLTokens/2), ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := packageFixture("3.0")
			entries[2].body = strings.Replace(entries[2].body, tc.old, tc.replacement, 1)
			data := fixtureArchive(t, entries)
			_, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestInspectEPUB2XMLDeclarations(t *testing.T) {
	entries := packageFixture("2.0")
	entries[2].body = `<?xml version="1.0" encoding="UTF-16"?><!DOCTYPE package SYSTEM "https://example.invalid/never-fetch.dtd">` + strings.Replace(entries[2].body, "A Novel", "甲&nbsp;乙", 1)
	encoded, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(entries[2].body))
	if err != nil {
		t.Fatal(err)
	}
	entries[2].body = string(encoded)
	data := fixtureArchive(t, entries)
	p, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "甲\u00a0乙" {
		t.Fatalf("title: %q", p.Title)
	}
}

// Explicit local compatibility check only. Installed but unreadable/malformed
// fixtures fail; absent opt-in skips. Never log original metadata or book text.
func openLocalEPUB(t *testing.T) (*os.File, int64) {
	t.Helper()
	name := os.Getenv("NOVELREADER_EPUB_FIXTURE")
	if name == "" {
		t.Skip("optional local EPUB: set NOVELREADER_EPUB_FIXTURE")
	}
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	return f, info.Size()
}

func TestLocalPackageInspection(t *testing.T) {
	f, size := openLocalEPUB(t)
	p, err := Inspect(context.Background(), f, size)
	if err != nil {
		t.Fatal(err)
	}
	var countEntries func([]NavigationEntry) int
	countEntries = func(entries []NavigationEntry) int {
		count := len(entries)
		for _, entry := range entries {
			count += countEntries(entry.Children)
		}
		return count
	}
	t.Logf("inventory/navigation only: compressed bytes=%d manifest items=%d spine items=%d navigation entries=%d navigation diagnostics=%v support diagnostics=%v", size, len(p.Items), len(p.Spine), countEntries(p.Navigation.Entries), p.Navigation.Diagnostics, p.Diagnostics)
}

func BenchmarkInspect(b *testing.B) {
	entries := packageFixture("3.0")
	// A large single-spine document must not be decoded during metadata work.
	entries[4].body = "<html><body>" + strings.Repeat("<p>Novel prose.</p>", 1<<20) + "</body></html>"
	data := fixtureArchive(b, entries)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := Inspect(context.Background(), bytes.NewReader(data), int64(len(data))); err != nil {
			b.Fatal(err)
		}
	}
}
