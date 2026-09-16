package epub

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in, like the inventory check. Never log text, original anchors,
// references or image bytes; real-book compatibility is not a CI dependency.
func TestLocalSectionNormalization(t *testing.T) {
	f, size := openLocalEPUB(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	p, err := Inspect(ctx, f, size)
	if err != nil {
		t.Fatal(err)
	}
	a, err := openArchive(ctx, f, size)
	if err != nil {
		t.Fatal(err)
	}
	items := make(map[string]Item, len(p.Items))
	for _, item := range p.Items {
		items[item.ID] = item
	}
	var inputBytes, outputBytes, images, links, anchors int
	diagnostics := map[string]int{}
	for _, spine := range p.Spine {
		item := items[spine.ContentID]
		data, err := a.readSection(ctx, item.Reference.Path)
		if err != nil {
			t.Fatal(err)
		}
		section, err := NormalizeSection(ctx, data, item.Reference.Path)
		if err != nil {
			t.Fatal(err)
		}
		// Serialize only the semantic tree, not private reference/anchor maps.
		encoded, err := json.Marshal(section.Root)
		if err != nil {
			t.Fatal(err)
		}
		inputBytes += len(data)
		outputBytes += len(encoded)
		images += len(section.Images)
		links += len(section.Links)
		anchors += len(section.Anchors)
		for _, code := range section.Diagnostics {
			diagnostics[code]++
		}
	}
	t.Logf("normalization only: sections=%d input bytes=%d tree JSON bytes=%d image bindings=%d link bindings=%d anchors=%d diagnostics=%v", len(p.Spine), inputBytes, outputBytes, images, links, anchors, diagnostics)
}

func BenchmarkNormalizeSection(b *testing.B) {
	for _, fixture := range []struct{ name, body string }{
		{"normal", `<h1>Title</h1><p>Text with <em>emphasis</em> and <a href="notes.xhtml#one">a note</a>.</p>`},
		{"large_single_document", strings.Repeat("<p>"+strings.Repeat("Novel prose. ", 8)+"</p>", 10000)},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			data := []byte(xhtml(fixture.body))
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := NormalizeSection(context.Background(), data, "chapter.xhtml"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
