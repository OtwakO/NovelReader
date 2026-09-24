package epubstore

import (
	"crypto/rand"
	"github.com/otwako/novelreader/internal/epub"
)

// PreparedResource is private immutable delivery evidence, not a public URL.
// Original bytes stay in the archive; only derivatives have a separate byte size.
type PreparedResource struct {
	ID              string
	Image           epub.PreparedImage
	DerivativeBytes int64
}

type resourceIndex struct {
	byPath map[string]PreparedResource
	sizes  map[string]int64
}

func newResourceIndex() *resourceIndex {
	return &resourceIndex{byPath: map[string]PreparedResource{}, sizes: map[string]int64{}}
}
func (r *resourceIndex) add(image epub.PreparedImage) {
	if _, ok := r.byPath[image.Reference.Path]; ok {
		return
	}
	id := image.DerivativeID
	if id == "" {
		id = rand.Text()
	}
	r.byPath[image.Reference.Path] = PreparedResource{ID: id, Image: image, DerivativeBytes: r.sizes[image.DerivativeID]}
}
