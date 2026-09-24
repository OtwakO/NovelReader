// Package epub interprets local EPUB originals without extracting archive paths,
// fetching remote resources, or owning reader storage and publication state.
package epub

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"
)

var (
	ErrArchive     = errors.New("epub: invalid archive")
	ErrLimit       = errors.New("epub: resource limit exceeded")
	ErrReference   = errors.New("epub: invalid local reference")
	ErrPackage     = errors.New("epub: invalid package")
	ErrUnsupported = errors.New("epub: unsupported publication")
)

// Initial import budgets, not operator settings. They bound work before any
// publication exists; semantic section/image limits belong to normalization.
const (
	// MaxInputBytes is shared by acquisition and archive inspection.
	MaxInputBytes        int64  = 256 << 20
	maxEntries                  = 20000
	maxExpandedBytes     uint64 = 1 << 30
	maxMetadataBytes     int64  = 4 << 20
	maxMetadataReadBytes int64  = 16 << 20
)

// An archive belongs to one sequential inspection/preparation. The caller owns
// the ReaderAt and keeps it open and unchanged for the entire operation.
type archive struct {
	files     map[string]*zip.File
	readBytes int64
}

func openArchive(ctx context.Context, source io.ReaderAt, size int64) (*archive, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, ErrArchive
	}
	if size > MaxInputBytes {
		return nil, ErrLimit
	}
	zr, err := zip.NewReader(contextReaderAt{ctx, source}, size)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArchive, err)
	}
	if len(zr.File) > maxEntries {
		return nil, ErrLimit
	}
	a := &archive{files: make(map[string]*zip.File, len(zr.File))}
	var expanded uint64
	seen := make(map[string]bool, len(zr.File))
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(f.Name, "/")
		if !validEntryName(name) || seen[name] {
			return nil, ErrArchive
		}
		seen[name] = true
		if f.Flags&1 != 0 || (f.Method != zip.Store && f.Method != zip.Deflate) {
			return nil, ErrUnsupported
		}
		if f.UncompressedSize64 > maxExpandedBytes-expanded {
			return nil, ErrLimit
		}
		expanded += f.UncompressedSize64
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() {
			return nil, ErrUnsupported
		}
		a.files[name] = f
	}
	return a, nil
}

func validEntryName(name string) bool {
	return utf8.ValidString(name) && name != "" && name != "." && path.Clean(name) == name &&
		!strings.HasPrefix(name, "/") && name != ".." && !strings.HasPrefix(name, "../") &&
		!strings.ContainsAny(name, "\\\x00")
}

func (a *archive) readMetadata(ctx context.Context, name string) ([]byte, error) {
	return a.readBounded(ctx, name, maxMetadataBytes, maxMetadataReadBytes)
}

func (a *archive) readSection(ctx context.Context, name string) ([]byte, error) {
	return a.readBounded(ctx, name, maxSectionBytes, int64(maxExpandedBytes))
}

func (a *archive) readBounded(ctx context.Context, name string, entryLimit, totalLimit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, ok := a.files[name]
	if !ok {
		return nil, fmt.Errorf("%w: missing resource", ErrPackage)
	}
	if f.UncompressedSize64 > uint64(entryLimit) {
		return nil, ErrLimit
	}
	r, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArchive, err)
	}
	defer r.Close()
	remaining := min(entryLimit, totalLimit-a.readBytes)
	if remaining < 0 {
		return nil, ErrLimit
	}
	data, err := io.ReadAll(io.LimitReader(contextReader{ctx, r}, remaining+1))
	a.readBytes += int64(len(data))
	if int64(len(data)) > remaining {
		return nil, ErrLimit
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrArchive, err)
	}
	return data, nil
}

type contextReader struct {
	ctx    context.Context
	source io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(p)
}

type contextReaderAt struct {
	ctx    context.Context
	source io.ReaderAt
}

func (r contextReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.ReadAt(p, offset)
}
