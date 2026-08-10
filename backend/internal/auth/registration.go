package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

var ErrRegistrationUnavailable = errors.New("auth: registration is unavailable")

// RegistrationService owns the cross-store creation of an ordinary reader account and home.
type RegistrationService struct {
	accounts   *AccountService
	store      *Store
	readers    *readerstore.Manager
	enabled    bool
	inviteCode string
	randomID   func() (readerstore.UserID, error)
}

func NewRegistrationService(store *Store, readers *readerstore.Manager, enabled bool, inviteCode string) *RegistrationService {
	return &RegistrationService{
		accounts: NewAccountService(store), store: store, readers: readers,
		enabled: enabled, inviteCode: inviteCode, randomID: randomUserID,
	}
}

func (s *RegistrationService) Policy() (enabled, inviteRequired bool) {
	return s != nil && s.enabled, s != nil && s.enabled && s.inviteCode != ""
}

func (s *RegistrationService) Register(ctx context.Context, rawUsername, password, inviteCode string, now time.Time) (Account, error) {
	if s == nil || !s.enabled || s.store == nil || s.readers == nil || !validRegistrationInvite(s.inviteCode, inviteCode) {
		return Account{}, ErrRegistrationUnavailable
	}
	userID, err := s.randomID()
	if err != nil {
		return Account{}, fmt.Errorf("auth: generate reader identity: %w", err)
	}
	account, err := s.accounts.createAccount(ctx, userID, rawUsername, password, RoleReader, StatusDisabled, now.Unix(), 0)
	if errors.Is(err, ErrUsernameUnavailable) {
		account, err = s.resumeReservation(ctx, rawUsername, password)
	}
	if err != nil {
		return Account{}, err
	}
	if err := s.readers.Create(ctx, account.ID); err != nil {
		return Account{}, fmt.Errorf("auth: create registered reader home: %w", err)
	}
	if err := s.store.TransitionAccountStatus(account.ID, StatusActive, now.Unix()); err != nil {
		activated, readErr := accountByID(ctx, s.store.db, account.ID)
		if readErr != nil || activated.Status != StatusActive {
			return Account{}, fmt.Errorf("auth: activate registered reader: %w", err)
		}
		return activated, nil
	}
	account.Status = StatusActive
	account.UpdatedAt = now.Unix()
	account.AuthVersion++
	return account, nil
}

func (s *RegistrationService) resumeReservation(ctx context.Context, rawUsername, password string) (Account, error) {
	username, err := NormalizeUsername(rawUsername)
	if err != nil {
		return Account{}, err
	}
	account, passwordHash, err := s.accounts.accountByNormalizedUsername(ctx, username.Normalized)
	if err != nil || account.Role != RoleReader || account.Status != StatusDisabled || account.UpdatedAt != 0 {
		return Account{}, ErrUsernameUnavailable
	}
	valid, err := s.accounts.passwords.Verify(ctx, password, passwordHash)
	if err != nil {
		return Account{}, err
	}
	if !valid {
		return Account{}, ErrUsernameUnavailable
	}
	return account, nil
}

func validRegistrationInvite(configured, presented string) bool {
	return configured == "" || validEnvironmentToken(configured, presented)
}
