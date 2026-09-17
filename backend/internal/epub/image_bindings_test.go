package epub

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"
)

func TestImageBindingsAndRepeatedResources(t *testing.T) {
	ctx := context.Background()
	image := rasterFixture(t, "image/png")
	data := fixtureArchive(t, []fixtureEntry{{"pic.png", string(image)}, {"bad.png", "not image data"}})
	a, err := openArchive(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	r := newImageResolver(a, []Item{{Reference: Reference{Path: "pic.png"}, MediaType: "image/png"}, {Reference: Reference{Path: "bad.png"}, MediaType: "image/png"}})
	s, err := NormalizeSection(ctx, []byte(xhtml(`<p>Prose</p><img src="pic.png"/><img src="pic.png"/><img src="absent.png" alt="Missing illustration"/><img src="bad.png"/>`)), "chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	readable, err := r.resolveSection(ctx, &s)
	if err != nil || !readable {
		t.Fatalf("readability: %v %v", readable, err)
	}
	images := nodesOfKind(s.Root, "image")
	if len(images) != 2 || len(s.Images) != 2 || images[0].Width != 3 || images[1].Height != 2 {
		t.Fatalf("image binding: %+v", images)
	}
	if a.readBytes != int64(len(image)+len("not image data")) {
		t.Fatal("repeated image was read/validated again")
	}
	if !slices.Contains(s.Diagnostics, "image_unavailable") || !slices.Contains(s.Diagnostics, "image_invalid") {
		t.Fatalf("diagnostics: %v", s.Diagnostics)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.resolve(ctx, Reference{Path: "pic.png"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached result bypassed cancellation: %v", err)
	}
}

func TestImageBindingFailureBoundaries(t *testing.T) {
	ctx := context.Background()
	data := fixtureArchive(t, []fixtureEntry{{"pic.png", string(rasterFixture(t, "image/png"))}})
	a, err := openArchive(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	r := newImageResolver(a, []Item{{Reference: Reference{Path: "pic.png"}, MediaType: "image/png"}})
	s, err := NormalizeSection(ctx, []byte(xhtml(`<img src="pic.png"/>`)), "chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	a.readBytes = int64(maxExpandedBytes)
	if _, err := r.resolveSection(ctx, &s); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit was downgraded to optional image warning: %v", err)
	}
	missing, err := NormalizeSection(ctx, []byte(xhtml(`<img src="missing.png"/>`)), "chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	readable, err := r.resolveSection(ctx, &missing)
	if err != nil || readable || !slices.Contains(missing.Diagnostics, "no_readable_content") {
		t.Fatalf("empty section: %v %v %v", readable, err, missing.Diagnostics)
	}
}
