package txtimport

import (
	"context"
	"errors"

	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

func analyzeNext(ctx context.Context, readers *readerstore.Manager, id readerstore.UserID) (bool, error) {
	// A queued job may wait for capacity without losing its wake-up. Quiesce and
	// shutdown cancel this wait; a capacity timeout would strand durable work.
	home, err := readers.Open(ctx, id)
	if err != nil {
		return false, err
	}
	worked, err := txtstore.NewStore(home.DB(), home.Files()).AnalyzeNext(ctx)
	return worked, errors.Join(err, home.Close())
}
