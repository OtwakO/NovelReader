package txtstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/otwako/novelreader/internal/txt"
)

var ErrInboxNotDuplicate = errors.New("txtstore: inbox file has no matching managed original; keep or release it")

// InboxReview is a server-owned approval proof, not a client request DTO. Its
// private fields bind the reviewed entry to this database lifetime. HTTP callers
// must retain the returned proof; decoding its display fields cannot authorize IO.
type InboxReview struct {
	Claim        InboxClaim
	InputPresent bool
	CanRemove    bool

	owner     *sql.DB
	claim     InboxClaim
	input     fileReview
	removable bool
}

type fileReview struct {
	info   os.FileInfo
	digest [sha256.Size]byte
}

// ReviewInbox inspects one settled claim. Hashing is bounded and occurs only for
// explicit leftover review, never for normal section reads or recovery.
func (s *Store) ReviewInbox(ctx context.Context, id string) (*InboxReview, error) {
	claim, value, err := s.settledInboxClaim(ctx, id)
	if err != nil {
		return nil, err
	}
	inbox, err := s.files.OpenInbox()
	if err != nil {
		return nil, err
	}
	defer inbox.Close()
	input, err := reviewFile(ctx, inbox, claim.Name)
	if err != nil {
		return nil, err
	}
	review := &InboxReview{Claim: claim, InputPresent: input.info != nil, owner: s.db, claim: claim, input: input}
	if input.info == nil || value == nil || value.State == Removing {
		return review, nil
	}
	managed, err := s.files.OpenRoot()
	if err != nil {
		return nil, err
	}
	defer managed.Close()
	if err := validateReceiptPath(*value); err != nil {
		return nil, err
	}
	original, err := reviewFile(ctx, managed, value.Path)
	if err != nil {
		return nil, err
	}
	review.removable = original.info != nil && original.info.Size() == value.Size && original.digest == input.digest
	review.CanRemove = review.removable
	return review, nil
}

func (s *Store) settledInboxClaim(ctx context.Context, id string) (InboxClaim, *Receipt, error) {
	var claim InboxClaim
	err := s.db.QueryRowContext(ctx, `SELECT name,receipt_id FROM txt_inbox_claims WHERE receipt_id=?`, id).Scan(&claim.Name, &claim.ReceiptID)
	if errors.Is(err, sql.ErrNoRows) {
		return claim, nil, ErrNotFound
	}
	if err != nil {
		return claim, nil, err
	}
	if _, err := managedPath(claim.Name, claim.ReceiptID); err != nil {
		return claim, nil, err
	}
	value, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return claim, nil, nil
	}
	if err != nil {
		return claim, nil, err
	}
	if value.State == Receiving {
		return claim, nil, ErrStateChanged
	}
	return claim, &value, nil
}

func reviewFile(ctx context.Context, root *os.Root, name string) (fileReview, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return fileReview{}, nil
	}
	if err != nil {
		return fileReview{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > txt.MaxInputBytes {
		return fileReview{}, fmt.Errorf("txtstore: review requires a regular file within the TXT size limit")
	}
	file, err := root.Open(name)
	if err != nil {
		return fileReview{}, err
	}
	hash := sha256.New()
	count, readErr := io.Copy(hash, io.LimitReader(receiveReader{ctx: ctx, reader: file}, txt.MaxInputBytes+1))
	current, statErr := file.Stat()
	err = errors.Join(readErr, statErr, file.Close())
	if err != nil {
		return fileReview{}, err
	}
	if count != info.Size() || !sameInboxFile(info, current) {
		return fileReview{}, ErrInboxChanged
	}
	result := fileReview{info: info}
	copy(result.digest[:], hash.Sum(nil))
	return result, nil
}

func sameReview(expected, current fileReview) bool {
	if expected.info == nil || current.info == nil {
		return expected.info == nil && current.info == nil
	}
	return sameInboxFile(expected.info, current.info) && expected.digest == current.digest
}
