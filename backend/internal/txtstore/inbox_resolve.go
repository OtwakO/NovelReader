package txtstore

import (
	"context"
	"errors"

	"github.com/otwako/novelreader/internal/inboxfiles"
	"github.com/otwako/novelreader/internal/txt"
)

// ConfirmInboxRemoval removes only the reviewed duplicate, never managed bytes.
// The caller must quiesce acquisition for this claim before review/resolution.
func (s *Store) ConfirmInboxRemoval(ctx context.Context, review *InboxReview) error {
	return s.resolveInbox(ctx, review, true)
}

// ReleaseInbox keeps all files and allows a later scan to acquire the input anew.
// It does not retry acquisition or discard a failed/managed receipt implicitly.
func (s *Store) ReleaseInbox(ctx context.Context, review *InboxReview) error {
	return s.resolveInbox(ctx, review, false)
}

func (s *Store) resolveInbox(ctx context.Context, review *InboxReview, remove bool) error {
	if review == nil || review.owner != s.db || review.claim.ReceiptID == "" {
		return ErrStateChanged
	}
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	claim, value, err := s.settledInboxClaim(ctx, review.claim.ReceiptID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if claim != review.claim {
		return ErrStateChanged
	}
	inbox, err := s.files.OpenInbox()
	if err != nil {
		return err
	}
	defer inbox.Close()
	input, err := inboxfiles.Review(ctx, inbox, claim.Name, txt.MaxInputBytes)
	if err != nil {
		return err
	}
	if !inboxfiles.SameEvidence(review.input, input) {
		return ErrInboxChanged
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if remove {
		if !review.removable || value == nil || value.State == Removing {
			return ErrInboxNotDuplicate
		}
		if err := validateReceiptPath(*value); err != nil {
			return err
		}
		managed, err := s.files.OpenRoot()
		if err != nil {
			return err
		}
		defer managed.Close()
		original, err := inboxfiles.Review(ctx, managed, value.Path, txt.MaxInputBytes)
		if err != nil {
			return err
		}
		if original.Info == nil || original.Info.Size() != value.Size || original.Digest != input.Digest {
			return ErrInboxNotDuplicate
		}
		// No concurrent producers are supported. Recheck the entry after hashing
		// managed bytes so a normal replacement/overwrite invalidates approval.
		current, err := inbox.Lstat(claim.Name)
		if err != nil {
			return err
		}
		if !inboxfiles.SameFile(input.Info, current) {
			return ErrInboxChanged
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := inbox.Remove(claim.Name); err != nil {
			return err
		}
	}
	// Once deletion occurred, request cancellation must not interrupt the short
	// journal cleanup. If it fails, the retained claim still blocks silent replay.
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), metadataTimeout)
	defer cancel()
	return s.clearInboxClaim(finalCtx, claim.ReceiptID)
}
