package api

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

const (
	inboxControlSlots    = 2
	inboxProofTTL        = 5 * time.Minute
	inboxProofsPerReader = 16
	maxInboxProofs       = 1024
)

var errInboxControlBusy = errors.New("TXT inbox control capacity is busy")
var errInboxProofLimit = errors.New("finish outstanding inbox reviews before opening more")
var errInboxProofMissing = errors.New("inbox review expired or changed; review again")

type inboxProof struct {
	reader  readerstore.UserID
	review  *txtstore.InboxReview
	expires time.Time
}

// txtInboxControls owns bounded HTTP review state and filesystem-control capacity,
// not file cleanup decisions. Native proofs retain database identity, never a home
// lease. The reader-runtime lifecycle drains control requests before invalidation.
type txtInboxControls struct {
	mu      sync.Mutex
	proofs  map[string]inboxProof
	ioSlots chan struct{}
}

func newTXTInboxControls() *txtInboxControls {
	return &txtInboxControls{proofs: make(map[string]inboxProof), ioSlots: make(chan struct{}, inboxControlSlots)}
}

// Control work must not monopolize foreground runtimes or wait behind uploads.
func (c *txtInboxControls) beginIO() (func(), error) {
	select {
	case c.ioSlots <- struct{}{}:
		return func() { <-c.ioSlots }, nil
	default:
		return nil, errInboxControlBusy
	}
}

func (c *txtInboxControls) retain(reader readerstore.UserID, review *txtstore.InboxReview) (string, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.expireLocked(now)
	count := 0
	for token, proof := range c.proofs {
		if proof.reader == reader {
			// A fresh review supersedes this reader's older approval for the claim.
			if proof.review.Claim.ReceiptID == review.Claim.ReceiptID {
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
func (c *txtInboxControls) take(reader readerstore.UserID, token string) (*txtstore.InboxReview, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireLocked(time.Now())
	proof, found := c.proofs[token]
	if !found || proof.reader != reader {
		return nil, errInboxProofMissing
	}
	delete(c.proofs, token)
	return proof.review, nil
}

func (c *txtInboxControls) invalidate(reader readerstore.UserID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for token, proof := range c.proofs {
		if proof.reader == reader {
			delete(c.proofs, token)
		}
	}
}

func (c *txtInboxControls) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.proofs)
}

func (c *txtInboxControls) expireLocked(now time.Time) {
	for token, proof := range c.proofs {
		if !proof.expires.After(now) {
			delete(c.proofs, token)
		}
	}
}
