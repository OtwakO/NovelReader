package epub

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"
)

func TestPrepareCoverSelection(t *testing.T) {
	for _, test := range []struct {
		name, version, propertiesA, propertiesB, metadata, extraItems, guide, page, want, diagnostic string
		brokenArt, pageInSpine                                                                       bool
	}{
		{name: "modern beats legacy", version: "3.0", propertiesA: "cover-image", metadata: `<meta name="cover" content="b"/>`, want: "art.bin"},
		{name: "legacy", version: "2.0", metadata: `<meta name="cover" content="a"/>`, want: "art.bin"},
		{name: "aliases", version: "3.0", propertiesA: "cover-image", extraItems: `<item id="alias" href="art.bin" media-type="image/png" properties="cover-image"/>`, want: "art.bin"},
		{name: "conflicting declarations", version: "3.0", propertiesA: "cover-image", propertiesB: "cover-image", guide: `<guide><reference type="cover" href="cover.xhtml"/></guide>`, page: `<img src="art.bin"/>`, diagnostic: "cover_image_ambiguous"},
		{name: "unresolved competing declaration", version: "2.0", metadata: `<meta name="cover" content="a"/><meta name="cover" content="unknown"/>`, diagnostic: "cover_image_ambiguous"},
		{name: "explicit page outside reading graph", version: "3.0", guide: `<guide><reference type="cover" href="cover.xhtml"/></guide>`, page: `<img src="art.bin"/><img src="art.bin"/><a href="unused.xhtml">Not reading content</a>`, want: "art.bin"},
		{name: "broken declaration then page", version: "3.0", propertiesA: "cover-image", brokenArt: true, guide: `<guide><reference type="cover" href="cover.xhtml"/></guide>`, page: `<img src="decoration.png"/>`, want: "decoration.png", diagnostic: "cover_image_invalid"},
		{name: "do not promote surviving decoration", version: "3.0", brokenArt: true, pageInSpine: true, guide: `<guide><reference type="cover" href="cover.xhtml"/></guide>`, page: `<img src="art.bin"/><img src="decoration.png"/>`, diagnostic: "cover_image_ambiguous"},
		{name: "unknown image cannot promote decoration", version: "3.0", guide: `<guide><reference type="cover" href="cover.xhtml"/></guide>`, page: `<img src="missing.png"/><img src="decoration.png"/>`, diagnostic: "cover_image_unavailable"},
		{name: "malformed optional cover page", version: "3.0", guide: `<guide><reference type="cover" href="cover.xhtml"/></guide>`, page: `<broken`, diagnostic: "cover_image_unavailable"},
		{name: "ordinary illustrations and filenames are not covers", version: "3.0", page: `<img src="art.bin"/>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			art := rasterFixture(t, "image/png")
			if test.brokenArt {
				art = []byte("not image bytes")
			}
			page := xhtml(test.page)
			if test.page == "<broken" {
				page = test.page
			}
			spine, expectedSections := `<itemref idref="main"/>`, 1
			if test.pageInSpine {
				spine += `<itemref idref="page"/>`
				expectedSections++
			}
			data := fixtureArchive(t, []fixtureEntry{
				{"META-INF/container.xml", `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
				{"book.opf", `<package xmlns="http://www.idpf.org/2007/opf" version="` + test.version + `"><metadata>` + test.metadata + `</metadata><manifest>
<item id="main" href="main.xhtml" media-type="application/xhtml+xml"/>
<item id="a" href="art.bin" media-type="image/png" properties="` + test.propertiesA + `"/>
<item id="b" href="decoration.png" media-type="image/png" properties="` + test.propertiesB + `"/>
<item id="page" href="cover.xhtml" media-type="application/xhtml+xml"/>
<item id="unused" href="unused.xhtml" media-type="application/xhtml+xml"/>` + test.extraItems + `</manifest><spine>` + spine + `</spine>` + test.guide + `</package>`},
				{"main.xhtml", xhtml(`<p>Main prose<img src="decoration.png"/></p>`)},
				{"cover.xhtml", page}, {"unused.xhtml", "broken but unreferenced"},
				{"art.bin", string(art)}, {"decoration.png", string(rasterFixture(t, "image/png"))},
			})
			emitted := 0
			result, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(PreparedSection) error { emitted++; return nil })
			if err != nil {
				t.Fatal(err)
			}
			if emitted != expectedSections || len(result.Sections) != expectedSections || !result.Sections[0].Main {
				t.Fatal("cover inspection changed the reading graph")
			}
			if test.want == "" {
				if result.Cover != nil {
					t.Fatalf("guessed a cover: %+v", result.Cover)
				}
			} else if result.Cover == nil || result.Cover.Reference.Path != test.want || result.Cover.Info != (ImageInfo{MediaType: "image/png", Width: 3, Height: 2}) {
				t.Fatalf("cover evidence: %+v", result.Cover)
			}
			if test.diagnostic != "" && !slices.Contains(result.Diagnostics, test.diagnostic) {
				t.Fatalf("diagnostics: %v", result.Diagnostics)
			}
		})
	}
}

func TestCoverSelectionPreservesFatalErrors(t *testing.T) {
	ctx := context.Background()
	data := fixtureArchive(t, []fixtureEntry{{"cover.png", string(rasterFixture(t, "image/png"))}})
	a, err := openArchive(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	ref := Reference{Path: "cover.png"}
	work := newPreparationWork(a, Package{CoverImage: &ref, Items: []Item{{ID: "art", Reference: ref, MediaType: "image/png"}}}, nil)
	a.readBytes = int64(maxExpandedBytes)
	if image, err := work.selectCover(ctx); image != nil || !errors.Is(err, ErrLimit) {
		t.Fatalf("cover limit softened: %v", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if image, err := work.selectCover(ctx); image != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cover cancellation softened: %v", err)
	}
}
