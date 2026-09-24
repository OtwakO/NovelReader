package reading

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

// Minimal synthetic publication for the preparation/reading boundary, independent
// of installed books, websites and future persistence or HTTP implementations.
func prepareReadingEPUB(t *testing.T) (epub.Preparation, []epub.PreparedSection) {
	t.Helper()
	xhtml := func(title, body string) string {
		return `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-Hant" dir="ltr"><head><title>` + title + `</title></head><body>` + body + `</body></html>`
	}
	main := `<div id="private-intro" style="publisher-style">Block introduction</div>` +
		`<h2>Heading</h2><p id="private-paragraph">Text <span lang="ja"><ruby>字<rt>じ</rt></ruby></span> <strong>strong</strong> <em>emphasis</em><br/>` +
		`<a epub:type="noteref" href="private-notes.xhtml#private-note">Note</a><a href="https://example.invalid/read">Outside</a><a href="private-notes.xhtml#missing">Unavailable</a></p>` +
		`<figure><img src="private-map.png" alt="A small map"/><figcaption>Map caption</figcaption></figure>` +
		`<ol start="0"><li>First</li></ol><table><tbody><tr><th rowspan="2">Heading</th><td colspan="2">Cell</td></tr></tbody></table>` +
		`<blockquote><p>Quote</p></blockquote><pre>preserved  space</pre><hr/><math xmlns="http://www.w3.org/1998/Math/MathML"><mi>fallback text</mi></math>`
	var raster bytes.Buffer
	if err := png.Encode(&raster, image.NewNRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	var original bytes.Buffer
	archive := zip.NewWriter(&original)
	for _, entry := range []struct{ name, data string }{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="private-book.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
		{"private-book.opf", `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata/><manifest><item id="main" href="private-main.xhtml" media-type="application/xhtml+xml"/><item id="notes" href="private-notes.xhtml" media-type="application/xhtml+xml"/><item id="map" href="private-map.png" media-type="image/png"/></manifest><spine><itemref idref="main"/><itemref idref="notes" linear="no"/></spine></package>`},
		{"private-main.xhtml", xhtml("Main section", main)},
		{"private-notes.xhtml", xhtml("Notes", `<aside epub:type="footnote" id="private-note"><p>Note body</p></aside>`)},
		{"private-map.png", raster.String()},
	} {
		file, err := archive.CreateHeader(&zip.FileHeader{Name: entry.name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	scratch, err := os.CreateTemp(t.TempDir(), "epub-scratch-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := scratch.Close(); err != nil {
			t.Error(err)
		}
	})
	var sections []epub.PreparedSection
	prepared, err := epub.Prepare(context.Background(), bytes.NewReader(original.Bytes()), int64(original.Len()), scratch, func(section epub.PreparedSection) error { sections = append(sections, section); return nil }, epub.ImageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return prepared, sections
}
