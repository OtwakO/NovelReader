package txtstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/otwako/novelreader/internal/txt"
)

// Receive streams one upload into disposable work, then finalizes its original.
// Success means durable acquisition, not successful analysis or shelf admission.
// On an interrupted finalization the returned receipt remains recoverable.
func (s *Store) Receive(ctx context.Context, name string, input io.Reader) (Receipt, error) {
	value := Receipt{ID: rand.Text(), OriginalName: name, State: Receiving, CreatedAt: time.Now().UnixMilli()}
	var err error
	value.Path, err = managedPath(name, value.ID)
	if err != nil {
		return Receipt{}, err
	}
	value.UpdatedAt = value.CreatedAt
	root, err := s.files.OpenRoot()
	if err != nil {
		return Receipt{}, err
	}
	defer root.Close()
	_, err = s.db.ExecContext(ctx, `INSERT INTO txt_files(id,original_name,path,state,created_at,updated_at) VALUES(?,?,?,?,?,?)`, value.ID, name, value.Path, Receiving, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return Receipt{}, err
	}

	value.Size, err = receiveWork(ctx, root, workPath(value.ID), input)
	if err != nil {
		return s.failTransfer(ctx, root, value, err)
	}
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return s.failTransfer(ctx, root, value, err)
	}
	defer unlock()
	// Once finalization starts, finish the short metadata/file sequence even if
	// the upload request disconnects. A database failure still leaves durable intent.
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	if err := s.transition(finalCtx, value.ID, Receiving, Receiving, value.Size, ""); err != nil {
		return value, err
	}
	if err := root.MkdirAll(path.Dir(value.Path), 0o700); err != nil {
		return value, err
	}
	if err := root.Rename(workPath(value.ID), value.Path); err != nil {
		return value, err
	}
	if err := s.transition(finalCtx, value.ID, Receiving, Received, value.Size, ""); err != nil {
		return value, err
	}
	return s.Get(finalCtx, value.ID)
}

func receiveWork(ctx context.Context, root *os.Root, destination string, input io.Reader) (int64, error) {
	if err := root.MkdirAll(path.Dir(destination), 0o700); err != nil {
		return 0, err
	}
	file, err := root.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	size, copyErr := io.Copy(file, io.LimitReader(receiveReader{ctx: ctx, reader: input}, txt.MaxInputBytes+1))
	if size > txt.MaxInputBytes {
		copyErr = fmt.Errorf("txtstore: file exceeds %d bytes", txt.MaxInputBytes)
	}
	if copyErr == nil {
		copyErr = file.Sync()
	}
	return size, errors.Join(copyErr, file.Close())
}

func (s *Store) failTransfer(ctx context.Context, root *os.Root, value Receipt, cause error) (Receipt, error) {
	value, err := s.recordFailure(ctx, root, value, cause)
	return value, errors.Join(cause, err)
}

func (s *Store) recordFailure(ctx context.Context, root *os.Root, value Receipt, cause error) (Receipt, error) {
	cleanupErr := removeIfPresent(root, workPath(value.ID))
	cause = errors.Join(cause, cleanupErr)
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	err := s.transition(cleanupCtx, value.ID, Receiving, Failed, value.Size, cause.Error())
	if err == nil {
		value.State = Failed
		value.Error = cause.Error()
	}
	return value, errors.Join(cleanupErr, err)
}

type receiveReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r receiveReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

func removeIfPresent(root *os.Root, name string) error {
	err := root.Remove(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
