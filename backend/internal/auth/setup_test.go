package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const setupBootstrapToken = "temporary-bootstrap-authority"

func TestSetupServiceCreatesOnlyInitialAdministratorAndClosesSetup(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	service := NewSetupService(store, readers, setupBootstrapToken)
	service.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }

	account, err := service.CreateInitialAdministrator(context.Background(), setupBootstrapToken, "  Alice  ", "correct horse battery staple", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != testUserID || account.Username != "Alice" || account.Role != RoleAdmin || account.Status != StatusActive {
		t.Fatalf("account=%#v", account)
	}
	home, err := readers.Open(context.Background(), testUserID)
	if err != nil {
		t.Fatal(err)
	}
	_ = home.Close()

	var status string
	var username, passwordHash *string
	if err := store.db.QueryRow(`SELECT status, username, password_hash FROM setup_state WHERE id = 1`).Scan(&status, &username, &passwordHash); err != nil {
		t.Fatal(err)
	}
	if status != "closed" || username != nil || passwordHash != nil {
		t.Fatalf("closed setup state status=%q username=%v passwordHash=%v", status, username, passwordHash)
	}
	if _, err := service.CreateInitialAdministrator(context.Background(), setupBootstrapToken, "Bob", "another secure password", time.Unix(101, 0)); !errors.Is(err, ErrSetupClosed) {
		t.Fatalf("second setup error=%v", err)
	}
}

func TestSetupServiceRejectsMissingOrWrongBootstrapTokenBeforePasswordWork(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()

	for _, configured := range []string{"", setupBootstrapToken} {
		service := NewSetupService(store, readers, configured)
		called := false
		service.passwords.derive = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
			called = true
			return make([]byte, keyLen)
		}
		_, err := service.CreateInitialAdministrator(context.Background(), "wrong-token", "Alice", "correct horse battery staple", time.Unix(100, 0))
		if !errors.Is(err, ErrSetupUnavailable) {
			t.Fatalf("configured=%q error=%v", configured, err)
		}
		if called {
			t.Fatalf("configured=%q performed password work", configured)
		}
	}
}

func TestSetupServiceRecoversExpiredDurableClaimUnderBootstrapAuthority(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	service := NewSetupService(store, readers, setupBootstrapToken)
	claim := setupClaim{
		userID:             testUserID,
		username:           "Alice",
		usernameNormalized: "alice",
		passwordHash:       dummyPasswordHash,
		claimedAt:          100,
		claimExpiresAt:     200,
	}
	if _, created, err := service.claim(context.Background(), claim); err != nil || !created {
		t.Fatalf("claim created=%t error=%v", created, err)
	}
	account, err := service.RecoverInitialAdministrator(context.Background(), setupBootstrapToken, time.Unix(500, 0))
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != testUserID || account.Role != RoleAdmin {
		t.Fatalf("account=%#v", account)
	}
}

func TestSetupServiceRejectsInvalidProposedClaimBeforePersistence(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	service := NewSetupService(store, readers, setupBootstrapToken)
	service.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	if _, err := service.CreateInitialAdministrator(context.Background(), setupBootstrapToken, "Alice", "correct horse battery staple", time.Unix(-10, 0)); !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("negative-time setup error=%v", err)
	}
	var status string
	if err := store.db.QueryRow(`SELECT status FROM setup_state WHERE id = 1`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "open" {
		t.Fatalf("setup status=%q", status)
	}
	if _, err := readers.Open(context.Background(), testUserID); !errors.Is(err, readerstore.ErrHomeNotFound) {
		t.Fatalf("invalid proposal created home: %v", err)
	}
}

func TestSetupServiceRejectsMalformedDurableClaimBeforeCreatingHome(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	if _, err := store.db.Exec(`
		UPDATE setup_state SET status = 'claimed', proposed_user_id = ?, username = 'Alice',
		username_normalized = 'wrong', password_hash = ?, claimed_at = 100, claim_expires_at = 200
		WHERE id = 1
	`, string(testUserID), dummyPasswordHash); err != nil {
		t.Fatal(err)
	}
	service := NewSetupService(store, readers, setupBootstrapToken)
	if _, err := service.RecoverInitialAdministrator(context.Background(), setupBootstrapToken, time.Unix(150, 0)); !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("recovery error=%v", err)
	}
	if _, err := readers.Open(context.Background(), testUserID); !errors.Is(err, readerstore.ErrHomeNotFound) {
		t.Fatalf("malformed claim created home: %v", err)
	}
}

func TestSetupServiceRejectsRecoveryWhenClaimAndAccountRowsConflict(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	service := NewSetupService(store, readers, setupBootstrapToken)
	claim := setupClaim{
		userID:             testUserID,
		username:           "Alice",
		usernameNormalized: "alice",
		passwordHash:       dummyPasswordHash,
		claimedAt:          100,
		claimExpiresAt:     1000,
	}
	if _, created, err := service.claim(context.Background(), claim); err != nil || !created {
		t.Fatalf("claim created=%t error=%v", created, err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
		VALUES (?, 'Bob', 'bob', 'reader', ?, 'active', 100, 100)
	`, string(secondTestUserID), dummyPasswordHash); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecoverInitialAdministrator(context.Background(), setupBootstrapToken, time.Unix(200, 0)); !errors.Is(err, ErrInvalidSystemSchema) {
		t.Fatalf("conflicting recovery error=%v", err)
	}
	var administrators int
	if err := store.db.QueryRow(`SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&administrators); err != nil {
		t.Fatal(err)
	}
	if administrators != 0 {
		t.Fatalf("administrator count=%d", administrators)
	}
}

func TestSetupServiceRecoversDurableClaimAfterReaderHomePublication(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	service := NewSetupService(store, readers, setupBootstrapToken)
	claim := setupClaim{
		userID:             testUserID,
		username:           "Alice",
		usernameNormalized: "alice",
		passwordHash:       dummyPasswordHash,
		claimedAt:          100,
		claimExpiresAt:     1000,
	}
	if _, created, err := service.claim(context.Background(), claim); err != nil || !created {
		t.Fatalf("claim created=%t error=%v", created, err)
	}
	if err := readers.Create(context.Background(), testUserID); err != nil {
		t.Fatal(err)
	}

	account, err := service.RecoverInitialAdministrator(context.Background(), setupBootstrapToken, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != testUserID || account.Role != RoleAdmin {
		t.Fatalf("account=%#v", account)
	}
	stored, err := accountByID(context.Background(), store.db, testUserID)
	if err != nil || stored.ID != testUserID || stored.Role != RoleAdmin {
		t.Fatalf("stored account=%#v error=%v", stored, err)
	}
}

func TestSetupServiceConcurrentStoresHaveOneWinner(t *testing.T) {
	root := prepareTestRoot(t)
	stores := make([]*Store, 2)
	readers := make([]*readerstore.Manager, 2)
	for index := range stores {
		var err error
		stores[index], err = OpenSystemStore(root)
		if err != nil {
			t.Fatal(err)
		}
		defer stores[index].Close()
		readers[index], err = readerstore.NewManager(root, 1)
		if err != nil {
			t.Fatal(err)
		}
		defer readers[index].Close()
	}

	ids := []readerstore.UserID{testUserID, secondTestUserID}
	start := make(chan struct{})
	results := make(chan error, 2)
	for index := range stores {
		go func(index int) {
			service := NewSetupService(stores[index], readers[index], setupBootstrapToken)
			service.randomUserID = func() (readerstore.UserID, error) { return ids[index], nil }
			<-start
			_, err := service.CreateInitialAdministrator(context.Background(), setupBootstrapToken, []string{"Alice", "Bob"}[index], "correct horse battery staple", time.Unix(int64(100+index), 0))
			results <- err
		}(index)
	}
	close(start)
	successes, rejected := 0, 0
	for range stores {
		switch err := <-results; {
		case err == nil:
			successes++
		case errors.Is(err, ErrSetupClosed), errors.Is(err, ErrSetupInProgress):
			rejected++
		default:
			t.Fatalf("concurrent store setup error=%v", err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("successes=%d rejected=%d", successes, rejected)
	}
	var administrators int
	if err := stores[0].db.QueryRow(`SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&administrators); err != nil {
		t.Fatal(err)
	}
	if administrators != 1 {
		t.Fatalf("administrator count=%d", administrators)
	}
}

func TestSetupServiceConcurrentClaimsHaveOneWinner(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()

	ids := []readerstore.UserID{testUserID, secondTestUserID}
	start := make(chan struct{})
	results := make(chan error, len(ids))
	var wait sync.WaitGroup
	for index, id := range ids {
		wait.Add(1)
		go func(index int, id readerstore.UserID) {
			defer wait.Done()
			service := NewSetupService(store, readers, setupBootstrapToken)
			service.randomUserID = func() (readerstore.UserID, error) { return id, nil }
			<-start
			_, err := service.CreateInitialAdministrator(context.Background(), setupBootstrapToken, []string{"Alice", "Bob"}[index], "correct horse battery staple", time.Unix(int64(100+index), 0))
			results <- err
		}(index, id)
	}
	close(start)
	wait.Wait()
	close(results)

	successes, rejected := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrSetupInProgress), errors.Is(err, ErrSetupClosed):
			rejected++
		default:
			t.Fatalf("concurrent setup error=%v", err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatalf("successes=%d rejected=%d", successes, rejected)
	}
	var administrators int
	if err := store.db.QueryRow(`SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&administrators); err != nil {
		t.Fatal(err)
	}
	if administrators != 1 {
		t.Fatalf("administrator count=%d", administrators)
	}
}

func TestSetupServiceDoesNotPersistBootstrapToken(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	service := NewSetupService(store, readers, setupBootstrapToken)
	service.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	if _, err := service.CreateInitialAdministrator(context.Background(), setupBootstrapToken, "Alice", "correct horse battery staple", time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), setupBootstrapToken) {
		t.Fatal("bootstrap token appears in system database")
	}
}
