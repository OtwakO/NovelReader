package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestSessionServiceCreatesIndependentHashOnlySessions(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	sessions := NewSessionService(store)

	first, err := sessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sessions.Create(context.Background(), testUserID, 101)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Token == second.Token {
		t.Fatalf("sessions are not independent: %#v %#v", first, second)
	}
	for _, created := range []SessionCredential{first, second} {
		raw, err := base64.RawURLEncoding.Strict().DecodeString(created.Token)
		if err != nil || len(raw) != sessionTokenBytes {
			t.Fatalf("token %q decoded length = %d, error = %v", created.Token, len(raw), err)
		}
		var stored []byte
		if err := store.db.QueryRow(`SELECT token_hash FROM auth_sessions WHERE id = ?`, created.ID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(created.Token))
		if string(stored) != string(hash[:]) {
			t.Fatal("stored token hash does not match")
		}
		var rawCount int
		if err := store.db.QueryRow(`SELECT count(*) FROM auth_sessions WHERE token_hash = ?`, []byte(created.Token)).Scan(&rawCount); err != nil {
			t.Fatal(err)
		}
		if rawCount != 0 {
			t.Fatal("raw token was persisted")
		}
	}
}

func TestSessionServiceRejectsStaleAuthenticatedAccountAfterPasswordReplacement(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	authenticated, err := accounts.Authenticate(context.Background(), "Alice", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.ReplacePassword(context.Background(), testUserID, "replacement password value", 200); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSessionService(store).CreateAuthenticated(context.Background(), authenticated, 201); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("stale credential session error = %v", err)
	}
}

func TestSessionServiceRequiresActiveAccountWhenCreating(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	sessions := NewSessionService(store)

	if _, err := sessions.Create(context.Background(), testUserID, 100); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("missing account error = %v", err)
	}
	insertTestUser(t, store.db, StatusDisabled)
	if _, err := sessions.Create(context.Background(), testUserID, 100); !errors.Is(err, ErrAccountNotActive) {
		t.Fatalf("disabled account error = %v", err)
	}
}

func TestSessionServiceAuthenticatesActiveSessionAndUpdatesActivity(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	sessions := NewSessionService(store)
	credential, err := sessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}

	account, err := sessions.Authenticate(context.Background(), credential.Token, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != testUserID || account.Username != "Alice" || account.Role != RoleReader || account.Status != StatusActive {
		t.Fatalf("account = %#v", account)
	}
	var lastSeen int64
	if err := store.db.QueryRow(`SELECT last_seen_at FROM auth_sessions WHERE id = ?`, credential.ID).Scan(&lastSeen); err != nil {
		t.Fatal(err)
	}
	if lastSeen != 4000 {
		t.Fatalf("last_seen_at = %d", lastSeen)
	}
	if _, err := sessions.Authenticate(context.Background(), credential.Token, 200); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT last_seen_at FROM auth_sessions WHERE id = ?`, credential.ID).Scan(&lastSeen); err != nil {
		t.Fatal(err)
	}
	if lastSeen != 4000 {
		t.Fatalf("last_seen_at moved backward to %d", lastSeen)
	}
}

func TestSessionServiceRejectsInvalidUnknownAndInactiveSessionsGenerically(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	sessions := NewSessionService(store)
	credential, err := sessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}
	unknownRaw := make([]byte, sessionTokenBytes)
	unknownRaw[0] = 1
	unknown := base64.RawURLEncoding.EncodeToString(unknownRaw)

	for _, token := range []string{"", "not base64!", base64.RawURLEncoding.EncodeToString(make([]byte, sessionTokenBytes-1)), unknown} {
		if _, err := sessions.Authenticate(context.Background(), token, 200); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("token %q error = %v", token, err)
		}
	}
	for _, status := range []AccountStatus{StatusDisabled, StatusDeleting} {
		setTestUserStatus(t, store.db, status)
		if _, err := sessions.Authenticate(context.Background(), credential.Token, 200); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("status %q error = %v", status, err)
		}
	}
}

func TestSessionServiceStoredIdentityCorruptionIsNotHidden(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	sessions := NewSessionService(store)
	credential, err := sessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`PRAGMA ignore_check_constraints = ON`); err != nil {
		t.Fatal(err)
	}
	for column, value := range map[string]string{
		"role":                "owner",
		"status":              "unknown",
		"username":            " Alice ",
		"username_normalized": "ALICE",
	} {
		if _, err := store.db.Exec(`UPDATE users SET `+column+` = ? WHERE id = ?`, value, string(testUserID)); err != nil {
			t.Fatal(err)
		}
		if _, err := sessions.Authenticate(context.Background(), credential.Token, 200); !errors.Is(err, ErrInvalidAccountRecord) {
			t.Fatalf("corrupt %s error = %v", column, err)
		}
		insertTestUserRecordValue(t, store, column)
	}
}

func TestSessionServiceLogoutOneAndAllDevices(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	sessions := NewSessionService(store)
	first, err := sessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sessions.Create(context.Background(), testUserID, 101)
	if err != nil {
		t.Fatal(err)
	}

	if err := sessions.Logout(context.Background(), first.Token); err != nil {
		t.Fatal(err)
	}
	if err := sessions.Logout(context.Background(), first.Token); err != nil {
		t.Fatalf("repeated logout: %v", err)
	}
	if err := sessions.Logout(context.Background(), "malformed"); err != nil {
		t.Fatalf("malformed logout: %v", err)
	}
	if _, err := sessions.Authenticate(context.Background(), first.Token, 200); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("logged-out first session error = %v", err)
	}
	if _, err := sessions.Authenticate(context.Background(), second.Token, 200); err != nil {
		t.Fatalf("second device was disturbed: %v", err)
	}

	if err := sessions.LogoutAll(context.Background(), testUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Authenticate(context.Background(), second.Token, 200); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("second session after logout-all error = %v", err)
	}
}

func TestSessionServiceConcurrentAuthenticationAndLogoutFailsClosedAfterLogout(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	sessions := NewSessionService(store)
	credential, err := sessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	var wait sync.WaitGroup
	results := make(chan error, 9)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := sessions.Authenticate(context.Background(), credential.Token, 200)
			results <- err
		}()
	}
	wait.Add(1)
	go func() {
		defer wait.Done()
		<-start
		results <- sessions.Logout(context.Background(), credential.Token)
	}()
	close(start)
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil && !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("concurrent result = %v", err)
		}
	}
	if _, err := sessions.Authenticate(context.Background(), credential.Token, 201); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("post-logout authentication error = %v", err)
	}
}

func TestSessionGuardGivesQueuedWriterPriority(t *testing.T) {
	guard := newSharedSessionGuard()
	if err := guard.readLock(context.Background()); err != nil {
		t.Fatal(err)
	}
	writerAcquired := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		if err := guard.lock(context.Background()); err != nil {
			return
		}
		close(writerAcquired)
		<-writerDone
		guard.unlock()
	}()
	deadline := time.Now().Add(time.Second)
	for {
		guard.mutex.Lock()
		waiting := guard.waitingWriters
		guard.mutex.Unlock()
		if waiting == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("writer did not queue")
		}
		time.Sleep(time.Millisecond)
	}
	readerCtx, cancelReader := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelReader()
	if err := guard.readLock(readerCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reader bypassed queued writer: %v", err)
	}
	guard.readUnlock()
	select {
	case <-writerAcquired:
	case <-time.After(time.Second):
		t.Fatal("queued writer did not acquire after readers exited")
	}
	close(writerDone)
}

func TestSessionServiceSharesRevocationBarrierAcrossStoreInstances(t *testing.T) {
	root := prepareTestRoot(t)
	firstStore, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer firstStore.Close()
	secondStore, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer secondStore.Close()
	insertTestUser(t, firstStore.db, StatusActive)
	firstSessions := NewSessionService(firstStore)
	secondSessions := NewSessionService(secondStore)
	credential, err := firstSessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}

	lookedUp := make(chan struct{})
	release := make(chan struct{})
	firstSessions.afterLookup = func() {
		close(lookedUp)
		<-release
	}
	authenticated := make(chan error, 1)
	go func() {
		_, err := firstSessions.Authenticate(context.Background(), credential.Token, 200)
		authenticated <- err
	}()
	<-lookedUp

	loggedOut := make(chan error, 1)
	go func() { loggedOut <- secondSessions.Logout(context.Background(), credential.Token) }()
	select {
	case err := <-loggedOut:
		t.Fatalf("logout completed before in-flight authentication released: %v", err)
	default:
	}
	close(release)
	if err := <-authenticated; err != nil {
		t.Fatalf("in-flight authentication error = %v", err)
	}
	if err := <-loggedOut; err != nil {
		t.Fatal(err)
	}
	if _, err := firstSessions.Authenticate(context.Background(), credential.Token, 201); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("post-revocation authentication error = %v", err)
	}
}

func TestSessionServiceSharesBarrierAcrossAliasPathsAndCloseReopen(t *testing.T) {
	root := prepareTestRoot(t)
	aliasParent := t.TempDir()
	parentAlias := filepath.Join(aliasParent, "parent-alias")
	if err := os.Symlink(filepath.Dir(root), parentAlias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Fatal(err)
	}
	alias := filepath.Join(parentAlias, filepath.Base(root))
	firstStore, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	insertTestUser(t, firstStore.db, StatusActive)
	firstSessions := NewSessionService(firstStore)
	credential, err := firstSessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}
	guard := firstStore.sessionGuard
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}

	aliasStore, err := OpenSystemStore(alias)
	if err != nil {
		t.Fatal(err)
	}
	defer aliasStore.Close()
	if aliasStore.sessionGuard != guard {
		t.Fatal("alias/reopened store acquired a different session guard")
	}
	if _, err := NewSessionService(aliasStore).Authenticate(context.Background(), credential.Token, 200); err != nil {
		t.Fatal(err)
	}
}

func TestSessionServiceHonorsCanceledContext(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	sessions := NewSessionService(store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := sessions.Create(ctx, testUserID, 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("create error = %v", err)
	}
	if _, err := sessions.Authenticate(ctx, base64.RawURLEncoding.EncodeToString(make([]byte, sessionTokenBytes)), 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("authenticate error = %v", err)
	}
}
