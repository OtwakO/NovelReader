package chapterresource

import (
	"context"
	"log/slog"
	"time"
)

// Cleanup expires recipes for inactive readers as well as active ones.
func (s *Store) Cleanup(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM bundles WHERE expires_at <= ?", s.now().UnixNano())
	return err
}

// DeleteReader is called on account deletion, not ordinary runtime eviction or
// failed replacement. Replacement identity already prevents old-reference reuse.
func (s *Store) DeleteReader(ctx context.Context, readerID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM bundles WHERE reader_id = ?", readerID)
	return err
}

func (s *Store) cleanupLoop(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := s.Cleanup(cleanup)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("chapter resource cleanup failed", "error", err)
			}
		}
	}
}
