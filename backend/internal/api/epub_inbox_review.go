package api

import (
	"context"
	"net/http"
	"time"

	"github.com/otwako/novelreader/internal/epubstore"
)

// Checking the durable claim first avoids a check-then-start race: its one-use
// grant must have started before the claim exists, and cannot start a second time.
func (s *readerAPI) requireFinishedEPUBInboxAcquisition(ctx context.Context, id string) error {
	if _, err := s.epubStore.GetInboxClaim(ctx, id); err != nil {
		return err
	}
	if s.fileAdmission.Active(s.home.ID(), id) {
		return epubstore.ErrStateChanged
	}
	return nil
}

func (s *readerAPI) handleReviewEPUBInbox(w http.ResponseWriter, r *http.Request) {
	release, err := s.fileInbox.beginIO()
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	defer release()
	id := r.PathValue("id")
	if err := s.requireFinishedEPUBInboxAcquisition(r.Context(), id); err != nil {
		writeEPUBError(w, err)
		return
	}
	value, err := s.epubStore.SettleAcquisition(r.Context(), id)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	var warnings []string
	if value != nil && value.State == epubstore.Acquired {
		if value.PreparationGeneration == 0 {
			_, warnings = queueAcquiredEPUB(r.Context(), s.epubStore, s.fileImports, s.home.ID(), *value)
		} else {
			warnings = wakeEPUBPreparation(s.fileImports, s.home.ID(), id)
		}
	}
	review, err := s.epubStore.ReviewInbox(r.Context(), id)
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	token, expiry, err := s.fileInbox.retain(s.home.ID(), inboxReview{epub: review})
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Token        string    `json:"token"`
		ExpiresAt    time.Time `json:"expiresAt"`
		Name         string    `json:"name"`
		ReceiptID    string    `json:"receiptId"`
		InputPresent bool      `json:"inputPresent"`
		CanRemove    bool      `json:"canRemove"`
		Warnings     []string  `json:"warnings,omitempty"`
	}{token, expiry, review.Claim.Name, review.Claim.ReceiptID, review.InputPresent, review.CanRemove, warnings})
}

func (s *readerAPI) handleConfirmEPUBInbox(w http.ResponseWriter, r *http.Request) {
	s.resolveEPUBInbox(w, r, true)
}

func (s *readerAPI) handleReleaseEPUBInbox(w http.ResponseWriter, r *http.Request) {
	s.resolveEPUBInbox(w, r, false)
}

func (s *readerAPI) resolveEPUBInbox(w http.ResponseWriter, r *http.Request, remove bool) {
	release, err := s.fileInbox.beginIO()
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	defer release()
	proof, err := s.fileInbox.take(s.home.ID(), r.PathValue("token"), "epub")
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	if err := s.requireFinishedEPUBInboxAcquisition(r.Context(), proof.epub.Claim.ReceiptID); err != nil {
		writeEPUBError(w, err)
		return
	}
	if remove {
		err = s.epubStore.ConfirmInboxRemoval(r.Context(), proof.epub)
	} else {
		err = s.epubStore.ReleaseInbox(r.Context(), proof.epub)
	}
	if err != nil {
		writeEPUBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *readerAPI) handleCancelEPUBInboxReview(w http.ResponseWriter, r *http.Request) {
	if _, err := s.fileInbox.take(s.home.ID(), r.PathValue("token"), "epub"); err != nil {
		writeEPUBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
