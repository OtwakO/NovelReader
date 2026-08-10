package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/readerstore"
)

const secondTestUserID readerstore.UserID = "22222222-2222-4222-8222-222222222222"

func TestAccountServiceCreatesNormalizedUniqueAccount(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)

	account, err := accounts.CreateReaderAccount(context.Background(), testUserID, "  Alice  ", "correct horse battery staple", 100)
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != testUserID || account.Username != "Alice" || account.UsernameNormalized != "alice" || account.Role != RoleReader || account.Status != StatusActive {
		t.Fatalf("account = %#v", account)
	}

	_, err = accounts.CreateReaderAccount(context.Background(), secondTestUserID, "ＡＬＩＣＥ", "another secure password", 101)
	if !errors.Is(err, ErrUsernameUnavailable) {
		t.Fatalf("duplicate normalized username error = %v", err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM users`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("account count = %d", count)
	}
}

func TestAccountServiceSeparatesUsernameAndAccountIDConflicts(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	for _, username := range []string{"Bob", "ＡＬＩＣＥ"} {
		_, err := accounts.CreateReaderAccount(context.Background(), testUserID, username, "another secure password", 101)
		if !errors.Is(err, ErrAccountAlreadyExists) {
			t.Fatalf("duplicate account ID with username %q error = %v", username, err)
		}
	}
}

func TestAccountServiceRejectsInvalidCreationBeforeHashing(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	called := false
	accounts.passwords.derive = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		called = true
		return make([]byte, keyLen)
	}

	for _, test := range []struct {
		name     string
		userID   readerstore.UserID
		username string
		want     error
	}{
		{name: "user ID", userID: "not-a-uuid", username: "alice", want: readerstore.ErrInvalidUserID},
		{name: "username", userID: testUserID, username: "a", want: ErrInvalidUsername},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := accounts.CreateReaderAccount(context.Background(), test.userID, test.username, "correct horse battery staple", 100)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if called {
		t.Fatal("invalid account input performed password work")
	}
}

func TestAccountServiceAuthenticatesOnlyActiveAccountsWithGenericFailures(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}

	account, err := accounts.Authenticate(context.Background(), "ＡＬＩＣＥ", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != testUserID || account.Username != "Alice" {
		t.Fatalf("account = %#v", account)
	}

	for _, attempt := range []struct {
		name     string
		username string
		password string
		status   AccountStatus
	}{
		{name: "wrong password", username: "alice", password: "incorrect password", status: StatusActive},
		{name: "unknown username", username: "unknown", password: "incorrect password", status: StatusActive},
		{name: "invalid username", username: "a", password: "incorrect password", status: StatusActive},
		{name: "disabled", username: "alice", password: "correct horse battery staple", status: StatusDisabled},
		{name: "deleting", username: "alice", password: "correct horse battery staple", status: StatusDeleting},
	} {
		t.Run(attempt.name, func(t *testing.T) {
			setTestUserStatus(t, store.db, attempt.status)
			_, err := accounts.Authenticate(context.Background(), attempt.username, attempt.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAccountServiceCredentialFailuresPerformOnePasswordDerivation(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	calls := 0
	accounts.passwords.derive = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		calls++
		return make([]byte, keyLen)
	}

	for _, username := range []string{"unknown", "a"} {
		calls = 0
		_, err := accounts.Authenticate(context.Background(), username, "any candidate password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("username %q error = %v", username, err)
		}
		if calls != 1 {
			t.Fatalf("username %q password derivations = %d", username, calls)
		}
	}
}

func TestAccountServiceConcurrentCanonicalUsernameCreationHasOneWinner(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)

	start := make(chan struct{})
	results := make(chan error, 2)
	for index, candidate := range []struct {
		id       readerstore.UserID
		username string
	}{
		{id: testUserID, username: "Alice"},
		{id: secondTestUserID, username: "ＡＬＩＣＥ"},
	} {
		go func(index int, candidate struct {
			id       readerstore.UserID
			username string
		}) {
			<-start
			_, err := accounts.CreateReaderAccount(context.Background(), candidate.id, candidate.username, "correct horse battery staple", int64(100+index))
			results <- err
		}(index, candidate)
	}
	close(start)

	successes, conflicts := 0, 0
	for range 2 {
		switch err := <-results; {
		case err == nil:
			successes++
		case errors.Is(err, ErrUsernameUnavailable):
			conflicts++
		default:
			t.Fatalf("concurrent creation error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes = %d, conflicts = %d", successes, conflicts)
	}
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM users`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("account count = %d", count)
	}
}

func TestAccountServicePropagatesCanceledAndOverloadedPasswordWork(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := accounts.CreateReaderAccount(ctx, testUserID, "Alice", "correct horse battery staple", 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled creation error = %v", err)
	}
	passwordWorkAdmission <- struct{}{}
	passwordWorkAdmission <- struct{}{}
	defer func() {
		<-passwordWorkAdmission
		<-passwordWorkAdmission
	}()
	if _, err := accounts.Authenticate(context.Background(), "unknown", "candidate password"); !errors.Is(err, ErrPasswordWorkOverloaded) {
		t.Fatalf("overloaded authentication error = %v", err)
	}
}

func TestAccountServiceStoredRecordFailuresAreNotHiddenAsBadCredentials(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	accounts := NewAccountService(store)

	_, err := accounts.Authenticate(context.Background(), "alice", "candidate password")
	if !errors.Is(err, ErrInvalidPasswordHash) {
		t.Fatalf("invalid hash error = %v", err)
	}

	if _, err := store.db.Exec(`PRAGMA ignore_check_constraints = ON`); err != nil {
		t.Fatal(err)
	}
	for column, value := range map[string]string{"role": "owner", "status": "unknown"} {
		if _, err := store.db.Exec(`UPDATE users SET `+column+` = ? WHERE id = ?`, value, string(testUserID)); err != nil {
			t.Fatal(err)
		}
		_, err := accounts.Authenticate(context.Background(), "alice", "candidate password")
		if !errors.Is(err, ErrInvalidAccountRecord) {
			t.Fatalf("invalid stored %s error = %v", column, err)
		}
		insertTestUserRecordValue(t, store, column)
	}
}

func TestAccountServiceChangePasswordRejectsStaleVerifiedCredential(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	verified := make(chan struct{})
	release := make(chan struct{})
	accounts.afterPasswordVerify = func() {
		close(verified)
		<-release
	}
	result := make(chan error, 1)
	go func() {
		result <- accounts.ChangePassword(context.Background(), testUserID, "correct horse battery staple", "stale replacement password", 200)
	}()
	<-verified
	if err := NewAccountService(store).ReplacePassword(context.Background(), testUserID, "authoritative replacement password", 201); err != nil {
		close(release)
		t.Fatal(err)
	}
	close(release)
	if err := <-result; !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("stale change error=%v", err)
	}
	if _, err := NewAccountService(store).Authenticate(context.Background(), "alice", "authoritative replacement password"); err != nil {
		t.Fatalf("authoritative password err=%v", err)
	}
}

func TestAccountServiceReplacesPasswordAndRevokesSessionsAtomically(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	insertTestSession(t, store, "session-1")

	if err := accounts.ReplacePassword(context.Background(), testUserID, "replacement password value", 200); err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.Authenticate(context.Background(), "alice", "correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password error = %v", err)
	}
	if _, err := accounts.Authenticate(context.Background(), "alice", "replacement password value"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	var sessions int
	if err := store.db.QueryRow(`SELECT count(*) FROM auth_sessions WHERE id = 'session-1'`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	var updatedAt int64
	if err := store.db.QueryRow(`SELECT updated_at FROM users WHERE id = ?`, string(testUserID)).Scan(&updatedAt); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || updatedAt != 200 {
		t.Fatalf("sessions = %d, updated_at = %d", sessions, updatedAt)
	}
}

func TestAccountServicePasswordReplacementRollsBackIfRevocationFails(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	insertTestSession(t, store, "session-1")
	if _, err := store.db.Exec(`
		CREATE TRIGGER reject_session_revocation
		BEFORE DELETE ON auth_sessions
		BEGIN SELECT RAISE(ABORT, 'reject revocation'); END
	`); err != nil {
		t.Fatal(err)
	}

	if err := accounts.ReplacePassword(context.Background(), testUserID, "replacement password value", 200); err == nil {
		t.Fatal("replacement succeeded despite failed session revocation")
	}
	if _, err := accounts.Authenticate(context.Background(), "alice", "correct horse battery staple"); err != nil {
		t.Fatalf("original password no longer works: %v", err)
	}
	if _, err := accounts.Authenticate(context.Background(), "alice", "replacement password value"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("replacement password error = %v", err)
	}
}

func TestAccountServicePasswordReplacementChecksAccountState(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)

	if err := accounts.ReplacePassword(context.Background(), testUserID, "replacement password value", 200); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("missing account error = %v", err)
	}
	insertTestUser(t, store.db, StatusDeleting)
	if err := accounts.ReplacePassword(context.Background(), testUserID, "replacement password value", 200); !errors.Is(err, ErrAccountNotActive) {
		t.Fatalf("deleting account error = %v", err)
	}
}

func insertTestUserRecordValue(t *testing.T, store *Store, column string) {
	t.Helper()
	var value string
	switch column {
	case "role":
		value = string(RoleReader)
	case "status":
		value = string(StatusActive)
	case "username":
		value = "Alice"
	case "username_normalized":
		value = "alice"
	default:
		t.Fatalf("unsupported account column %q", column)
	}
	if _, err := store.db.Exec(`UPDATE users SET `+column+` = ? WHERE id = ?`, value, string(testUserID)); err != nil {
		t.Fatal(err)
	}
}

func insertTestSession(t *testing.T, store *Store, id string) {
	t.Helper()
	hash := sha256.Sum256([]byte(id))
	_, err := store.db.Exec(`
		INSERT INTO auth_sessions (id, user_id, token_hash, created_at, last_seen_at)
		VALUES (?, ?, ?, 100, 100)
	`, id, string(testUserID), hash[:])
	if err != nil {
		t.Fatal(err)
	}
}
