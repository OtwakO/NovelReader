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

func TestPasswordChangeHTTPVerifiesCurrentPasswordAndRevokesEverySession(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	account, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100)
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionService(store)
	current, err := sessions.CreateAuthenticated(context.Background(), account, 101)
	if err != nil {
		t.Fatal(err)
	}
	other, err := sessions.CreateAuthenticated(context.Background(), account, 102)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	handler.now = func() time.Time { return time.Unix(200, 0) }

	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password", bytes.NewBufferString(`{"currentPassword":"correct horse battery staple","newPassword":"replacement password value"}`))
	request.Header.Set("Origin", "http://reader.local")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: current.Token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != SessionCookieName || cookies[0].MaxAge != -1 {
		t.Fatalf("cookies=%v", cookies)
	}
	for _, token := range []string{current.Token, other.Token} {
		if _, err := sessions.Authenticate(context.Background(), token, 201); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("session remained valid: %v", err)
		}
	}
	if _, err := accounts.Authenticate(context.Background(), "alice", "correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password err=%v", err)
	}
	if _, err := accounts.Authenticate(context.Background(), "alice", "replacement password value"); err != nil {
		t.Fatalf("new password err=%v", err)
	}
}

func TestPasswordChangeHTTPRequiresAuthenticationAndStrictBody(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	account, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSessionService(store).CreateAuthenticated(context.Background(), account, 101)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password", bytes.NewBufferString(`{"currentPassword":"current password value","newPassword":"replacement password value"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", response.Code)
	}
	for _, body := range []string{
		`{"currentPassword":"current password value","newPassword":"replacement password value","extra":true}`,
		`{"currentPassword":"first current password","currentPassword":"second current password","newPassword":"replacement password value"}`,
		`{"currentPassword":"current password value"}`,
	} {
		request = httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password", bytes.NewBufferString(body))
		request.Header.Set("Origin", "http://reader.local")
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session.Token})
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
	request = httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password", bytes.NewBufferString(`{"currentPassword":"correct horse battery staple","newPassword":"`+strings.Repeat("x", maxLoginRequestSize)+`"}`))
	request.Header.Set("Origin", "http://reader.local")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session.Token})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPasswordChangeHTTPFailsClosedWhenCompletionIsAmbiguous(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	account, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSessionService(store).CreateAuthenticated(context.Background(), account, 101)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{LoginTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	handler.changePassword = func(context.Context, readerstore.UserID, string, string, int64) error {
		close(started)
		<-release
		return nil
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password", bytes.NewBufferString(`{"currentPassword":"correct horse battery staple","newPassword":"replacement password value"}`))
	request.Header.Set("Origin", "http://reader.local")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session.Token})
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { handler.ServeHTTP(response, request); close(done) }()
	<-started
	<-done
	close(release)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Fatalf("cookies=%v", cookies)
	}
}

func TestPasswordChangeHTTPRejectsWrongCurrentPasswordWithoutChangingCredentials(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	account, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100)
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionService(store)
	current, err := sessions.CreateAuthenticated(context.Background(), account, 101)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/password", bytes.NewBufferString(`{"currentPassword":"wrong current password","newPassword":"replacement password value"}`))
	request.Header.Set("Origin", "http://reader.local")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: current.Token})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body["error"] != "invalid current password" {
		t.Fatalf("body=%s err=%v", response.Body.String(), err)
	}
	if _, err := sessions.Authenticate(context.Background(), current.Token, 102); err != nil {
		t.Fatalf("current session revoked: %v", err)
	}
	if _, err := accounts.Authenticate(context.Background(), "alice", "correct horse battery staple"); err != nil {
		t.Fatalf("current password changed: %v", err)
	}
}
