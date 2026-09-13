package txtimport

import (
	"context"
	"crypto/rand"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

// Transfers is independent of analysis workers and foreground API capacity.
// Composition must budget this many additional active reader-home leases.
const Transfers = 2

const (
	maxAdmissionTickets = 1024
	waitingTicketTTL    = 2 * time.Minute
	grantedTicketTTL    = 30 * time.Second
	transferTimeout     = 30 * time.Minute
)

var (
	ErrAdmissionFull  = errors.New("txtimport: intake queue full")
	ErrTicketNotFound = errors.New("txtimport: intake ticket missing or expired")
	ErrTicketNotReady = errors.New("txtimport: intake ticket is not an unused grant")
)

type TicketState string

const (
	TicketWaiting      TicketState = "waiting"
	TicketGranted      TicketState = "granted"
	TicketTransferring TicketState = "transferring"
)

// AdmissionTicket describes permission to start one transfer, not acquisition.
// It contains no file identity, receipt, path, or inbox-cleanup authority.
type AdmissionTicket struct {
	ID        string
	State     TicketState
	ExpiresAt time.Time
}

type admissionEntry struct {
	reader readerstore.UserID
	ticket AdmissionTicket
	cancel context.CancelFunc
	done   chan struct{}
}

// Admission owns reader-fair, bounded transfer admission. Waiting clients request
// metadata only; callers must Begin before consuming file bytes or opening a home.
// Tickets are process-local. No timer, file content or home lease is kept per waiter.
// HTTP, file acquisition and analysis remain outside this scheduling owner.
type Admission struct {
	mu      sync.Mutex
	entries map[readerstore.UserID]*admissionEntry
	queue   []*admissionEntry
	paused  map[readerstore.UserID]bool
	closed  bool
	active  sync.WaitGroup
}

func NewAdmission() *Admission {
	return &Admission{entries: make(map[readerstore.UserID]*admissionEntry), paused: make(map[readerstore.UserID]bool)}
}

// Request creates at most one ticket per reader. Repeated requests return the
// existing ticket; they neither jump the queue nor extend an unused grant.
func (a *Admission) Request(id readerstore.UserID) (AdmissionTicket, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.availableLocked(id); err != nil {
		return AdmissionTicket{}, err
	}
	now := time.Now()
	a.advanceLocked(now)
	entry := a.entries[id]
	if entry == nil {
		if len(a.entries) >= maxAdmissionTickets {
			return AdmissionTicket{}, ErrAdmissionFull
		}
		entry = &admissionEntry{reader: id, ticket: AdmissionTicket{ID: rand.Text(), State: TicketWaiting}}
		entry.ticket.ExpiresAt = now.Add(waitingTicketTTL)
		a.entries[id] = entry
		a.queue = append(a.queue, entry)
		a.advanceLocked(now)
	} else if entry.ticket.State == TicketWaiting {
		entry.ticket.ExpiresAt = now.Add(waitingTicketTTL)
	}
	return entry.ticket, nil
}

// Status refreshes a live waiter, never resurrecting expired or replaced tickets.
// Clients need only one status request per waiting reader, not one per file.
func (a *Admission) Status(id readerstore.UserID, ticketID string) (AdmissionTicket, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.availableLocked(id); err != nil {
		return AdmissionTicket{}, err
	}
	now := time.Now()
	a.advanceLocked(now)
	entry := a.entries[id]
	if entry == nil || entry.ticket.ID != ticketID {
		return AdmissionTicket{}, ErrTicketNotFound
	}
	if entry.ticket.State == TicketWaiting {
		entry.ticket.ExpiresAt = now.Add(waitingTicketTTL)
	}
	return entry.ticket, nil
}

func (a *Admission) availableLocked(id readerstore.UserID) error {
	if a.closed {
		return ErrClosed
	}
	if a.paused[id] {
		return ErrPaused
	}
	return nil
}

// Expiration and promotion need no background scheduler: a waiting client must
// contact us to use a grant anyway. Active transfers retire only after cleanup.
func (a *Admission) advanceLocked(now time.Time) {
	occupied := 0
	for id, entry := range a.entries {
		if entry.ticket.State != TicketTransferring && !entry.ticket.ExpiresAt.After(now) {
			delete(a.entries, id)
		} else if entry.ticket.State != TicketWaiting {
			occupied++
		}
	}
	a.queue = slices.DeleteFunc(a.queue, func(entry *admissionEntry) bool { return a.entries[entry.reader] != entry })
	for occupied < Transfers && len(a.queue) > 0 {
		entry := a.queue[0]
		a.queue[0] = nil
		a.queue = a.queue[1:]
		entry.ticket.State = TicketGranted
		entry.ticket.ExpiresAt = now.Add(grantedTicketTTL)
		occupied++
	}
	if len(a.queue) == 0 {
		a.queue = nil
	}
}
