package api

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/epubstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

const (
	inboxControlSlots    = 2
	inboxProofTTL        = 5 * time.Minute
	inboxProofsPerReader = 16
	maxInboxProofs       = 1024
)

var errInboxControlBusy = errors.New("File inbox control capacity is busy")
var errInboxProofLimit = errors.New("finish outstanding inbox reviews before opening more")
var errInboxProofMissing = errors.New("inbox review expired or changed; review again")

type inboxProof struct {
	reader  readerstore.UserID
	review  inboxReview
	expires time.Time
}

// fileInboxControls owns bounded HTTP review state and filesystem-control capacity,
// not file cleanup decisions. Native proofs retain database identity, never a home
// lease. The reader-runtime lifecycle drains control requests before invalidation.
type fileInboxControls struct {
	mu      sync.Mutex
	proofs  map[string]inboxProof
	ioSlots chan struct{}
}

func newInboxControls() *fileInboxControls {
	return &fileInboxControls{proofs: make(map[string]inboxProof), ioSlots: make(chan struct{}, inboxControlSlots)}
}

// Control work must not monopolize foreground runtimes or wait behind uploads.
func (c *fileInboxControls) beginIO() (func(), error) {
	select {
	case c.ioSlots <- struct{}{}:
		return func() { <-c.ioSlots }, nil
	default:
		return nil, errInboxControlBusy
	}
}

func (c *fileInboxControls) retain(reader readerstore.UserID, review inboxReview) (string, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.expireLocked(now)
	count := 0
	for token, proof := range c.proofs {
		if proof.reader == reader {
			// A fresh review supersedes this reader's older approval for the claim.
			if proof.review.format() == review.format() && proof.review.receiptID() == review.receiptID() {
				delete(c.proofs, token)
			} else {
				count++
			}
		}
	}
	if count >= inboxProofsPerReader || len(c.proofs) >= maxInboxProofs {
		return "", time.Time{}, errInboxProofLimit
	}
	token, expiry := rand.Text(), now.Add(inboxProofTTL)
	c.proofs[token] = inboxProof{reader: reader, review: review, expires: expiry}
	return token, expiry, nil
}

// Take consumes only this reader's actual server-owned proof. Resolution errors
// require a new review; client flags and reconstructed objects never authorize IO.
func (c *fileInboxControls) take(reader readerstore.UserID, token, format string) (inboxReview, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireLocked(time.Now())
	proof, found := c.proofs[token]
	if !found || proof.reader != reader || proof.review.format() != format {
		return inboxReview{}, errInboxProofMissing
	}
	delete(c.proofs, token)
	return proof.review, nil
}

func (c *fileInboxControls) invalidate(reader readerstore.UserID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for token, proof := range c.proofs {
		if proof.reader == reader {
			delete(c.proofs, token)
		}
	}
}

func (c *fileInboxControls) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.proofs)
}

func (c *fileInboxControls) expireLocked(now time.Time) {
	for token, proof := range c.proofs {
		if !proof.expires.After(now) {
			delete(c.proofs, token)
		}
	}
}

// Concrete proofs stay in their storage owners; only HTTP token lifetime and
// capacity are shared. No client flags or generic cleanup callbacks live here.
type inboxReview struct {
	txt  *txtstore.InboxReview
	epub *epubstore.InboxReview
}

func (r inboxReview) format() string {
	if r.epub != nil {
		return "epub"
	}
	return "txt"
}
func (r inboxReview) receiptID() string {
	if r.epub != nil {
		return r.epub.Claim.ReceiptID
	}
	return r.txt.Claim.ReceiptID
}
