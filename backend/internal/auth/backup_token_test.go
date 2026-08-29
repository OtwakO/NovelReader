package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBackupTokensAreHashOnlyScopedAndReaderOwned(t *testing.T) {
	store := openTestStore(t)
	insertTestUser(t, store.db, StatusActive)
	userID := testUserID
	service := NewBackupTokenService(store)
	service.now = func() time.Time { return time.Unix(100, 0) }

	credential, err := service.Create(context.Background(), userID, CreateBackupToken{Name: "Nightly export", CanExport: true})
	if err != nil {
		t.Fatal(err)
	}
	if credential.Token == "" || !validBackupToken(credential.Token) {
		t.Fatalf("credential=%#v", credential)
	}
	var stored string
	if err := store.db.QueryRow(`SELECT CAST(token_hash AS TEXT) FROM backup_tokens WHERE id = ?`, credential.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == credential.Token {
		t.Fatal("backup token stored in plaintext")
	}
	account, err := service.Authenticate(context.Background(), credential.Token, BackupExport)
	if err != nil || account.ID != userID {
		t.Fatalf("account=%#v error=%v", account, err)
	}
	if _, err := service.Authenticate(context.Background(), credential.Token, BackupRestore); !errors.Is(err, ErrInvalidBackupToken) {
		t.Fatalf("restore scope error=%v", err)
	}
	tokens, err := service.List(context.Background(), userID)
	if err != nil || len(tokens) != 1 || tokens[0].LastUsedAt == nil || *tokens[0].LastUsedAt != 100 {
		t.Fatalf("tokens=%#v error=%v", tokens, err)
	}
	if err := service.Revoke(context.Background(), userID, credential.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), credential.Token, BackupExport); !errors.Is(err, ErrInvalidBackupToken) {
		t.Fatalf("revoked token error=%v", err)
	}
}

func TestBackupTokenExpiryAndAccountStatusFailClosed(t *testing.T) {
	store := openTestStore(t)
	insertTestUser(t, store.db, StatusActive)
	userID := testUserID
	service := NewBackupTokenService(store)
	now := time.Unix(100, 0)
	service.now = func() time.Time { return now }
	expiresAt := now.Add(time.Hour).Unix()
	credential, err := service.Create(context.Background(), userID, CreateBackupToken{Name: "Restore", CanRestore: true, ExpiresAt: &expiresAt})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Unix(expiresAt, 0) }
	if _, err := service.Authenticate(context.Background(), credential.Token, BackupRestore); !errors.Is(err, ErrInvalidBackupToken) {
		t.Fatalf("expired token error=%v", err)
	}
	service.now = func() time.Time { return now }
	setTestUserStatus(t, store.db, StatusDisabled)
	if _, err := service.Authenticate(context.Background(), credential.Token, BackupRestore); !errors.Is(err, ErrInvalidBackupToken) {
		t.Fatalf("disabled account token error=%v", err)
	}
}

func TestBackupTokensCascadeWithAccount(t *testing.T) {
	store := openTestStore(t)
	insertTestUser(t, store.db, StatusActive)
	userID := testUserID
	service := NewBackupTokenService(store)
	if _, err := service.Create(context.Background(), userID, CreateBackupToken{Name: "Export", CanExport: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`DELETE FROM users WHERE id = ?`, string(userID)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT count(*) FROM backup_tokens WHERE user_id = ?`, string(userID)).Scan(&count); err != nil || count != 0 {
		t.Fatalf("count=%d error=%v", count, err)
	}
}
