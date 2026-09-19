package fileimport

import (
	"context"
	"database/sql"
	"errors"

	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

func prepareNext(ctx context.Context, readers *readerstore.Manager, id readerstore.UserID) (bool, error) {
	// Waiting for capacity must not lose durable work. Quiesce/shutdown cancel it.
	home, err := readers.Open(ctx, id)
	if err != nil {
		return false, err
	}
	worked, err := prepareHome(ctx, home)
	return worked, errors.Join(err, home.Close())
}

// Compare only scheduling evidence. Each store owns its selection SQL, guarded
// claim, preparation and cleanup; the pool never reads provider tables.
func prepareHome(ctx context.Context, home *readerstore.Home) (bool, error) {
	txt := txtstore.NewStore(home.DB(), home.Files())
	epub := epubstore.NewStore(home.DB(), home.Files())
	text, book, err := pendingWork(ctx, home.DB())
	if err != nil {
		return false, err
	}
	if text.ReceiptID == "" && book.ReceiptID == "" {
		return false, nil
	}
	// Equal timestamps use TXT first; each store orders its own ties by receipt ID.
	// A new generation has its own queue time and cannot reuse the selected claim.
	if text.ReceiptID != "" && (book.ReceiptID == "" || text.QueuedAt <= book.CreatedAt) {
		worked, err := txt.AnalyzePending(ctx, text)
		// A removed/replaced snapshot merits a fresh turn, not a lost wake-up.
		return worked || err == nil || errors.Is(err, txtstore.ErrStateChanged), err
	}
	worked, err := epub.PreparePending(ctx, book)
	return worked || errors.Is(err, epubstore.ErrStateChanged), err
}

func isSuperseded(err error) bool {
	return errors.Is(err, txtstore.ErrStateChanged) || errors.Is(err, epubstore.ErrStateChanged)
}

// Release the read snapshot before claiming or doing file I/O. Independent reads
// could miss an older concurrent TXT enqueue yet observe a newer EPUB enqueue.
func pendingWork(ctx context.Context, db *sql.DB) (txtstore.PendingAnalysis, epubstore.PreparationAttempt, error) {
	var text txtstore.PendingAnalysis
	var book epubstore.PreparationAttempt
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return text, book, err
	}
	defer tx.Rollback()
	text, err = txtstore.NextAnalysis(ctx, tx)
	if err != nil && !errors.Is(err, txtstore.ErrNotFound) {
		return text, book, err
	}
	book, err = epubstore.NextPreparation(ctx, tx)
	if err != nil && !errors.Is(err, epubstore.ErrNotFound) {
		return text, book, err
	}
	return text, book, tx.Commit()
}
