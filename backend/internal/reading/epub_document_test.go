package reading

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/epub"
)

func TestPreparedEPUBReadingDocument(t *testing.T) {
	prepared, sections := prepareReadingEPUB(t)
	if len(sections) != 2 || !prepared.Sections[0].Main || prepared.Sections[1].Main {
		t.Fatal("main/note preparation membership")
	}
	var issued []string
	content, err := epubContent(context.Background(), 9, prepared.Sections[0].Title, sections[0], func(key string) string {
		issued = append(issued, key)
		return "/api/books/proof/chapters/0/resources/" + key + "?contentRevision=9"
	})
	if err != nil {
		t.Fatal(err)
	}
	if content.Version != 2 || content.ContentRevision != 9 || content.Document.Kind != "prose" || content.Document.Title != "Main section" {
		t.Fatalf("envelope: %+v", content)
	}
	root := content.Document.Blocks[0]
	if root.Kind != "group" || root.Language != "zh-Hant" || root.Direction != "ltr" || root.Children[0].Kind != "group" {
		t.Fatal("block flow or inherited text semantics lost")
	}
	byKind := make(map[string][]Block)
	var walk func(Block)
	walk = func(node Block) {
		byKind[node.Kind] = append(byKind[node.Kind], node)
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(root)
	for _, kind := range []string{"inlineGroup", "figure", "figureCaption", "ruby", "rubyText", "strong", "emphasis", "quote", "preformatted", "break", "separator", "table", "row", "headerCell", "cell"} {
		if len(byKind[kind]) == 0 {
			t.Fatalf("missing projected semantics: %s", kind)
		}
	}
	links := byKind["link"]
	if len(links) != 3 || links[0].Role != "noteref" || links[0].Target == nil || *links[0].Target != (ReadingTarget{ChapterIndex: 1, ContentRevision: 9, Anchor: "a1"}) || links[1].URL != "https://example.invalid/read" || links[1].Target != nil || !links[2].Unavailable || links[2].Target != nil {
		t.Fatalf("link actions: %+v", links)
	}
	if len(byKind["orderedList"]) != 1 || byKind["orderedList"][0].Start == nil || *byKind["orderedList"][0].Start != 0 || byKind["cell"][0].ColSpan != 2 || byKind["headerCell"][0].RowSpan != 2 {
		t.Fatal("list/table structure lost")
	}
	images := byKind["image"]
	if !reflect.DeepEqual(issued, []string{"r1"}) || len(images) != 1 || images[0].Resource == nil || images[0].Resource.Href != "/api/books/proof/chapters/0/resources/r1?contentRevision=9" || images[0].Resource.MediaType != "image/png" || images[0].Width != 40 || images[0].Height != 30 || images[0].Alt != "A small map" {
		t.Fatalf("resource projection: %+v; issued %v", images, issued)
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "fallback text") || strings.Contains(string(encoded), `"kind":"unsupported"`) {
		t.Fatal("unsupported source content did not become safe fallback prose")
	}
	for _, private := range []string{"private-", ".xhtml", ".png", "publisher-style", `"image":`, `"link":`, `"Images":`, `"Reference":`} {
		// MIME type image/png is public; private image filenames end in .png.
		if strings.Contains(string(encoded), private) {
			t.Fatalf("private preparation data leaked: %s", private)
		}
	}
	note, err := epubContent(context.Background(), 9, prepared.Sections[1].Title, sections[1], nil)
	if err != nil {
		t.Fatal(err)
	}
	if node := note.Document.Blocks[0].Children[0]; node.ID != links[0].Target.Anchor || node.Role != "footnote" {
		t.Fatal("note anchor/semantics lost")
	}
	// Projection owns its mutable containers; canonical prepared content stays
	// untouched if a downstream consumer modifies the response.
	*byKind["orderedList"][0].Start = 10
	again, err := epubContent(context.Background(), 9, "Main", sections[0], func(string) string { return "/api/resource" })
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range again.Document.Blocks[0].Children {
		if child.Kind == "orderedList" && *child.Start != 0 {
			t.Fatal("projection aliased canonical list state")
		}
	}
}

func TestEPUBProjectionRejectsUnresolvedImagesAndUnknownKinds(t *testing.T) {
	for _, root := range []epub.Node{{Kind: "image", Image: "r1"}, {Kind: "future-kind"}} {
		content, err := epubContent(context.Background(), 1, "Title", epub.PreparedSection{Root: root}, func(string) string { t.Fatal("issuer called without a validated binding"); return "" })
		if err == nil || content.Version != 0 {
			t.Fatal("invalid prepared node produced readable content")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := epubContent(ctx, 1, "Title", epub.PreparedSection{Root: epub.Node{Kind: "group"}}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestVersionOneProseWireShapeIsUnchanged(t *testing.T) {
	encoded, err := json.Marshal(prose(3, "Title", []Block{{Kind: "paragraph", Text: "Literal <text>"}, {Kind: "image", Resource: &ResourceReference{Href: "/api/image"}, Alt: "Map"}}))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"contentRevision":3,"version":1,"document":{"kind":"prose","title":"Title","blocks":[{"kind":"paragraph","text":"Literal \u003ctext\u003e"},{"kind":"image","resource":{"href":"/api/image"},"alt":"Map"}]}}`
	if string(encoded) != want {
		t.Fatalf("version-1 wire changed: %s", encoded)
	}
	encoded, err = json.Marshal(prose(3, "Empty", []Block{}))
	if err != nil || !strings.Contains(string(encoded), `"blocks":[]`) {
		t.Fatal("empty version-1 blocks no longer serialized")
	}
}

func TestEPUBCoverPlaceholderPreservesAnchorsWithoutInventedProse(t *testing.T) {
	section := epub.PreparedSection{CoverPlaceholder: true, Root: epub.Node{Kind: "group", ID: "a1"}}
	content, err := epubContent(context.Background(), 9, "Cover", section, func(string) string { t.Fatal("placeholder issued a resource"); return "" })
	if err != nil {
		t.Fatal(err)
	}
	if !content.Document.CoverPlaceholder || content.Document.Blocks[0].ID != "a1" || content.Document.Blocks[0].Text != "" {
		t.Fatalf("placeholder projection: %+v", content)
	}
}
