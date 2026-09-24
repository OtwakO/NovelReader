package fileimport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

var (
	ErrTXTRecovery  = errors.New("fileimport: TXT recovery incomplete")
	ErrEPUBRecovery = errors.New("fileimport: EPUB recovery incomplete")
)

// RecoverHome reconciles unfinished TXT and EPUB records. Call before serving intake,
// or after restore while that reader's requests and workers remain quiescent.
// Runtime eviction, ordinary reads and analysis jobs are not recovery triggers.
func RecoverHome(ctx context.Context, readers *readerstore.Manager, id readerstore.UserID) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	home, err := readers.Open(ctx, id)
	if err != nil {
		return err
	}
	txtErr := txtstore.NewStore(home.DB(), home.Files()).Recover(ctx)
	if txtErr != nil {
		txtErr = fmt.Errorf("%w: %w", ErrTXTRecovery, txtErr)
	}
	epubErr := epubstore.NewStore(home.DB(), home.Files()).Recover(ctx)
	if epubErr != nil {
		epubErr = fmt.Errorf("%w: %w", ErrEPUBRecovery, epubErr)
	}
	return errors.Join(txtErr, epubErr, home.Close())
}

// Start recovers retained account homes before admitting any background work.
// Call before HTTP serving; new accounts created afterward have empty homes.
// A per-home problem is logged, not allowed to stop unrelated readers. Failed
// records are retained; workers select queued work, never replay intake.
func Start(ctx context.Context, readers *readerstore.Manager, ids []readerstore.UserID) (*Pool, error) {
	for _, id := range ids {
		if err := RecoverHome(ctx, readers, id); err != nil {
			slog.Warn("File import startup recovery incomplete", "reader_id", id, "error", err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	pool := NewPool(readers)
	for _, id := range ids {
		if err := pool.Notify(id); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return pool, nil
}
