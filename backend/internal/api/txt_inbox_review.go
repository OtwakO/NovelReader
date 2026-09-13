package api

import (
	"context"
	"net/http"
	"time"

	"github.com/otwako/novelreader/internal/txtstore"
)

// Checking the durable claim first avoids a check-then-start race: its one-use
// grant must have started before the claim exists, and cannot start a second time.
func (s *readerAPI) requireFinishedInboxAcquisition(ctx context.Context, id string) error {
	if _, err := s.txtStore.GetInboxClaim(ctx, id); err != nil {
		return err
	}
	if s.txtAdmission.Active(s.home.ID(), id) {
		return txtstore.ErrStateChanged
	}
	return nil
}

func (s *readerAPI) handleReviewTXTInbox(w http.ResponseWriter, r *http.Request) {
	release, err := s.txtInbox.beginIO()
	if err != nil {
		writeTXTError(w, err)
		return
	}
	defer release()
	id := r.PathValue("id")
	if err := s.requireFinishedInboxAcquisition(r.Context(), id); err != nil {
		writeTXTError(w, err)
		return
	}
	value, err := s.txtStore.SettleAcquisition(r.Context(), id)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	var warnings []string
	if value != nil && value.State == txtstore.Received {
		warnings = wakeTXTAnalysis(s.txtImports, s.home.ID(), id)
	}
	review, err := s.txtStore.ReviewInbox(r.Context(), id)
	if err != nil {
		writeTXTError(w, err)
		return
	}
	token, expiry, err := s.txtInbox.retain(s.home.ID(), review)
	if err != nil {
		writeTXTError(w, err)
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

func (s *readerAPI) handleConfirmTXTInbox(w http.ResponseWriter, r *http.Request) {
	s.resolveTXTInbox(w, r, true)
}

func (s *readerAPI) handleReleaseTXTInbox(w http.ResponseWriter, r *http.Request) {
	s.resolveTXTInbox(w, r, false)
}

func (s *readerAPI) resolveTXTInbox(w http.ResponseWriter, r *http.Request, remove bool) {
	release, err := s.txtInbox.beginIO()
	if err != nil {
		writeTXTError(w, err)
		return
	}
	defer release()
	proof, err := s.txtInbox.take(s.home.ID(), r.PathValue("token"))
	if err != nil {
		writeTXTError(w, err)
		return
	}
	if err := s.requireFinishedInboxAcquisition(r.Context(), proof.Claim.ReceiptID); err != nil {
		writeTXTError(w, err)
		return
	}
	if remove {
		err = s.txtStore.ConfirmInboxRemoval(r.Context(), proof)
	} else {
		err = s.txtStore.ReleaseInbox(r.Context(), proof)
	}
	if err != nil {
		writeTXTError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *readerAPI) handleCancelTXTInboxReview(w http.ResponseWriter, r *http.Request) {
	if _, err := s.txtInbox.take(s.home.ID(), r.PathValue("token")); err != nil {
		writeTXTError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
