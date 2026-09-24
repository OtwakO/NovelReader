package txtstore

import (
	"context"
	"errors"
)

// SettleAcquisition reconciles one finished attempt for explicit inbox review.
// The caller must first prove that acquisition for this ID is no longer active.
// Unlike whole-home recovery it never resets live analysis or retries removals,
// and never touches the external inbox. A discarded receipt needs no settlement.
func (s *Store) SettleAcquisition(ctx context.Context, id string) (*Receipt, error) {
	unlock, err := s.files.LockMutation(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	value, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if value.State == Receiving {
		root, err := s.files.OpenRoot()
		if err != nil {
			return nil, err
		}
		err = errors.Join(s.recoverReceipt(ctx, root, id), root.Close())
		if err != nil {
			return nil, err
		}
		value, err = s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	return &value, nil
}
