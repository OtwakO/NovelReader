package api

import (
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/txtstore"
)

func TestInboxHTTPProofsAreBoundedReaderOwnedAndExpiring(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		controls := newTXTInboxControls()
		proof := &txtstore.InboxReview{Claim: txtstore.InboxClaim{ReceiptID: "first"}}
		old, _, err := controls.retain("alice", proof)
		if err != nil {
			t.Fatal(err)
		}
		token, _, err := controls.retain("alice", proof)
		if err != nil || token == old {
			t.Fatalf("fresh review: %v", err)
		}
		if _, err := controls.take("alice", old); !errors.Is(err, errInboxProofMissing) {
			t.Fatalf("superseded proof: %v", err)
		}
		if _, err := controls.take("bob", token); !errors.Is(err, errInboxProofMissing) {
			t.Fatalf("cross-reader proof: %v", err)
		}
		if actual, err := controls.take("alice", token); err != nil || actual != proof {
			t.Fatal("did not retain the original server proof")
		}
		if _, err := controls.take("alice", token); !errors.Is(err, errInboxProofMissing) {
			t.Fatal("proof reused")
		}
		for index := 0; index < maxInboxProofs; index++ {
			if index == inboxProofsPerReader {
				if _, _, err := controls.retain("reader-0", proof); !errors.Is(err, errInboxProofLimit) {
					t.Fatal("reader limit missing")
				}
			}
			reader := readerstore.UserID(fmt.Sprintf("reader-%d", index/inboxProofsPerReader))
			_, _, err := controls.retain(reader, &txtstore.InboxReview{Claim: txtstore.InboxClaim{ReceiptID: fmt.Sprint(index)}})
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := controls.retain("new-reader", proof); !errors.Is(err, errInboxProofLimit) {
			t.Fatal("global limit missing")
		}
		controls.invalidate("reader-0")
		token, _, err = controls.retain("new-reader", proof)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(inboxProofTTL)
		if _, err := controls.take("new-reader", token); !errors.Is(err, errInboxProofMissing) {
			t.Fatalf("expired proof: %v", err)
		}
		if len(controls.proofs) != 0 {
			t.Fatal("expired proof state retained")
		}
	})
}

func TestInboxControlIOHasIndependentBound(t *testing.T) {
	controls := newTXTInboxControls()
	first, err := controls.beginIO()
	if err != nil {
		t.Fatal(err)
	}
	second, err := controls.beginIO()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controls.beginIO(); !errors.Is(err, errInboxControlBusy) {
		t.Fatal("unbounded control IO")
	}
	first()
	next, err := controls.beginIO()
	if err != nil {
		t.Fatal(err)
	}
	next()
	second()
}
