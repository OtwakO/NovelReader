package txtstore

import (
	"context"
	"io"
)

const candidateCheckBytes = 1 << 20

// Check durable ownership at coarse source-byte boundaries, not once per line.
// The final result transaction still decides whether this generation may publish.
type candidateReader struct {
	ctx        context.Context
	store      *Store
	fileID     string
	generation int64
	input      io.Reader
	sinceCheck int
}

func (r *candidateReader) check() error {
	ctx, cancel := context.WithTimeout(r.ctx, metadataTimeout)
	defer cancel()
	var current bool
	err := r.store.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM txt_interpretations i JOIN txt_files f ON f.id=i.file_id WHERE i.file_id=? AND i.generation=? AND i.role='candidate' AND i.state='analyzing' AND f.state='acquired')`, r.fileID, r.generation).Scan(&current)
	if err != nil {
		return err
	}
	if !current {
		return ErrStateChanged
	}
	r.sinceCheck = 0
	return nil
}

func (r *candidateReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if r.sinceCheck >= candidateCheckBytes {
		if err := r.check(); err != nil {
			return 0, err
		}
	}
	n, err := r.input.Read(data)
	r.sinceCheck += n
	return n, err
}
