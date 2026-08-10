package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestAccountAdministrationListsOnlyOrdinaryReaders(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusDisabled)
	accounts := NewAccountService(store)

	readers, err := accounts.ListReaderAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(readers) != 1 || readers[0].ID != secondTestUserID || readers[0].Username != "Bob" || readers[0].Status != StatusDisabled {
		t.Fatalf("readers=%#v", readers)
	}
}

func TestAccountAdministrationDisablesReaderAndRevokesSessionsThenReenables(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	accounts := NewAccountService(store)
	reader, err := accountByID(context.Background(), store.db, secondTestUserID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSessionService(store).CreateAuthenticated(context.Background(), reader, 101)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := accounts.SetReaderEnabled(context.Background(), secondTestUserID, false, 200)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusDisabled || updated.UpdatedAt != 200 || updated.AuthVersion != reader.AuthVersion+1 {
		t.Fatalf("disabled=%#v", updated)
	}
	if _, err := NewSessionService(store).Authenticate(context.Background(), session.Token, 201); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("session remained valid: %v", err)
	}
	if _, err := accounts.Authenticate(context.Background(), "bob", "correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("disabled login error=%v", err)
	}

	updated, err = accounts.SetReaderEnabled(context.Background(), secondTestUserID, true, 300)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusActive || updated.UpdatedAt != 300 || updated.AuthVersion != reader.AuthVersion+2 {
		t.Fatalf("enabled=%#v", updated)
	}
	if _, err := accounts.Authenticate(context.Background(), "bob", "correct horse battery staple"); err != nil {
		t.Fatalf("re-enabled login error=%v", err)
	}
}

func TestAccountAdministrationProtectsAdministratorsAndTreatsDesiredStateAsIdempotent(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusDisabled)
	accounts := NewAccountService(store)

	if _, err := accounts.SetReaderEnabled(context.Background(), testUserID, false, 200); !errors.Is(err, ErrProtectedAccount) {
		t.Fatalf("administrator error=%v", err)
	}
	unchanged, err := accounts.SetReaderEnabled(context.Background(), secondTestUserID, false, 200)
	if err != nil || unchanged.Status != StatusDisabled || unchanged.UpdatedAt != 100 || unchanged.AuthVersion != 1 {
		t.Fatalf("idempotent result=%#v error=%v", unchanged, err)
	}
}

func TestAccountAdministrationRollsBackDisableWhenSessionRevocationFails(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	reader, err := accountByID(context.Background(), store.db, secondTestUserID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSessionService(store).CreateAuthenticated(context.Background(), reader, 101); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		CREATE TRIGGER reject_admin_session_revocation
		BEFORE DELETE ON auth_sessions
		BEGIN SELECT RAISE(ABORT, 'reject revocation'); END
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewAccountService(store).SetReaderEnabled(context.Background(), secondTestUserID, false, 200); err == nil {
		t.Fatal("disable succeeded despite failed revocation")
	}
	unchanged, err := accountByID(context.Background(), store.db, secondTestUserID)
	if err != nil || unchanged.Status != StatusActive || unchanged.AuthVersion != reader.AuthVersion {
		t.Fatalf("unchanged=%#v error=%v", unchanged, err)
	}
}

func TestAccountAdministrationHTTPRetryConvergesAfterCommittedTimeout(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	adminSession, err := NewSessionService(store).CreateAuthenticated(context.Background(), admin, 101)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{LoginTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	realSet := handler.setReaderEnabled
	handler.setReaderEnabled = func(ctx context.Context, userID readerstore.UserID, enabled bool, now int64) (Account, error) {
		account, err := realSet(ctx, userID, enabled, now)
		if err != nil {
			return account, err
		}
		<-ctx.Done()
		return Account{}, ctx.Err()
	}
	request := httptest.NewRequest(http.MethodPut, "http://reader.local/api/auth/admin/readers/"+string(secondTestUserID)+"/status", bytes.NewBufferString(`{"enabled":false}`))
	request.Header.Set("Origin", "http://reader.local")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminSession.Token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("timeout status=%d body=%s", response.Code, response.Body.String())
	}

	handler.setReaderEnabled = realSet
	request = httptest.NewRequest(http.MethodPut, "http://reader.local/api/auth/admin/readers/"+string(secondTestUserID)+"/status", bytes.NewBufferString(`{"enabled":false}`))
	request.Header.Set("Origin", "http://reader.local")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminSession.Token})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("retry status=%d body=%s", response.Code, response.Body.String())
	}
	var updated adminAccountResponse
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil || updated.Status != StatusDisabled {
		t.Fatalf("updated=%#v error=%v", updated, err)
	}
}

func TestAccountAdministrationHTTPRequiresAdministratorAndUsesStrictTarget(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	sessions := NewSessionService(store)
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	reader, _ := accountByID(context.Background(), store.db, secondTestUserID)
	adminSession, err := sessions.CreateAuthenticated(context.Background(), admin, 101)
	if err != nil {
		t.Fatal(err)
	}
	readerSession, err := sessions.CreateAuthenticated(context.Background(), reader, 102)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	handler.now = func() time.Time { return time.Unix(200, 0) }

	request := httptest.NewRequest(http.MethodGet, "/api/auth/admin/readers", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: readerSession.Token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("reader list status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/auth/admin/readers", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminSession.Token})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin list status=%d body=%s", response.Code, response.Body.String())
	}
	var listed struct {
		Accounts []adminAccountResponse `json:"accounts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Accounts) != 1 || listed.Accounts[0].ID != string(secondTestUserID) || listed.Accounts[0].Status != StatusActive {
		t.Fatalf("accounts=%#v", listed.Accounts)
	}

	for _, test := range []struct {
		path   string
		body   string
		origin string
		want   int
	}{
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{"enabled":false}`, origin: "http://reader.local", want: http.StatusOK},
		{path: "/api/auth/admin/readers/" + string(testUserID) + "/status", body: `{"enabled":false}`, origin: "http://reader.local", want: http.StatusForbidden},
		{path: "/api/auth/admin/readers/not-a-uuid/status", body: `{"enabled":false}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{"enabled":true,"extra":1}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{"enabled":true,"padding":"` + strings.Repeat("x", maxLoginRequestSize) + `"}`, origin: "http://reader.local", want: http.StatusRequestEntityTooLarge},
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{"enabled":true,"enabled":false}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{"enabled":"yes"}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{"enabled":true} {}`, origin: "http://reader.local", want: http.StatusBadRequest},
		{path: "/api/auth/admin/readers/" + string(secondTestUserID) + "/status", body: `{"enabled":true}`, origin: "http://evil.local", want: http.StatusForbidden},
	} {
		request = httptest.NewRequest(http.MethodPut, "http://reader.local"+test.path, bytes.NewBufferString(test.body))
		request.Header.Set("Origin", test.origin)
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminSession.Token})
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("path=%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}
