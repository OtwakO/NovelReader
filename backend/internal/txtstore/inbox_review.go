package txtstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/otwako/novelreader/internal/inboxfiles"
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
	input     inboxfiles.Evidence
	removable bool
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
	input, err := inboxfiles.Review(ctx, inbox, claim.Name, txt.MaxInputBytes)
	if err != nil {
		return nil, err
	}
	review := &InboxReview{Claim: claim, InputPresent: input.Info != nil, owner: s.db, claim: claim, input: input}
	if input.Info == nil || value == nil || value.State == Removing {
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
	original, err := inboxfiles.Review(ctx, managed, value.Path, txt.MaxInputBytes)
	if err != nil {
		return nil, err
	}
	review.removable = original.Info != nil && original.Info.Size() == value.Size && original.Digest == input.Digest
	review.CanRemove = review.removable
	return review, nil
}

func (s *Store) settledInboxClaim(ctx context.Context, id string) (InboxClaim, *Receipt, error) {
	claim, err := s.GetInboxClaim(ctx, id)
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
