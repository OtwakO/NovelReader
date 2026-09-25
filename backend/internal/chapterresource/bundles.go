package chapterresource

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Admit publishes immutable recipes before a reading response issues their URLs.
// Reusing the same owner and recipes only extends availability; callers retain
// responsibility for the separate chapter freshness deadline.
func (s *Store) Admit(ctx context.Context, owner Owner, images Images, until time.Time) (Reference, error) {
	if owner.ReaderID == "" || owner.Generation == "" || owner.BookID == "" || owner.SourceID == "" || owner.SourceIdentity == "" || len(images.URLs) == 0 {
		return Reference{}, errors.New("chapter resources: incomplete bundle ownership or images")
	}
	if !until.After(s.now()) {
		return Reference{}, ErrUnavailable
	}
	ownership, err := json.Marshal(owner)
	if err != nil {
		return Reference{}, err
	}
	payload, err := json.Marshal(images)
	if err != nil {
		return Reference{}, fmt.Errorf("chapter resources: encode recipes: %w", err)
	}
	hash := sha256.New()
	_, _ = hash.Write(ownership)
	_, _ = hash.Write(payload)
	id := hex.EncodeToString(hash.Sum(nil))
	size := int64(len(ownership) + len(payload) + len(id) + len(owner.ReaderID))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Reference{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM bundles WHERE expires_at <= ?", s.now().UnixNano()); err != nil {
		return Reference{}, err
	}
	var existing int64
	err = tx.QueryRowContext(ctx, "SELECT expires_at FROM bundles WHERE id = ?", id).Scan(&existing)
	if err == nil {
		// Even after an operator lowers limits, an already admitted bundle can be
		// reused without new allocation. Never shorten a previously issued promise.
		if until.UnixNano() < existing {
			until = time.Unix(0, existing)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE bundles SET expires_at = ? WHERE id = ?", until.UnixNano(), id); err != nil {
			return Reference{}, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		var totalBytes, readerBytes int64
		var totalCount, readerCount int
		err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes),0), COUNT(*),
   COALESCE(SUM(CASE WHEN reader_id = ? THEN bytes ELSE 0 END),0),
   COALESCE(SUM(CASE WHEN reader_id = ? THEN 1 ELSE 0 END),0) FROM bundles`, owner.ReaderID, owner.ReaderID).
			Scan(&totalBytes, &totalCount, &readerBytes, &readerCount)
		if err != nil {
			return Reference{}, err
		}
		if size > s.limits.ReaderBytes-readerBytes || size > s.limits.TotalBytes-totalBytes || readerCount >= s.limits.ReaderBundles || totalCount >= s.limits.TotalBundles {
			// Commit expired-data reclamation even when this new admission cannot fit.
			if err := tx.Commit(); err != nil {
				return Reference{}, err
			}
			return Reference{}, ErrCapacity
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO bundles(id,reader_id,owner,payload,bytes,expires_at) VALUES(?,?,?,?,?,?)`, id, owner.ReaderID, ownership, payload, size, until.UnixNano()); err != nil {
			return Reference{}, err
		}
	} else {
		return Reference{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reference{}, err
	}
	return Reference{ID: id, AvailableUntil: until}, nil
}

// Resolve returns a request-owned copy. A different reader, replacement home,
// source definition or interpretation cannot resolve an old reference. Current
// book/source membership must still be checked by the reading boundary.
func (s *Store) Resolve(ctx context.Context, owner Owner, id string) (Images, error) {
	ownership, err := json.Marshal(owner)
	if err != nil {
		return Images{}, err
	}
	var payload []byte
	err = s.db.QueryRowContext(ctx, `SELECT payload FROM bundles WHERE id = ? AND reader_id = ? AND owner = ? AND expires_at > ?`, id, owner.ReaderID, ownership, s.now().UnixNano()).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return Images{}, ErrUnavailable
	}
	if err != nil {
		return Images{}, err
	}
	var images Images
	if err := json.Unmarshal(payload, &images); err != nil {
		return Images{}, fmt.Errorf("chapter resources: decode stored recipes: %w", err)
	}
	return images, nil
}
