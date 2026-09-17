package epub

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
)

func preparationFixture(t *testing.T) []fixtureEntry {
	return []fixtureEntry{
		{"META-INF/container.xml", `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"book.opf", `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata/><manifest>
<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/><item id="main" href="main.xhtml" media-type="application/xhtml+xml"/>
<item id="sidebar" href="sidebar.xhtml" media-type="application/xhtml+xml"/><item id="notes" href="notes.xhtml" media-type="application/xhtml+xml"/>
<item id="more" href="more.xhtml" media-type="application/xhtml+xml"/><item id="broken" href="broken.xhtml" media-type="application/xhtml+xml"/>
<item id="unused" href="unused.xhtml" media-type="application/xhtml+xml"/><item id="pic" href="pic.png" media-type="image/png"/>
</manifest><spine><itemref idref="cover"/><itemref idref="sidebar" linear="no"/><itemref idref="main"/></spine></package>`},
		{"cover.xhtml", strings.Replace(xhtml(`<img src="missing.png"/>`), "<body>", `<body epub:type="cover">`, 1)},
		{"main.xhtml", xhtml(`<p id="start">Main prose <a epub:type="noteref" href="notes.xhtml#note">Note</a><a href="broken.xhtml">Broken</a><a href="absent.xhtml">Absent</a><a href="https://example.invalid/remote.xhtml">Outside</a></p>`)},
		{"sidebar.xhtml", xhtml(`<p>Sidebar<img src="pic.png"/></p>`)},
		{"pic.png", string(rasterFixture(t, "image/png"))},
		{"notes.xhtml", xhtml(`<aside epub:type="footnote" id="note">Note <a href="more.xhtml#more">More</a><a href="main.xhtml#start">Back</a></aside>`)},
		{"more.xhtml", xhtml(`<p id="more">More notes <a href="notes.xhtml#note">Cycle</a></p>`)},
		{"broken.xhtml", `<not valid xml`},
		{"unused.xhtml", `<not valid xml either`},
	}
}

func preparationScratch(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "epub-stage-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestPrepareStagesDiscoveryAndBinding(t *testing.T) {
	data := fixtureArchive(t, preparationFixture(t))
	var sections []PreparedSection
	result, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(section PreparedSection) error { sections = append(sections, section); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 5 || len(result.Sections) != 5 {
		t.Fatalf("sections: %d/%d", len(sections), len(result.Sections))
	}
	for i, section := range result.Sections {
		if section.Main != (i == 0 || i == 2) || sections[i].Ordinal != i || sections[i].CoverPlaceholder != section.CoverPlaceholder {
			t.Fatalf("main sequence/ordinal %d: %+v", i, section)
		}
	}
	if !result.Sections[0].CoverPlaceholder || !slices.Contains(result.Diagnostics, "cover_placeholder") || !slices.Contains(result.Diagnostics, "auxiliary_content_unavailable") {
		t.Fatalf("review diagnostics: %v", result.Diagnostics)
	}
	mainLinks := nodesOfKind(sections[2].Root, "link")
	if mainLinks[0].Target == nil || mainLinks[0].Target.Section != 3 || mainLinks[0].Role != "noteref" || !mainLinks[1].Unavailable || !mainLinks[2].Unavailable || mainLinks[3].URL == "" {
		t.Fatalf("bound main links: %+v", mainLinks)
	}
	if len(sections[1].Images) != 1 {
		t.Fatal("validated image binding lost in staging")
	}
	for _, image := range sections[1].Images {
		if image.Reference.Path != "pic.png" || image.Info != (ImageInfo{MediaType: "image/png", Width: 3, Height: 2}) {
			t.Fatalf("staged image evidence: %+v", image)
		}
	}
	back := nodesOfKind(sections[3].Root, "link")[1].Target
	if back == nil || back.Section != 2 || back.Anchor == "" {
		t.Fatalf("note return: %+v", back)
	}
	if result.Navigation.Source != "spine" || len(result.Navigation.Entries) != 2 || result.Navigation.Entries[1].Target.Section != 2 {
		t.Fatalf("labeled main-only fallback: %+v", result.Navigation)
	}
}

func TestPrepareAdmissionAndOutputFailure(t *testing.T) {
	for _, variant := range []string{"unmarked-cover", "unreadable-main", "no-readable-main", "invalid-spine", "auxiliary-limit"} {
		t.Run(variant, func(t *testing.T) {
			entries := preparationFixture(t)
			want := ErrUnsupported
			switch variant {
			case "unmarked-cover":
				entries[2].body = xhtml(`<img src="missing.png"/>`)
			case "unreadable-main":
				entries[3].body = xhtml(`<img src="missing.png"/>`)
			case "no-readable-main":
				entries[1].body = strings.Replace(entries[1].body, `<itemref idref="main"/>`, `<itemref idref="main" linear="no"/>`, 1)
				entries[3].body = xhtml(`<p>Only auxiliary prose</p>`)
			case "invalid-spine":
				entries[3].body = `<broken`
				want = ErrPackage
			case "auxiliary-limit":
				entries[6].body = xhtml(strings.Repeat("<div>", 140))
				want = ErrLimit
			}
			data := fixtureArchive(t, entries)
			emitted := 0
			result, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(PreparedSection) error { emitted++; return nil })
			if !errors.Is(err, want) || emitted != 0 || len(result.Sections) != 0 {
				t.Fatalf("admission: %v emitted=%d", err, emitted)
			}
		})
	}
	data := fixtureArchive(t, preparationFixture(t))
	failure := errors.New("output disk unavailable")
	result, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(PreparedSection) error { return failure })
	if !errors.Is(err, failure) || len(result.Sections) != 0 {
		t.Fatalf("partial result escaped: %+v %v", result, err)
	}
}

func TestPreparationScratchAndCancellation(t *testing.T) {
	data := fixtureArchive(t, preparationFixture(t))
	scratch := preparationScratch(t)
	if _, err := scratch.WriteString("preserve"); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), scratch, func(PreparedSection) error { t.Fatal("unexpected output"); return nil }); err == nil {
		t.Fatal("accepted occupied scratch")
	}
	stored, err := os.ReadFile(scratch.Name())
	if err != nil || string(stored) != "preserve" {
		t.Fatalf("scratch overwritten: %q %v", stored, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	emitted := 0
	result, err := Prepare(ctx, bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(PreparedSection) error { emitted++; cancel(); return nil })
	if !errors.Is(err, context.Canceled) || emitted != 1 || len(result.Sections) != 0 {
		t.Fatalf("canceled preparation: %v emitted=%d", err, emitted)
	}
	stage, err := newPreparationStage(preparationScratch(t))
	if err != nil {
		t.Fatal(err)
	}
	stage.bytes = maxStagedBytes - 1
	if err := stage.write(context.Background(), Section{}); !errors.Is(err, ErrLimit) || len(stage.spans) != 0 {
		t.Fatalf("stage budget: %v", err)
	}
	writer := &stageWriter{ctx: context.Background(), target: shortStageWriter{}, remaining: 100}
	if _, err := writer.Write([]byte("ab")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
}

type shortStageWriter struct{}

func (shortStageWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestExplicitCoverDeclarations(t *testing.T) {
	for _, declaration := range []string{"guide", "landmark", "fragment"} {
		t.Run(declaration, func(t *testing.T) {
			entries := preparationFixture(t)
			entries[2].body = xhtml(`<img src="missing.png"/>`)
			if declaration == "guide" {
				entries[1].body = strings.Replace(entries[1].body, `version="3.0"`, `version="2.0"`, 1)
				entries[1].body = strings.Replace(entries[1].body, `</package>`, `<guide><reference type="cover" href="cover.xhtml"/></guide></package>`, 1)
			} else {
				entries[1].body = strings.Replace(entries[1].body, `</manifest>`, `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest>`, 1)
				href := "cover.xhtml"
				if declaration == "fragment" {
					href += "#part"
				}
				entries = append(entries, fixtureEntry{"nav.xhtml", xhtml(`<nav epub:type="toc"><ol><li><a href="main.xhtml">Main</a></li></ol></nav><nav epub:type="landmarks"><ol><li><a epub:type="cover" href="` + href + `">Cover</a></li></ol></nav>`)})
			}
			data := fixtureArchive(t, entries)
			result, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(PreparedSection) error { return nil })
			if declaration == "fragment" {
				if !errors.Is(err, ErrUnsupported) {
					t.Fatalf("fragment exempted whole document: %v", err)
				}
			} else if err != nil || !result.Sections[0].CoverPlaceholder {
				t.Fatalf("declared cover: %v", err)
			}
		})
	}
}

func TestPreparationReferencesSurviveStagingWithoutReplacement(t *testing.T) {
	for _, href := range []string{"#%ff", "%ff.xhtml"} {
		if _, err := resolveReference("main.xhtml", href); !errors.Is(err, ErrReference) {
			t.Fatalf("non-UTF-8 reference accepted: %v", err)
		}
	}
	data := fixtureArchive(t, []fixtureEntry{{string([]byte{'x', 0xff}), "content"}})
	if _, err := openArchive(context.Background(), bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrArchive) {
		t.Fatalf("non-UTF-8 archive path: %v", err)
	}
}

func TestPrepareLargeSection(t *testing.T) {
	entries := preparationFixture(t)
	entries[3].body = xhtml(strings.Repeat("<p>"+strings.Repeat("Novel text ", 10)+"</p>", 10000))
	data := fixtureArchive(t, entries)
	count := 0
	result, err := Prepare(context.Background(), bytes.NewReader(data), int64(len(data)), preparationScratch(t), func(section PreparedSection) error {
		count++
		if section.Ordinal == 2 && len(section.Root.Children) != 10000 {
			t.Fatalf("large section truncated: %d", len(section.Root.Children))
		}
		return nil
	})
	if err != nil || count != 3 || len(result.Sections) != 3 {
		t.Fatalf("large section: %v count=%d", err, count)
	}
}
