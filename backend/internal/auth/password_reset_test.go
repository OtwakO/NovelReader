package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReaderPasswordResetIsHashOnlySingleUseAndRevokesSessions(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	reader, _ := accountByID(context.Background(), store.db, secondTestUserID)
	session, err := NewSessionService(store).CreateAuthenticated(context.Background(), reader, 101)
	if err != nil {
		t.Fatal(err)
	}
	resets := NewPasswordResetService(store)
	resets.now = func() int64 { return 202 }
	first, err := resets.Issue(context.Background(), secondTestUserID, admin, 200)
	if err != nil {
		t.Fatal(err)
	}
	second, err := resets.Issue(context.Background(), secondTestUserID, admin, 201)
	if err != nil {
		t.Fatal(err)
	}
	if first.Token == second.Token || second.ExpiresAt != 201+int64(defaultPasswordResetTTL.Seconds()) {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
	var storedHash []byte
	var count int
	if err := store.db.QueryRow(`SELECT token_hash, COUNT(*) OVER () FROM password_reset_tokens WHERE user_id = ? AND used_at IS NULL`, string(secondTestUserID)).Scan(&storedHash, &count); err != nil {
		t.Fatal(err)
	}
	expected := sha256.Sum256([]byte(second.Token))
	if count != 1 || !bytes.Equal(storedHash, expected[:]) || bytes.Contains(storedHash, []byte(second.Token)) {
		t.Fatalf("count=%d stored=%x", count, storedHash)
	}
	if err := resets.Complete(context.Background(), first.Token, "new correct horse battery staple"); !errors.Is(err, ErrInvalidPasswordReset) {
		t.Fatalf("superseded token error=%v", err)
	}
	if err := resets.Complete(context.Background(), second.Token, "new correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSessionService(store).Authenticate(context.Background(), session.Token, 203); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("session error=%v", err)
	}
	if _, err := NewAccountService(store).Authenticate(context.Background(), "Bob", "new correct horse battery staple"); err != nil {
		t.Fatalf("new password error=%v", err)
	}
	if err := resets.Complete(context.Background(), second.Token, "another correct horse battery staple"); !errors.Is(err, ErrInvalidPasswordReset) {
		t.Fatalf("replay error=%v", err)
	}
}

func TestReaderPasswordResetRetainsIssuerAuditMetadataAfterIssuerRemoval(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	resets := NewPasswordResetService(store)
	resets.now = func() int64 { return 201 }
	if _, err := resets.Issue(context.Background(), secondTestUserID, admin, 200); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`DELETE FROM users WHERE id = ?`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	var issuerID *string
	var issuerUsername string
	if err := store.db.QueryRow(`SELECT created_by_user_id, created_by_username FROM password_reset_tokens`).Scan(&issuerID, &issuerUsername); err != nil {
		t.Fatal(err)
	}
	if issuerID != nil || issuerUsername != "Administrator" {
		t.Fatalf("issuerID=%v issuerUsername=%q", issuerID, issuerUsername)
	}
}

func TestReaderPasswordResetConcurrentCompletionHasOneConsumer(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	resets := NewPasswordResetService(store)
	resets.now = func() int64 { return 201 }
	issued, err := resets.Issue(context.Background(), secondTestUserID, admin, 200)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for _, password := range []string{"first new correct horse battery staple", "second new correct horse battery staple"} {
		group.Add(1)
		go func(password string) {
			defer group.Done()
			<-start
			results <- resets.Complete(context.Background(), issued.Token, password)
		}(password)
	}
	close(start)
	group.Wait()
	close(results)
	var succeeded, rejected int
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrInvalidPasswordReset) {
			rejected++
		} else {
			t.Fatalf("unexpected completion error=%v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("succeeded=%d rejected=%d", succeeded, rejected)
	}
}

func TestReaderPasswordResetRollsBackPasswordAndTokenWhenSessionRevocationFails(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	reader, _ := accountByID(context.Background(), store.db, secondTestUserID)
	if _, err := NewSessionService(store).CreateAuthenticated(context.Background(), reader, 101); err != nil {
		t.Fatal(err)
	}
	resets := NewPasswordResetService(store)
	resets.now = func() int64 { return 201 }
	issued, err := resets.Issue(context.Background(), secondTestUserID, admin, 200)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		CREATE TRIGGER reject_reset_session_revocation
		BEFORE DELETE ON auth_sessions
		BEGIN SELECT RAISE(ABORT, 'reject revocation'); END
	`); err != nil {
		t.Fatal(err)
	}
	if err := resets.Complete(context.Background(), issued.Token, "new correct horse battery staple"); err == nil {
		t.Fatal("reset succeeded despite failed session revocation")
	}
	if _, err := NewAccountService(store).Authenticate(context.Background(), "Bob", "correct horse battery staple"); err != nil {
		t.Fatalf("old password changed after rollback: %v", err)
	}
	var usedAt *int64
	if err := store.db.QueryRow(`SELECT used_at FROM password_reset_tokens`).Scan(&usedAt); err != nil || usedAt != nil {
		t.Fatalf("usedAt=%v error=%v", usedAt, err)
	}
}

func TestReaderPasswordResetRejectsWhenExpiryPassesDuringPasswordWork(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	resetNow := int64(201)
	resets := NewPasswordResetService(store)
	resets.now = func() int64 { return resetNow }
	issued, err := resets.Issue(context.Background(), secondTestUserID, admin, 200)
	if err != nil {
		t.Fatal(err)
	}
	resets.passwords.derive = func(_, _ []byte, _, _ uint32, _ uint8, keyLen uint32) []byte {
		resetNow = issued.ExpiresAt
		return make([]byte, keyLen)
	}
	if err := resets.Complete(context.Background(), issued.Token, "new correct horse battery staple"); !errors.Is(err, ErrInvalidPasswordReset) {
		t.Fatalf("expiry crossing error=%v", err)
	}
}

func TestReaderPasswordResetProtectsAdministratorsExpiresAndPreservesDisabledStatus(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusDisabled)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	resets := NewPasswordResetService(store)
	resetNow := int64(200 + int64(defaultPasswordResetTTL.Seconds()))
	resets.now = func() int64 { return resetNow }
	if _, err := resets.Issue(context.Background(), testUserID, admin, 200); !errors.Is(err, ErrProtectedAccount) {
		t.Fatalf("administrator issue error=%v", err)
	}
	issued, err := resets.Issue(context.Background(), secondTestUserID, admin, 200)
	if err != nil {
		t.Fatal(err)
	}
	if err := resets.Complete(context.Background(), issued.Token, "new correct horse battery staple"); !errors.Is(err, ErrInvalidPasswordReset) {
		t.Fatalf("expiry error=%v", err)
	}
	issued, err = resets.Issue(context.Background(), secondTestUserID, admin, 300)
	if err != nil {
		t.Fatal(err)
	}
	resetNow = 301
	if err := resets.Complete(context.Background(), issued.Token, "new correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	account, err := accountByID(context.Background(), store.db, secondTestUserID)
	if err != nil || account.Status != StatusDisabled {
		t.Fatalf("account=%#v error=%v", account, err)
	}
	if _, err := NewAccountService(store).Authenticate(context.Background(), "Bob", "new correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("disabled login error=%v", err)
	}
}

func TestReaderPasswordResetHTTPRejectsMalformedCompletionRequests(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	validToken := strings.Repeat("a", 43)
	for _, test := range []struct {
		body   string
		origin string
		want   int
	}{
		{body: `{"token":"` + validToken + `","newPassword":"new correct horse battery staple","extra":1}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{body: `{"token":"` + validToken + `","token":"` + validToken + `","newPassword":"new correct horse battery staple"}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{body: `{"token":"` + validToken + `"}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{body: `{"token":1,"newPassword":"new correct horse battery staple"}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{body: `{"token":"` + validToken + `","newPassword":"new correct horse battery staple"} {}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{body: `{"token":"` + validToken + `","newPassword":"` + strings.Repeat("x", maxLoginRequestSize) + `"}`, origin: "http://reader.local", want: http.StatusRequestEntityTooLarge},
		{body: `{"token":"` + validToken + `","newPassword":"new correct horse battery staple"}`, origin: "http://evil.local", want: http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password-reset", bytes.NewBufferString(test.body))
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("body length=%d status=%d body=%s", len(test.body), response.Code, response.Body.String())
		}
	}
}

func TestReaderPasswordResetHTTPAuthorizationAndPublicCompletion(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	reader, _ := accountByID(context.Background(), store.db, secondTestUserID)
	sessions := NewSessionService(store)
	adminSession, _ := sessions.CreateAuthenticated(context.Background(), admin, 101)
	readerSession, _ := sessions.CreateAuthenticated(context.Background(), reader, 102)
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	handler.now = func() time.Time { return time.Unix(200, 0) }

	issue := func(session string, target string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/admin/readers/"+target+"/password-reset", nil)
		request.Header.Set("Origin", "http://reader.local")
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	if response := issue(readerSession.Token, string(secondTestUserID)); response.Code != http.StatusForbidden {
		t.Fatalf("reader issue status=%d body=%s", response.Code, response.Body.String())
	}
	if response := issue(adminSession.Token, string(testUserID)); response.Code != http.StatusForbidden {
		t.Fatalf("protected issue status=%d body=%s", response.Code, response.Body.String())
	}
	response := issue(adminSession.Token, string(secondTestUserID))
	if response.Code != http.StatusCreated {
		t.Fatalf("issue status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("cache headers=%v", response.Header())
	}
	var issued passwordResetIssueResponse
	if err := json.Unmarshal(response.Body.Bytes(), &issued); err != nil || issued.Token == "" || issued.ExpiresAt != 200+int64(defaultPasswordResetTTL.Seconds()) {
		t.Fatalf("issued=%#v error=%v", issued, err)
	}

	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password-reset", bytes.NewBufferString(`{"token":"`+issued.Token+`","newPassword":"new correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("complete status=%d body=%s", response.Code, response.Body.String())
	}
}
