package readerstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
)

var ErrFileTooLarge = errors.New("readerstore: work file exceeds byte limit")

// WriteWorkFile creates an exclusive confined scratch file. The caller owns
// cleanup, including partial output on error, and supplies a positive byte cap.
// Input remains caller-owned; cancellation cannot interrupt a blocked Read.
func WriteWorkFile(ctx context.Context, root *os.Root, name string, input io.Reader, limit int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := root.MkdirAll(path.Dir(name), 0700); err != nil {
		return 0, err
	}
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	size, err := io.Copy(f, io.LimitReader(workReader{ctx, input}, limit+1))
	if size > limit {
		err = errors.Join(ErrFileTooLarge, err)
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = f.Sync()
	}
	return size, errors.Join(err, f.Close())
}

type workReader struct {
	ctx    context.Context
	source io.Reader
}

func (r workReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(data)
}
