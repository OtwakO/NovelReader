package epubstore

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/otwako/novelreader/internal/epub"
)

func writeDerivative(ctx context.Context, root *os.Root, path string, data []byte) error {
	f, err := root.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	w := &outputWriter{ctx: ctx, target: f, remaining: maxDerivativeBytes}
	_, err = w.Write(data)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

type outputWriter struct {
	ctx                context.Context
	target             io.Writer
	remaining, written int64
}

func (w *outputWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(data)) > w.remaining {
		return 0, epub.ErrLimit
	}
	n, err := w.target.Write(data)
	w.remaining -= int64(n)
	w.written += int64(n)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}
