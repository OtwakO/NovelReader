package readerstore

import "context"

// LockMutation coordinates a composite database/file mutation with snapshots of
// this home. Acquire before beginning its database transaction and release after
// commit and file cleanup. All FileStores leased from this home share the gate.
//
// Callers must keep their Home lease open, call the returned unlock once, and not
// nest acquisitions. Raw file helpers do not acquire it: locking each individual
// file call would leave the database/file boundary unprotected. Ordinary reads,
// analysis, and progress writes need no gate.
func (f FileStore) LockMutation(ctx context.Context) (func(), error) {
	if f.mutation == nil {
		return nil, ErrInvalidFilePath
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case f.mutation <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-f.mutation
			return nil, err
		}
		return func() { <-f.mutation }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
