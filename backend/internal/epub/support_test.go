package epub

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func inspectFixture(t *testing.T, entries []fixtureEntry) (Package, error) {
	t.Helper()
	data := fixtureArchive(t, entries)
	return Inspect(context.Background(), bytes.NewReader(data), int64(len(data)))
}

func TestSpineSupport(t *testing.T) {
	for _, tc := range []struct {
		name, replacement string
		want              error
	}{
		{"fallback", `<item id="chapter" href="drawing.svg" media-type="image/svg+xml" fallback="text"/><item id="text" href="text/first%20chapter.xhtml" media-type="application/xhtml+xml"/>`, nil},
		{"unsupported", `<item id="chapter" href="drawing.svg" media-type="image/svg+xml"/>`, ErrUnsupported},
		{"cycle", `<item id="chapter" href="drawing.svg" media-type="image/svg+xml" fallback="other"/><item id="other" href="other.svg" media-type="image/svg+xml" fallback="chapter"/>`, ErrPackage},
		{"missing fallback", `<item id="chapter" href="drawing.svg" media-type="image/svg+xml" fallback="absent"/>`, ErrPackage},
		{"missing required document", `<item id="chapter" href="missing.xhtml" media-type="application/xhtml+xml"/>`, ErrPackage},
		{"conflicting media aliases", `<item id="chapter" href="text/notes.xhtml" media-type="font/otf"/>`, ErrPackage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := packageFixture("3.0")
			entries[2].body = strings.Replace(entries[2].body, `<item id="chapter" href="text/first%20chapter.xhtml" media-type="application/xhtml+xml"/>`, tc.replacement, 1)
			p, err := inspectFixture(t, entries)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("got %v, want %v", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Spine[0].ID != "chapter" || p.Spine[0].ContentID != "text" || !p.Spine[0].Linear || p.Spine[1].ContentID != "notes" || p.Spine[1].Linear || len(p.Diagnostics) != 1 || p.Diagnostics[0] != "spine_fallback_used" {
				t.Fatalf("support: %+v", p)
			}
		})
	}
}

func TestFixedLayoutDeclarations(t *testing.T) {
	for _, kind := range []string{"global", "spine", "alias", "legacy-meta", "legacy-options"} {
		t.Run(kind, func(t *testing.T) {
			entries := packageFixture("3.0")
			opf := &entries[2].body
			switch kind {
			case "global":
				*opf = strings.Replace(*opf, "</metadata>", `<meta property="rendition:layout">pre-paginated</meta></metadata>`, 1)
			case "spine":
				*opf = strings.Replace(*opf, `idref="notes"`, `idref="notes" properties="rendition:layout-pre-paginated"`, 1)
			case "alias":
				*opf = strings.Replace(*opf, `<package `, `<package prefix="r: http://www.idpf.org/vocab/rendition/#" `, 1)
				*opf = strings.Replace(*opf, "</metadata>", `<meta property="r:layout">pre-paginated</meta></metadata>`, 1)
			case "legacy-meta":
				*opf = strings.Replace(*opf, "</metadata>", `<meta name="fixed-layout" content="true"/></metadata>`, 1)
			case "legacy-options":
				entries = append(entries, fixtureEntry{"META-INF/com.apple.ibooks.display-options.xml", `<display_options><platform name="*"><option name="fixed-layout">true</option></platform></display_options>`})
			}
			_, err := inspectFixture(t, entries)
			if !errors.Is(err, ErrFixedLayout) || !errors.Is(err, ErrUnsupported) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestEncryptionClassification(t *testing.T) {
	for _, tc := range []struct {
		name, algorithm, uri string
		want                 error
	}{
		{"IDPF font", "http://www.idpf.org/2008/embedding", "Book/font.otf", nil},
		{"Adobe font", "http://ns.adobe.com/pdf/enc#RC", "Book/font.otf", nil},
		{"encrypted text", "http://www.idpf.org/2008/embedding", "Book/text/first%20chapter.xhtml", ErrEncrypted},
		{"unknown encryption", "http://example.invalid/encryption", "Book/font.otf", ErrEncrypted},
		{"external reference", "http://www.idpf.org/2008/embedding", "https://example.invalid/font.otf", ErrPackage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := packageFixture("2.0")
			entries[2].body = strings.Replace(entries[2].body, "</manifest>", `<item id="font" href="font.otf" media-type="font/otf"/></manifest>`, 1)
			entries = append(entries, fixtureEntry{"Book/font.otf", "synthetic-unused-font"}, fixtureEntry{"META-INF/encryption.xml", `<encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="` + tc.algorithm + `"/><CipherData><CipherReference URI="` + tc.uri + `"/></CipherData></EncryptedData></encryption>`})
			_, err := inspectFixture(t, entries)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestConsistentResourceAliases(t *testing.T) {
	entries := packageFixture("3.0")
	entries[2].body = strings.Replace(entries[2].body, "</manifest>", `<item id="alias" href="text/first%20chapter.xhtml" media-type="application/xhtml+xml"/></manifest>`, 1)
	p, err := inspectFixture(t, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 5 || p.Spine[0].ContentID != "chapter" {
		t.Fatalf("aliases changed identity: %+v", p.Spine)
	}
}
