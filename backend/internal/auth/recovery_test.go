package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const adminRecoveryToken = "temporary-recovery-authority"

func preparedRecovery(t *testing.T) (*Store, *readerstore.Manager, Account) {
	t.Helper()
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 4)
	if err != nil {
		store.Close()
		t.Fatal(err)
	}
	setup := NewSetupService(store, readers, setupBootstrapToken)
	setup.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	account, err := setup.CreateInitialAdministrator(context.Background(), setupBootstrapToken, "Alice", "correct horse battery staple", time.Unix(100, 0))
	if err != nil {
		readers.Close()
		store.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { readers.Close(); store.Close() })
	return store, readers, account
}

func TestRecoveryServiceResetsAdministratorReactivatesAndRevokesSessions(t *testing.T) {
	store, readers, admin := preparedRecovery(t)
	sessions := NewSessionService(store)
	oldSession, err := sessions.CreateAuthenticated(context.Background(), admin, 101)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionAccountStatus(admin.ID, StatusDisabled, 102); err != nil {
		t.Fatal(err)
	}
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	recovered, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryResetExisting, "Alice", "replacement password value", time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if recovered.ID != admin.ID || recovered.Status != StatusActive || recovered.AuthVersion <= admin.AuthVersion {
		t.Fatalf("recovered = %#v", recovered)
	}
	if _, err := NewAccountService(store).Authenticate(context.Background(), "Alice", "replacement password value"); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Authenticate(context.Background(), oldSession.Token, 201); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("old session error = %v", err)
	}
}

func TestRecoveryServiceCreatesReplacementWithNewEmptyHome(t *testing.T) {
	store, readers, initial := preparedRecovery(t)
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	service.randomUserID = func() (readerstore.UserID, error) { return secondTestUserID, nil }
	replacement, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryCreateReplacement, "Bob", "replacement administrator password", time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if replacement.ID != secondTestUserID || replacement.Role != RoleAdmin || replacement.Status != StatusActive {
		t.Fatalf("replacement = %#v", replacement)
	}
	for _, userID := range []readerstore.UserID{initial.ID, replacement.ID} {
		home, err := readers.Open(context.Background(), userID)
		if err != nil {
			t.Fatalf("open %s: %v", userID, err)
		}
		home.Close()
	}
	var admins int
	if err := store.db.QueryRow(`SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&admins); err != nil || admins != 2 {
		t.Fatalf("admins=%d error=%v", admins, err)
	}
}

func TestRecoveryServiceRejectsStaleClaimGeneration(t *testing.T) {
	store, readers, _ := preparedRecovery(t)
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	generation := readerstore.UserID("33333333-3333-4333-8333-333333333333")
	service.randomGeneration = func() (readerstore.UserID, error) { return generation, nil }
	service.randomUserID = func() (readerstore.UserID, error) { return secondTestUserID, nil }
	ctx, cancel := context.WithCancel(context.Background())
	service.afterClaim = func(recoveryClaim) { cancel() }
	if _, err := service.Recover(ctx, adminRecoveryToken, RecoveryCreateReplacement, "Bob", "replacement administrator password", time.Unix(200, 0)); !errors.Is(err, context.Canceled) {
		t.Fatalf("interrupted error=%v", err)
	}
	if _, err := store.db.Exec(`UPDATE admin_recovery_state SET generation = ? WHERE id = 1`, "44444444-4444-4444-8444-444444444444"); err != nil {
		t.Fatal(err)
	}
	state, err := service.readState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stale := state.claim
	stale.generation = string(generation)
	if _, err := service.completeClaim(context.Background(), stale, 201); !errors.Is(err, ErrRecoveryInProgress) {
		t.Fatalf("stale completion error=%v", err)
	}
}

func TestRecoveryServiceRejectsPreExistingReplacementHome(t *testing.T) {
	store, readers, _ := preparedRecovery(t)
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	service.randomUserID = func() (readerstore.UserID, error) { return secondTestUserID, nil }
	if _, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryCreateReplacement, "Bob", "replacement administrator password", time.Unix(200, 0)); !errors.Is(err, ErrAccountAlreadyExists) {
		t.Fatalf("pre-existing home error = %v", err)
	}
	var users int
	if err := store.db.QueryRow(`SELECT count(*) FROM users WHERE id = ?`, string(secondTestUserID)).Scan(&users); err != nil || users != 0 {
		t.Fatalf("users=%d error=%v", users, err)
	}
}

func TestRecoveryServiceResumesDurableReplacementClaim(t *testing.T) {
	store, readers, _ := preparedRecovery(t)
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	service.randomUserID = func() (readerstore.UserID, error) { return secondTestUserID, nil }
	ctx, cancel := context.WithCancel(context.Background())
	service.afterClaim = func(recoveryClaim) { cancel() }
	_, err := service.Recover(ctx, adminRecoveryToken, RecoveryCreateReplacement, "Bob", "replacement administrator password", time.Unix(200, 0))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("interrupted error = %v", err)
	}
	service.afterClaim = nil
	replacement, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryCreateReplacement, "Bob", "replacement administrator password", time.Unix(201, 0))
	if err != nil || replacement.ID != secondTestUserID {
		t.Fatalf("replacement=%#v error=%v", replacement, err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM users WHERE id = ?`, string(secondTestUserID)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("replacement count=%d error=%v", count, err)
	}
}

func TestRecoveryServiceReplacementSupersedesClaimedInitialSetup(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	setup := NewSetupService(store, readers, setupBootstrapToken)
	setup.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	username, err := NormalizeUsername("Alice")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := NewPasswordHasher().Hash(context.Background(), "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, created, err := setup.claim(context.Background(), setupClaim{
		userID: testUserID, username: username.Display, usernameNormalized: username.Normalized,
		passwordHash: hash, claimedAt: 100, claimExpiresAt: 100 + int64(defaultSetupClaimTTL.Seconds()),
	}); err != nil || !created {
		t.Fatalf("claim created=%v error=%v", created, err)
	}
	recovery := NewRecoveryService(store, readers, adminRecoveryToken)
	recovery.randomUserID = func() (readerstore.UserID, error) { return secondTestUserID, nil }
	if _, err := recovery.Recover(context.Background(), adminRecoveryToken, RecoveryCreateReplacement, "Bob", "replacement administrator password", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	var status, proposedID string
	if err := store.db.QueryRow(`SELECT status, proposed_user_id FROM setup_state WHERE id = 1`).Scan(&status, &proposedID); err != nil || status != "closed" || proposedID != string(secondTestUserID) {
		t.Fatalf("setup status=%q proposed=%q error=%v", status, proposedID, err)
	}
}

func TestRecoveryServiceCompletedReplayAuthenticatesWithoutRepeatingReset(t *testing.T) {
	store, readers, _ := preparedRecovery(t)
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	first, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryResetExisting, "Alice", "replacement password value", time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryResetExisting, "Alice", "replacement password value", time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != first.ID || replayed.AuthVersion != first.AuthVersion {
		t.Fatalf("first=%#v replayed=%#v", first, replayed)
	}
}

func TestRecoveryServiceCompletedReplayRequiresMatchingAction(t *testing.T) {
	store, readers, _ := preparedRecovery(t)
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	if _, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryResetExisting, "Alice", "replacement password value", time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	service.randomUserID = func() (readerstore.UserID, error) { return secondTestUserID, nil }
	replacement, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryCreateReplacement, "Bob", "replacement administrator password", time.Unix(201, 0))
	if err != nil || replacement.ID != secondTestUserID {
		t.Fatalf("replacement=%#v error=%v", replacement, err)
	}
}

func TestRecoveryServiceRejectsWrongTokenReaderAndDeletingAdministrator(t *testing.T) {
	store, readers, admin := preparedRecovery(t)
	service := NewRecoveryService(store, readers, adminRecoveryToken)
	called := false
	service.passwords.derive = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		called = true
		return make([]byte, keyLen)
	}
	if _, err := service.Recover(context.Background(), "wrong", RecoveryResetExisting, "Alice", "replacement password value", time.Unix(200, 0)); !errors.Is(err, ErrRecoveryUnavailable) || called {
		t.Fatalf("wrong token error=%v passwordWork=%v", err, called)
	}
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	if _, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryResetExisting, "Bob", "replacement password value", time.Unix(201, 0)); !errors.Is(err, ErrRecoveryTarget) {
		t.Fatalf("reader target error=%v", err)
	}
	setTestUserStatus(t, store.db, StatusDeleting)
	if _, err := service.Recover(context.Background(), adminRecoveryToken, RecoveryResetExisting, admin.Username, "replacement password value", time.Unix(202, 0)); !errors.Is(err, ErrRecoveryTarget) {
		t.Fatalf("deleting target error=%v", err)
	}
}

func insertTestUserWithID(t *testing.T, store *Store, id readerstore.UserID, username string, role Role, status AccountStatus) {
	t.Helper()
	hash, err := NewPasswordHasher().Hash(context.Background(), "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := NormalizeUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 100, 100)
	`, string(id), normalized.Display, normalized.Normalized, string(role), hash, string(status)); err != nil {
		t.Fatal(err)
	}
}
