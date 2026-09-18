package epub

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestReadOriginalImageWithoutRepreparingBook(t *testing.T) {
	image := rasterFixture(t, "image/png")
	// Resource delivery does not parse package/section XML. The archive and its
	// selected resource have already been admitted at the storage boundary.
	data := fixtureArchive(t, []fixtureEntry{{"book.opf", "not XML"}, {"images/art.png", string(image)}})
	original := bytes.Clone(data)
	got, err := ReadOriginalImage(context.Background(), bytes.NewReader(data), int64(len(data)), Reference{Path: "images/art.png"})
	if err != nil || !bytes.Equal(got, image) || !bytes.Equal(data, original) {
		t.Fatalf("original delivery: %v", err)
	}
}

func TestReadOriginalImageBoundaries(t *testing.T) {
	data := fixtureArchive(t, []fixtureEntry{{"large.png", string(make([]byte, maxRasterBytes+1))}})
	for _, tc := range []struct {
		ref  Reference
		want error
	}{
		{Reference{Path: "large.png"}, ErrLimit},
		{Reference{Path: "missing.png"}, ErrImageUnavailable},
		{Reference{Path: "../image.png"}, ErrReference},
		{Reference{Path: "large.png", Fragment: "fragment"}, ErrReference},
	} {
		got, err := ReadOriginalImage(context.Background(), bytes.NewReader(data), int64(len(data)), tc.ref)
		if !errors.Is(err, tc.want) || got != nil {
			t.Fatalf("%+v: %v", tc.ref, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadOriginalImage(ctx, bytes.NewReader(data), int64(len(data)), Reference{Path: "large.png"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
