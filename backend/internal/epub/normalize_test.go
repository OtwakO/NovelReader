package epub

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func xhtml(body string) string {
	return `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh"><head><title>A chapter</title><style>body{color:red}</style></head><body>` + body + `</body></html>`
}

func nodesOfKind(root Node, kind string) []Node {
	var found []Node
	var walk func(Node)
	walk = func(node Node) {
		if node.Kind == kind {
			found = append(found, node)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	return found
}

func TestNormalizeSemanticSection(t *testing.T) {
	body := `<h2 id="source-heading">Title</h2><p>Before <em>emphasis</em> and <strong>bold</strong>.<br/><ruby>字<rt>zi</rt><rp>(zi)</rp></ruby><a epub:type="noteref" href="notes.xhtml#note">[1]</a><a href="https://example.invalid/read?q=1">Outside</a></p><blockquote><p>Quote</p></blockquote><ol start="-2"><li>First<ul><li>Nested</li></ul></li></ol><table><caption>Data</caption><tbody><tr><th>Heading</th><td colspan="2">Cell</td></tr></tbody></table><pre> one\n  two</pre><img src="../images/pic.png" alt="Illustration"/><aside epub:type="footnote" id="note">Note text</aside>`
	s, err := NormalizeSection(context.Background(), []byte(xhtml(body)), "Book/text/chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "A chapter" || s.Root.Language != "zh" || len(s.Diagnostics) != 0 {
		t.Fatalf("metadata: %+v", s.Diagnostics)
	}
	for _, kind := range []string{"heading", "paragraph", "emphasis", "strong", "break", "ruby", "rubyText", "rubyFallback", "quote", "orderedList", "unorderedList", "listItem", "table", "caption", "row", "headerCell", "cell", "preformatted", "image"} {
		if len(nodesOfKind(s.Root, kind)) == 0 {
			t.Fatalf("missing %s", kind)
		}
	}
	heading := nodesOfKind(s.Root, "heading")[0]
	if heading.Level != 2 || heading.ID == "source-heading" || heading.ID != s.Anchors["source-heading"] {
		t.Fatalf("anchor: %+v", heading)
	}
	links := nodesOfKind(s.Root, "link")
	if len(links) != 2 || links[0].Role != "noteref" || s.Links[links[0].Link] != (Reference{Path: "Book/text/notes.xhtml", Fragment: "note"}) || links[1].URL != "https://example.invalid/read?q=1" {
		t.Fatalf("links: %+v", links)
	}
	image := nodesOfKind(s.Root, "image")[0]
	if s.Images[image.Image].Path != "Book/images/pic.png" || image.Alt != "Illustration" {
		t.Fatal("image reference")
	}
	if *nodesOfKind(s.Root, "orderedList")[0].Start != -2 || nodesOfKind(s.Root, "cell")[0].ColSpan != 2 {
		t.Fatal("structure lost")
	}
	encoded, err := json.Marshal(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"source-heading", "notes.xhtml", "pic.png", "color:red"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("private/publisher detail in semantic tree: %s", private)
		}
	}
}

func TestNormalizeUnavailableAndAmbiguousContent(t *testing.T) {
	body := `<p id="dup" onclick="run()">Readable <script>run()</script><a href="javascript:run()">inactive</a><img src="https://example.invalid/pic.png" alt="missing picture"/></p><p id="dup">More</p><custom>Fallback text</custom><math xmlns="http://www.w3.org/1998/Math/MathML"><mi>x</mi></math>`
	s, err := NormalizeSection(context.Background(), []byte(xhtml(body)), "chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"active_content_removed", "link_unavailable", "image_unavailable", "ambiguous_anchor", "unsupported_content"} {
		if !slices.Contains(s.Diagnostics, code) {
			t.Fatalf("missing diagnostic %s", code)
		}
	}
	if id, exists := s.Anchors["dup"]; !exists || id != "" || len(s.Links) != 0 || len(s.Images) != 0 {
		t.Fatal("unsafe binding")
	}
	encoded, _ := json.Marshal(s.Root)
	if strings.Contains(string(encoded), "run()") || !strings.Contains(string(encoded), "Fallback text") || !strings.Contains(string(encoded), "missing picture") {
		t.Fatal("unsafe or missing fallback text")
	}
}

func TestNormalizeSVGWrapperAndWhitespace(t *testing.T) {
	body := "<p> left\n <em>middle</em> right\u00a0end</p><pre> one\n  two</pre>" + `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><title>Cover</title><image xlink:href="cover.png"/></svg>`
	s, err := NormalizeSection(context.Background(), []byte(xhtml(body)), "chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	p := nodesOfKind(s.Root, "paragraph")[0]
	if p.Children[0].Text != " left " || p.Children[2].Text != " right\u00a0end" || nodesOfKind(s.Root, "preformatted")[0].Children[0].Text != " one\n  two" {
		t.Fatal("text whitespace lost")
	}
	image := nodesOfKind(s.Root, "image")[0]
	if image.Alt != "Cover" || s.Images[image.Image].Path != "cover.png" || len(s.Diagnostics) != 0 {
		t.Fatal("SVG raster wrapper")
	}
}

func TestNormalizeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         error
	}{
		{"malformed", xhtml("<p>text"), ErrPackage},
		{"empty", xhtml("<script>run()</script>"), ErrUnsupported},
		{"base override", xhtml(`<p xml:base="../other/">text</p>`), ErrUnsupported},
		{"bytes", strings.Repeat("x", maxSectionBytes+1), ErrLimit},
		{"depth", xhtml(strings.Repeat("<div>", maxXMLDepth) + "text" + strings.Repeat("</div>", maxXMLDepth)), ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NormalizeSection(context.Background(), []byte(tc.source), "chapter.xhtml"); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NormalizeSection(ctx, []byte(xhtml("<p>Text</p>")), "chapter.xhtml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
