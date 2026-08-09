package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"
)

func TestHTTPHandlerRejectsUnsafeAndExcessiveLoginRequests(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	handler, err := NewHTTPHandler(store, HTTPConfig{
		PublicURL:     "https://reader.example",
		LoginTimeout:  5 * time.Second,
		LoginAttempts: 2,
		LoginWindow:   time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"alice","password":"candidate password"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing origin status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"alice","password":"candidate password"}`))
	request.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("foreign origin status=%d body=%s", response.Code, response.Body.String())
	}

	for _, body := range []string{
		`{"username":"alice","password":"candidate password","extra":true}`,
		`{"username":"alice","username":"admin","password":"candidate password"}`,
		`{"username":"alice","password":"first","password":"second"}`,
	} {
		request = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		request.Header.Set("Origin", "https://reader.example")
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid body %s status=%d body=%s", body, response.Code, response.Body.String())
		}
	}

	request = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"alice","password":"`+strings.Repeat("x", maxLoginRequestSize)+`"}`))
	request.Header.Set("Origin", "https://reader.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status=%d body=%s", response.Code, response.Body.String())
	}

	for attempt := 1; attempt <= 3; attempt++ {
		request = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"alice","password":"candidate password"}`))
		request.Header.Set("Origin", "https://reader.example")
		request.RemoteAddr = "192.0.2.10:12345"
		request.Header.Set("X-Forwarded-For", "198.51.100."+string(rune('0'+attempt)))
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt <= 2 {
			if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "invalid username or password") {
				t.Fatalf("attempt %d status=%d body=%s", attempt, response.Code, response.Body.String())
			}
		} else if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
			t.Fatalf("limited attempt status=%d retry=%q body=%s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
		}
	}
}

func TestHTTPHandlerBoundsLoginResponseWhilePasswordWorkFinishes(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{
		LoginTimeout:  20 * time.Millisecond,
		LoginAttempts: 10,
		LoginWindow:   time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	workStarted := make(chan struct{})
	releaseWork := make(chan struct{})
	workFinished := make(chan struct{})
	handler.accounts.passwords.derive = func(password, salt []byte, iterations, memory uint32, parallelism uint8, keyLen uint32) []byte {
		close(workStarted)
		<-releaseWork
		result := argon2.IDKey(password, salt, iterations, memory, parallelism, keyLen)
		close(workFinished)
		return result
	}

	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/login", bytes.NewBufferString(`{"username":"alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	startedAt := time.Now()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()
	<-workStarted
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		close(releaseWork)
		t.Fatal("login response exceeded its deadline")
	}
	elapsed := time.Since(startedAt)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" || elapsed >= 200*time.Millisecond {
		close(releaseWork)
		t.Fatalf("deadline response status=%d retry=%q elapsed=%s body=%s", response.Code, response.Header().Get("Retry-After"), elapsed, response.Body.String())
	}
	close(releaseWork)
	<-workFinished
}

type blockingLoginBody struct {
	closed chan struct{}
	once   sync.Once
}

func (b *blockingLoginBody) Read([]byte) (int, error) {
	<-b.closed
	return 0, errors.New("closed")
}

func (b *blockingLoginBody) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

func TestHTTPHandlerBoundsSlowLoginBody(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	handler, err := NewHTTPHandler(store, HTTPConfig{LoginTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	body := &blockingLoginBody{closed: make(chan struct{})}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/login", nil)
	request.Body = body
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	startedAt := time.Now()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || time.Since(startedAt) >= 200*time.Millisecond {
		t.Fatalf("slow-body status=%d elapsed=%s body=%s", response.Code, time.Since(startedAt), response.Body.String())
	}
}

func TestHTTPHandlerRevokesSessionCommittedAtDeadline(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	created := make(chan SessionCredential, 1)
	releaseResult := make(chan struct{})
	handler.afterSessionCreate = func(credential SessionCredential) {
		created <- credential
		<-releaseResult
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, createErr := handler.createSessionWithinDeadline(ctx, testUserID, 100)
		result <- createErr
	}()
	credential := <-created
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		close(releaseResult)
		t.Fatalf("creation error=%v", err)
	}
	close(releaseResult)
	deadline := time.Now().Add(time.Second)
	for {
		_, err := handler.sessions.Authenticate(context.Background(), credential.Token, 101)
		if errors.Is(err, ErrInvalidSession) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("orphaned session remains valid: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestHTTPHandlerBoundsSessionCreationWhileRevocationBarrierIsBusy(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.sessionGuard.lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	_, err = handler.createSessionWithinDeadline(ctx, testUserID, 100)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(startedAt) >= 200*time.Millisecond {
		store.sessionGuard.unlock()
		t.Fatalf("session deadline error=%v elapsed=%s", err, time.Since(startedAt))
	}
	store.sessionGuard.unlock()
}

func TestHTTPHandlerKeepsDeviceSessionsIndependentAndLogoutIdempotent(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{LoginAttempts: 10, LoginWindow: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	login := func() *http.Cookie {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/login", bytes.NewBufferString(`{"username":"alice","password":"correct horse battery staple"}`))
		request.Header.Set("Origin", "http://reader.local")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("login status=%d body=%s", response.Code, response.Body.String())
		}
		cookie := response.Result().Cookies()[0]
		if cookie.Secure {
			t.Fatalf("development HTTP cookie is Secure: %#v", cookie)
		}
		return cookie
	}
	first := login()
	second := login()
	if first.Value == second.Value {
		t.Fatal("independent logins returned one token")
	}
	logout := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/logout", nil)
	logout.Header.Set("Origin", "http://reader.local")
	logout.AddCookie(first)
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("first logout status=%d", logoutResponse.Code)
	}
	secondAccount := httptest.NewRequest(http.MethodGet, "/api/auth/account", nil)
	secondAccount.AddCookie(second)
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondAccount)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second session status=%d body=%s", secondResponse.Code, secondResponse.Body.String())
	}
	missingLogout := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/logout", nil)
	missingLogout.Header.Set("Origin", "http://reader.local")
	missingLogoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingLogoutResponse, missingLogout)
	if missingLogoutResponse.Code != http.StatusNoContent {
		t.Fatalf("missing-cookie logout status=%d body=%s", missingLogoutResponse.Code, missingLogoutResponse.Body.String())
	}
}

func TestHTTPHandlerRequireAdminAllowsAdministrator(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	insertTestUser(t, store.db, StatusActive)
	if _, err := store.db.Exec(`UPDATE users SET role = 'admin' WHERE id = ?`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	sessions := NewSessionService(store)
	credential, err := sessions.Create(context.Background(), testUserID, 100)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: credential.Token})
	response := httptest.NewRecorder()
	handler.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !reached {
		t.Fatalf("admin status=%d reached=%t body=%s", response.Code, reached, response.Body.String())
	}
}

func TestHTTPHandlerLoginAccountAndLogoutLifecycle(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	accounts := NewAccountService(store)
	if _, err := accounts.CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{
		PublicURL:     "https://reader.example",
		LoginTimeout:  5 * time.Second,
		LoginAttempts: 10,
		LoginWindow:   time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	loginBody := bytes.NewBufferString(`{"username":"alice","password":"correct horse battery staple"}`)
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	login.Header.Set("Content-Type", "application/json")
	login.Header.Set("Origin", "https://reader.example")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies=%#v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != SessionCookieName || cookie.Value == "" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge != 0 {
		t.Fatalf("session cookie=%#v", cookie)
	}
	var accountResponse struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Role     Role   `json:"role"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &accountResponse); err != nil {
		t.Fatal(err)
	}
	if accountResponse.ID != string(testUserID) || accountResponse.Username != "Alice" || accountResponse.Role != RoleReader {
		t.Fatalf("login account=%#v", accountResponse)
	}

	identityReached := false
	identityHandler := handler.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, ok := IdentityFromContext(r.Context())
		identityReached = ok && account.ID == testUserID
		w.WriteHeader(http.StatusNoContent)
	}))
	identityRequest := httptest.NewRequest(http.MethodGet, "/protected", nil)
	identityRequest.AddCookie(cookie)
	identityResponse := httptest.NewRecorder()
	identityHandler.ServeHTTP(identityResponse, identityRequest)
	if identityResponse.Code != http.StatusNoContent || !identityReached {
		t.Fatalf("identity middleware status=%d reached=%t", identityResponse.Code, identityReached)
	}
	adminResponse := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(http.MethodGet, "/admin", nil)
	adminRequest.AddCookie(cookie)
	handler.RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("reader reached administrator handler")
	})).ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusForbidden {
		t.Fatalf("reader admin status=%d body=%s", adminResponse.Code, adminResponse.Body.String())
	}

	accountRequest := httptest.NewRequest(http.MethodGet, "/api/auth/account", nil)
	accountRequest.AddCookie(cookie)
	accountHTTPResponse := httptest.NewRecorder()
	handler.ServeHTTP(accountHTTPResponse, accountRequest)
	if accountHTTPResponse.Code != http.StatusOK || accountHTTPResponse.Body.String() != loginResponse.Body.String() {
		t.Fatalf("account status=%d body=%s", accountHTTPResponse.Code, accountHTTPResponse.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Origin", "https://reader.example")
	logoutRequest.AddCookie(cookie)
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d body=%s", logoutResponse.Code, logoutResponse.Body.String())
	}
	cleared := logoutResponse.Result().Cookies()
	if len(cleared) != 1 || cleared[0].Name != SessionCookieName || cleared[0].Value != "" || cleared[0].MaxAge >= 0 {
		t.Fatalf("cleared cookie=%#v", cleared)
	}

	revokedRequest := httptest.NewRequest(http.MethodGet, "/api/auth/account", nil)
	revokedRequest.AddCookie(cookie)
	revokedResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokedResponse, revokedRequest)
	if revokedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked account status=%d body=%s", revokedResponse.Code, revokedResponse.Body.String())
	}
}
