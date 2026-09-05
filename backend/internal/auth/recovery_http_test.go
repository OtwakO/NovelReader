package auth

import (
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

func newTestRecoveryHTTPHandler(t *testing.T) (*RecoveryHTTPHandler, *Store, *readerstore.Manager) {
	t.Helper()
	store, readers, _ := preparedRecovery(t)
	handler, err := NewRecoveryHTTPHandler(store, readers, RecoveryHTTPConfig{RecoveryToken: adminRecoveryToken})
	if err != nil {
		t.Fatal(err)
	}
	handler.now = func() time.Time { return time.Unix(200, 0) }
	return handler, store, readers
}

func TestRecoveryHTTPHandlerResetsAdministratorAndCreatesSession(t *testing.T) {
	handler, store, _ := newTestRecoveryHTTPHandler(t)
	body := `{"token":"` + adminRecoveryToken + `","action":"reset_existing","username":"Alice","password":"replacement password value"}`
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/recovery", strings.NewReader(body))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected recovery response to disable caching, got %q", response.Header().Get("Cache-Control"))
	}
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Secure || cookies[0].MaxAge != sessionCookieMaxAgeSecs {
		t.Fatalf("cookies=%#v", cookies)
	}
	if _, err := handler.sessions.Authenticate(context.Background(), cookies[0].Value, 201); err != nil {
		t.Fatal(err)
	}
	if _, err := NewAccountService(store).Authenticate(context.Background(), "Alice", "replacement password value"); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryHTTPHandlerCreatesReplacementAdministrator(t *testing.T) {
	handler, _, _ := newTestRecoveryHTTPHandler(t)
	handler.recovery.randomUserID = func() (readerstore.UserID, error) { return secondTestUserID, nil }
	body := `{"token":"` + adminRecoveryToken + `","action":"create_replacement","username":"Bob","password":"replacement administrator password"}`
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/recovery", strings.NewReader(body))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(response.Result().Cookies()) != 1 {
		t.Fatalf("status=%d cookies=%#v body=%s", response.Code, response.Result().Cookies(), response.Body.String())
	}
	var account struct {
		ID   readerstore.UserID `json:"id"`
		Role Role               `json:"role"`
	}
	if err := json.NewDecoder(response.Body).Decode(&account); err != nil || account.ID != secondTestUserID || account.Role != RoleAdmin {
		t.Fatalf("account=%#v error=%v", account, err)
	}
}

func TestRecoveryHTTPHandlerRetriesAutoLoginWithRecoveredCredentials(t *testing.T) {
	handler, _, _ := newTestRecoveryHTTPHandler(t)
	recoveryFinished := make(chan struct{})
	releaseHook := make(chan struct{})
	handler.afterRecovery = func(Account) {
		close(recoveryFinished)
		<-releaseHook
	}
	body := `{"token":"` + adminRecoveryToken + `","action":"reset_existing","username":"Alice","password":"replacement password value"}`
	requestContext, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/recovery", strings.NewReader(body)).WithContext(requestContext)
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	firstDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(firstDone)
	}()
	<-recoveryFinished
	cancel()
	<-firstDone
	if response.Code != http.StatusServiceUnavailable || len(response.Result().Cookies()) != 0 {
		t.Fatalf("first status=%d cookies=%#v", response.Code, response.Result().Cookies())
	}
	close(releaseHook)
	handler.afterRecovery = nil
	request = httptest.NewRequest(http.MethodPost, "http://reader.local/api/recovery", strings.NewReader(body))
	request.Header.Set("Origin", "http://reader.local")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(response.Result().Cookies()) != 1 {
		t.Fatalf("retry status=%d cookies=%#v body=%s", response.Code, response.Result().Cookies(), response.Body.String())
	}
}

func TestRecoveryHTTPHandlerRejectsMalformedOriginAndWrongToken(t *testing.T) {
	handler, _, _ := newTestRecoveryHTTPHandler(t)
	valid := `{"token":"` + adminRecoveryToken + `","action":"reset_existing","username":"Alice","password":"replacement password value"}`
	for _, test := range []struct {
		name   string
		body   string
		origin string
		status int
	}{
		{name: "origin", body: valid, origin: "http://evil.local", status: http.StatusForbidden},
		{name: "wrong token", body: strings.Replace(valid, adminRecoveryToken, "wrong-token", 1), origin: "http://reader.local", status: http.StatusUnauthorized},
		{name: "unknown action", body: strings.Replace(valid, "reset_existing", "delete_data", 1), origin: "http://reader.local", status: http.StatusBadRequest},
		{name: "duplicate", body: `{"token":"x","token":"y","action":"reset_existing","username":"Alice","password":"replacement password value"}`, origin: "http://reader.local", status: http.StatusBadRequest},
		{name: "trailing", body: valid + `{}`, origin: "http://reader.local", status: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/recovery", strings.NewReader(test.body))
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRecoveryHTTPHandlerRateLimitsWrongTokens(t *testing.T) {
	handler, _, _ := newTestRecoveryHTTPHandler(t)
	handler.recoveryLimiter.limit = 2
	body := `{"token":"wrong-token","action":"reset_existing","username":"Alice","password":"replacement password value"}`
	for attempt := 1; attempt <= 3; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/recovery", strings.NewReader(body))
		request.Header.Set("Origin", "http://reader.local")
		request.RemoteAddr = "192.0.2.30:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt <= 2 && response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt=%d status=%d", attempt, response.Code)
		}
		if attempt == 3 && (response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "") {
			t.Fatalf("limited status=%d retry=%q", response.Code, response.Header().Get("Retry-After"))
		}
	}
}

func TestRecoveryHTTPHandlerStatusOnlyReportsConfiguration(t *testing.T) {
	handler, _, _ := newTestRecoveryHTTPHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/recovery/status", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Body.String() != "{\"available\":true}\n" {
		t.Fatalf("body=%s", response.Body.String())
	}
	handler.recovery.recoveryToken = ""
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Body.String() != "{\"available\":false}\n" {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestRecoveryHTTPHandlerReturnsAtDeadlineWhilePasswordWorkFinishes(t *testing.T) {
	handler, _, _ := newTestRecoveryHTTPHandler(t)
	handler.timeout = 20 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	handler.recovery.passwords.derive = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		close(started)
		<-release
		return make([]byte, keyLen)
	}
	body := `{"token":"` + adminRecoveryToken + `","action":"reset_existing","username":"Alice","password":"replacement password value"}`
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/recovery", strings.NewReader(body))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { handler.ServeHTTP(response, request); close(done) }()
	<-started
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("recovery response exceeded deadline")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	close(release)
}

func TestRecoveryHTTPHandlerRevokesSessionCreatedAfterDeadline(t *testing.T) {
	handler, _, _ := newTestRecoveryHTTPHandler(t)
	account, err := NewAccountService(handler.recovery.store).Authenticate(context.Background(), "Alice", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	created := make(chan SessionCredential, 1)
	release := make(chan struct{})
	handler.afterSessionCreate = func(credential SessionCredential) { created <- credential; <-release }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := createAuthenticatedSessionWithinDeadline(ctx, handler.sessions, account, 101, handler.afterSessionCreate, "test")
		result <- err
	}()
	credential := <-created
	if err := <-result; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := handler.sessions.Authenticate(context.Background(), credential.Token, 102); errors.Is(err, ErrInvalidSession) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("late-created recovery session remains valid")
}
