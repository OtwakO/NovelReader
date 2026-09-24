// Package inboxfiles owns bounded filesystem mechanics for completed inbox files.
// It owns no database, claim, receipt state or authority to delete an input.
package inboxfiles

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"

	"github.com/otwako/novelreader/internal/readerstore"
)

var (
	ErrChanged = errors.New("inbox: entry changed during acquisition or review")
	ErrMissing = errors.New("inbox: entry is missing")
	ErrType    = errors.New("inbox: entry must be a regular file")
)

func SameFile(expected, current os.FileInfo) bool {
	return current.Mode().IsRegular() && os.SameFile(expected, current) && expected.Size() == current.Size() && expected.ModTime().Equal(current.ModTime())
}

// Inspect rejects links and checks readability before an owner records move intent.
func Inspect(root *os.Root, name string, limit int64) (os.FileInfo, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrMissing
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrType
	}
	if info.Size() > limit {
		return nil, readerstore.ErrFileTooLarge
	}
	input, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := input.Stat()
	if err = errors.Join(statErr, input.Close()); err != nil {
		return nil, err
	}
	if !SameFile(info, opened) {
		return nil, ErrChanged
	}
	return info, nil
}

// CopyWork never removes input or installs output. The caller owns work cleanup
// and receipt finalization. Streaming occurs outside the file-mutation gate.
func CopyWork(ctx context.Context, inbox, managed *os.Root, name, destination string, expected os.FileInfo, limit int64) (int64, error) {
	input, err := inbox.Open(name)
	if err != nil {
		return 0, err
	}
	current, err := input.Stat()
	if err == nil && !SameFile(expected, current) {
		err = ErrChanged
	}
	var size int64
	if err == nil {
		size, err = readerstore.WriteWorkFile(ctx, managed, destination, input, limit)
	}
	if err == nil {
		current, err = input.Stat()
		if err == nil && (size != expected.Size() || !SameFile(expected, current)) {
			err = ErrChanged
		}
	}
	return size, errors.Join(err, input.Close())
}

// Evidence is file identity/content, not deletion approval. Stores retain it
// privately alongside database/claim ownership and revalidate before removal.
type Evidence struct {
	Info   os.FileInfo
	Digest [sha256.Size]byte
}

func Review(ctx context.Context, root *os.Root, name string, limit int64) (Evidence, error) {
	if err := ctx.Err(); err != nil {
		return Evidence{}, err
	}
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return Evidence{}, nil
	}
	if err != nil {
		return Evidence{}, err
	}
	if !info.Mode().IsRegular() {
		return Evidence{}, ErrType
	}
	if info.Size() > limit {
		return Evidence{}, readerstore.ErrFileTooLarge
	}
	file, err := root.Open(name)
	if err != nil {
		return Evidence{}, err
	}
	hash := sha256.New()
	count, readErr := io.Copy(hash, io.LimitReader(contextReader{ctx, file}, limit+1))
	current, statErr := file.Stat()
	if err = errors.Join(readErr, statErr, file.Close()); err != nil {
		return Evidence{}, err
	}
	if count != info.Size() || !SameFile(info, current) {
		return Evidence{}, ErrChanged
	}
	result := Evidence{Info: info}
	copy(result.Digest[:], hash.Sum(nil))
	return result, nil
}

func SameEvidence(expected, current Evidence) bool {
	if expected.Info == nil || current.Info == nil {
		return expected.Info == nil && current.Info == nil
	}
	return SameFile(expected.Info, current.Info) && expected.Digest == current.Digest
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}
