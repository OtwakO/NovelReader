package candidate

import "time"

// transitionMu serializes write eligibility with cancellation/expiry. Snapshot
// reads use mu independently, so a slow shelf write does not block observation.
func (op *operation) acceptCandidate(selected int, value *resolved) bool {
	op.transitionMu.Lock()
	defer op.transitionMu.Unlock()
	if op.current().State != StateRunning {
		return false
	}
	preview := previewFromResolved(value)
	op.update(func(s *Snapshot) {
		op.resolved = value
		for i := range s.Attempts {
			if i == selected {
				s.Attempts[i].State = "verified"
			} else if s.Attempts[i].State == "queued" || s.Attempts[i].State == "ready" {
				s.Attempts[i].State = "skipped"
			}
		}
		s.State = StateVerified
		s.Preview = &preview
		s.Message = "book metadata verified"
	})
	if op.commitID != "" {
		if _, err := op.commitLocked(op.commitID); err != nil {
			op.update(func(s *Snapshot) {
				s.State = StateFailed
				s.CommitPending = true
				s.Message = "verified candidate could not be added to the shelf"
			})
			op.markTerminal()
		}
	}
	return true
}

func (op *operation) cancelResolution() {
	op.transitionMu.Lock()
	defer op.transitionMu.Unlock()
	snapshot := op.current()
	if snapshot.State != StateRunning && snapshot.State != StateVerified && !snapshot.CommitPending {
		return
	}
	op.update(func(s *Snapshot) { s.CommitPending = false })
	op.finish(StateCancelled, "candidate resolution was cancelled")
	op.cancel()
	select {
	case <-op.done:
		op.release()
	default: // run owns release until all source attempts drain.
	}
}

func (op *operation) finishRunning(state State, message string) {
	op.transitionMu.Lock()
	defer op.transitionMu.Unlock()
	if op.current().State == StateRunning {
		op.finish(state, message)
	}
}

func (op *operation) commit(bookID string) (Snapshot, error) {
	op.transitionMu.Lock()
	defer op.transitionMu.Unlock()
	return op.commitLocked(bookID)
}

func (op *operation) commitLocked(bookID string) (Snapshot, error) {
	if bookID == "" {
		return op.current(), ErrInvalidBookID
	}
	snapshot := op.current()
	if op.commitResult != nil {
		return snapshot, nil
	}
	if snapshot.State != StateVerified && !(snapshot.State == StateFailed && snapshot.CommitPending) {
		return snapshot, errNotVerified
	}
	candidate := *op.resolved.book
	candidate.ID = bookID
	stored, created, err := op.runtime.Books.AddOrMergeBook(&candidate)
	if err != nil {
		return op.current(), err
	}
	op.commitResult = stored
	op.update(func(s *Snapshot) {
		s.State = StateCommitted
		s.StoredBook = stored
		s.Created = created
		s.CommitPending = false
		s.Message = "book added to shelf"
	})
	op.markTerminal()
	return op.current(), nil
}

func (op *operation) expirePending() {
	timer := time.NewTimer(op.policy.Retention)
	defer timer.Stop()
	<-timer.C
	op.transitionMu.Lock()
	defer op.transitionMu.Unlock()
	snapshot := op.current()
	if snapshot.State == StateVerified || snapshot.CommitPending {
		op.update(func(s *Snapshot) {
			s.CommitPending = false
			s.State = StateFailed
			s.Message = "verified candidate expired before it was added to the shelf"
		})
		op.markTerminal()
		op.release()
	}
}
