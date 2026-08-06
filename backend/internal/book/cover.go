package book

import (
	"context"
	"fmt"

	"github.com/otwako/novelreader/internal/booksource"
)

// GetBookCover fetches and optionally decodes a stored book's cover image.
func (s *Searcher) GetBookCover(ctx context.Context, src booksource.BookSource, b *Book) ([]byte, string, error) {
	if b == nil {
		return nil, "", fmt.Errorf("cover: URL is empty")
	}
	return s.getStoredImage(ctx, src, b, nil, b.CoverURL, src.CoverDecodeJS, true, "cover")
}
