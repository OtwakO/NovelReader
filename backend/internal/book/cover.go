package book

import (
	"context"
	"fmt"
	"strings"

	"github.com/otwako/novelreader/internal/booksource"
)

// GetBookCover fetches and optionally decodes a stored book's cover image.
func (s *Searcher) GetBookCover(ctx context.Context, src booksource.BookSource, b *Book) ([]byte, string, error) {
	if s == nil || b == nil || strings.TrimSpace(b.CoverURL) == "" {
		return nil, "", fmt.Errorf("cover: URL is empty")
	}
	return s.getImageWithContext(ctx, src, bookContext(b, src), nil, b.CoverURL, src.CoverDecodeJS, true, "cover")
}
