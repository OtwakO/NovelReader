package txtimport

import (
	"context"
	"errors"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

func analyzeNext(ctx context.Context, readers *readerstore.Manager, id readerstore.UserID) (bool, error) {
	openCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	home, err := readers.Open(openCtx, id)
	cancel()
	if err != nil {
		return false, err
	}
	worked, err := txtstore.NewStore(home.DB(), home.Files()).AnalyzeNext(ctx)
	return worked, errors.Join(err, home.Close())
}
