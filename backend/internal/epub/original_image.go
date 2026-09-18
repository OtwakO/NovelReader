package epub

import (
	"context"
	"io"
)

// ReadOriginalImage reads an already validated original image from an unchanged
// managed EPUB. The caller must resolve an authorized resource ID to this private
// reference and preserve preparation evidence; never pass a client-supplied path.
// Portable validation is responsible for establishing that evidence on restore.
//
// Only the requested member is inflated, with the same image-byte budget and ZIP
// checksum checks as preparation. Pixels and package semantics are NOT revalidated.
// The caller owns the ReaderAt and the returned bytes. No archive/image cache or
// extracted file is retained. Derivatives are ordinary managed files, not inputs
// to this function.
func ReadOriginalImage(ctx context.Context, original io.ReaderAt, size int64, ref Reference) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ref.Fragment != "" || !validEntryName(ref.Path) {
		return nil, ErrReference
	}
	a, err := openArchive(ctx, original, size)
	if err != nil {
		return nil, err
	}
	if a.files[ref.Path] == nil {
		return nil, ErrImageUnavailable
	}
	return a.readBounded(ctx, ref.Path, maxRasterBytes, maxRasterBytes)
}
