package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestSetupHTTPHandlerCreatesAdministratorAndSession(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{
		PublicURL:      "https://reader.example",
		BootstrapToken: setupBootstrapToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler.setup.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	handler.now = func() time.Time { return time.Unix(100, 0) }

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), `"status":"open"`) {
		t.Fatalf("status=%d body=%s", statusResponse.Code, statusResponse.Body.String())
	}

	body, _ := json.Marshal(map[string]string{
		"token":    setupBootstrapToken,
		"username": "Alice",
		"password": "correct horse battery staple",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(body))
	request.Header.Set("Origin", "https://reader.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("setup status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != SessionCookieName || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookies=%#v", cookies)
	}
	account, err := NewSessionService(store).Authenticate(context.Background(), cookies[0].Value, 101)
	if err != nil || account.ID != testUserID || account.Role != RoleAdmin {
		t.Fatalf("session account=%#v error=%v", account, err)
	}

	closedRequest := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	closedResponse := httptest.NewRecorder()
	handler.ServeHTTP(closedResponse, closedRequest)
	if closedResponse.Code != http.StatusOK || !strings.Contains(closedResponse.Body.String(), `"status":"closed"`) || !strings.Contains(closedResponse.Body.String(), `"available":false`) {
		t.Fatalf("closed status=%d body=%s", closedResponse.Code, closedResponse.Body.String())
	}
}

func TestSetupHTTPHandlerRetriesAutoLoginAfterActivationWithoutCookie(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{BootstrapToken: setupBootstrapToken})
	if err != nil {
		t.Fatal(err)
	}
	handler.setup.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	handler.now = func() time.Time { return time.Unix(100, 0) }
	activated := make(chan struct{})
	release := make(chan struct{})
	handler.afterActivation = func(Account) {
		select {
		case <-activated:
		default:
			close(activated)
		}
		<-release
	}

	body := `{"token":"` + setupBootstrapToken + `","username":"Alice","password":"correct horse battery staple"}`
	firstRequest := httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(body))
	firstRequest.Header.Set("Origin", "http://reader.local")
	firstContext, cancel := context.WithCancel(firstRequest.Context())
	firstRequest = firstRequest.WithContext(firstContext)
	firstResponse := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(firstResponse, firstRequest)
		close(done)
	}()
	<-activated
	cancel()
	close(release)
	<-done
	if firstResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("canceled auto-login status=%d body=%s", firstResponse.Code, firstResponse.Body.String())
	}

	retryRequest := httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(body))
	retryRequest.Header.Set("Origin", "http://reader.local")
	retryResponse := httptest.NewRecorder()
	handler.ServeHTTP(retryResponse, retryRequest)
	if retryResponse.Code != http.StatusCreated || len(retryResponse.Result().Cookies()) != 1 {
		t.Fatalf("retry status=%d cookies=%#v body=%s", retryResponse.Code, retryResponse.Result().Cookies(), retryResponse.Body.String())
	}
}

func TestSetupHTTPHandlerClosedRetryAcceptsOnlyInitialAdministrator(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{BootstrapToken: setupBootstrapToken})
	if err != nil {
		t.Fatal(err)
	}
	handler.setup.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	initial := `{"token":"` + setupBootstrapToken + `","username":"Alice","password":"correct horse battery staple"}`
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(initial))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("initial status=%d body=%s", response.Code, response.Body.String())
	}
	hash, err := NewPasswordHasher().Hash(context.Background(), "second administrator password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`
		INSERT INTO users (id, username, username_normalized, role, password_hash, status, created_at, updated_at)
		VALUES (?, 'Bob', 'bob', 'admin', ?, 'active', 200, 200)
	`, string(secondTestUserID), hash); err != nil {
		t.Fatal(err)
	}
	second := `{"token":"` + setupBootstrapToken + `","username":"Bob","password":"second administrator password"}`
	request = httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(second))
	request.Header.Set("Origin", "http://reader.local")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || len(response.Result().Cookies()) != 0 {
		t.Fatalf("second admin retry status=%d cookies=%#v body=%s", response.Code, response.Result().Cookies(), response.Body.String())
	}
}

func TestSetupHTTPHandlerReturnsAtDeadlineWhilePasswordWorkFinishes(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{
		BootstrapToken: setupBootstrapToken,
		Timeout:        20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	workStarted := make(chan struct{})
	releaseWork := make(chan struct{})
	workFinished := make(chan struct{})
	handler.setup.passwords.derive = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		close(workStarted)
		<-releaseWork
		close(workFinished)
		return make([]byte, keyLen)
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(`{"token":"`+setupBootstrapToken+`","username":"Alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()
	<-workStarted
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("setup response exceeded deadline")
	}
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") == "" {
		t.Fatalf("status=%d retry=%q body=%s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
	}
	close(releaseWork)
	select {
	case <-workFinished:
	case <-time.After(time.Second):
		t.Fatal("password work did not finish")
	}
}

func TestSetupHTTPHandlerRateLimitsClosedSetupCredentialRetries(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{BootstrapToken: setupBootstrapToken})
	if err != nil {
		t.Fatal(err)
	}
	handler.setup.randomUserID = func() (readerstore.UserID, error) { return testUserID, nil }
	now := time.Unix(100, 0)
	handler.now = func() time.Time { return now }
	valid := `{"token":"` + setupBootstrapToken + `","username":"Alice","password":"correct horse battery staple"}`
	setupRequest := httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(valid))
	setupRequest.Header.Set("Origin", "http://reader.local")
	setupResponse := httptest.NewRecorder()
	handler.ServeHTTP(setupResponse, setupRequest)
	if setupResponse.Code != http.StatusCreated {
		t.Fatalf("initial setup status=%d body=%s", setupResponse.Code, setupResponse.Body.String())
	}

	handler.setupLimiter.limit = 2
	handler.setupLimiter.attempts = make(map[string]loginAttemptWindow)
	bad := `{"token":"` + setupBootstrapToken + `","username":"Alice","password":"wrong password candidate"}`
	for attempt := 1; attempt <= 3; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(bad))
		request.Header.Set("Origin", "http://reader.local")
		request.RemoteAddr = "192.0.2.20:12345"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt <= 2 && response.Code != http.StatusConflict {
			t.Fatalf("attempt %d status=%d body=%s", attempt, response.Code, response.Body.String())
		}
		if attempt == 3 && (response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "") {
			t.Fatalf("limited status=%d retry=%q body=%s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
		}
	}
}

func TestSetupHTTPHandlerRejectsUnsafeMalformedAndUnauthorizedRequests(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{
		PublicURL:      "https://reader.example",
		BootstrapToken: setupBootstrapToken,
	})
	if err != nil {
		t.Fatal(err)
	}

	validBody := `{"token":"` + setupBootstrapToken + `","username":"Alice","password":"correct horse battery staple"}`
	for _, origin := range []string{"", "https://evil.example"} {
		request := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(validBody))
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("origin=%q status=%d body=%s", origin, response.Code, response.Body.String())
		}
	}

	for _, body := range []string{
		`{"token":"wrong","username":"Alice","password":"correct horse battery staple"}`,
		`{"token":"wrong","token":"second","username":"Alice","password":"correct horse battery staple"}`,
		`{"token":"wrong","username":"Alice","password":"correct horse battery staple","extra":true}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(body))
		request.Header.Set("Origin", "https://reader.example")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if strings.Contains(body, `"token":"wrong","username"`) && !strings.Contains(body, `"extra"`) {
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("wrong token status=%d body=%s", response.Code, response.Body.String())
			}
		} else if response.Code != http.StatusBadRequest {
			t.Fatalf("malformed status=%d body=%s", response.Code, response.Body.String())
		}
	}

	oversized := `{"token":"wrong","username":"Alice","password":"` + strings.Repeat("x", maxSetupRequestSize) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(oversized))
	request.Header.Set("Origin", "https://reader.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSetupHTTPHandlerRecoversClaimWithoutReplayingCredentials(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{BootstrapToken: setupBootstrapToken})
	if err != nil {
		t.Fatal(err)
	}
	claim := setupClaim{
		userID:             testUserID,
		username:           "Alice",
		usernameNormalized: "alice",
		passwordHash:       dummyPasswordHash,
		claimedAt:          100,
		claimExpiresAt:     200,
	}
	if _, created, err := handler.setup.claim(context.Background(), claim); err != nil || !created {
		t.Fatalf("claim created=%t error=%v", created, err)
	}
	handler.now = func() time.Time { return time.Unix(500, 0) }
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/setup", strings.NewReader(`{"token":"`+setupBootstrapToken+`","username":"Different","password":"different secure password"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("recovery status=%d body=%s", response.Code, response.Body.String())
	}
	stored, err := accountByID(context.Background(), store.db, testUserID)
	if err != nil || stored.Username != "Alice" || stored.Role != RoleAdmin {
		t.Fatalf("stored account=%#v error=%v", stored, err)
	}
}

func TestSetupHTTPHandlerRetriesLateSessionRevocation(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	account, err := NewAccountService(store).CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100)
	if err != nil {
		t.Fatal(err)
	}
	handler := &SetupHTTPHandler{sessions: NewSessionService(store)}
	created := make(chan SessionCredential, 1)
	release := make(chan struct{})
	handler.afterSessionCreate = func(credential SessionCredential) {
		created <- credential
		<-release
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := handler.createSessionWithinDeadline(ctx, account.ID, 101)
		result <- err
	}()
	credential := <-created
	if err := <-result; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("session deadline error=%v", err)
	}
	if err := store.sessionGuard.lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(release)
	time.Sleep(25 * time.Millisecond)
	store.sessionGuard.unlock()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := handler.sessions.Authenticate(context.Background(), credential.Token, 102); errors.Is(err, ErrInvalidSession) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("late-created setup session was not revoked after transient guard contention")
}

func TestSetupHTTPHandlerReportsSetupUnavailableWithoutConfiguredToken(t *testing.T) {
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
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var status setupStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Status != "open" || status.Available {
		t.Fatalf("status=%#v", status)
	}
}

func TestSetupHTTPHandlerReportsClosedSetupWithoutTokenDetails(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	root := filepath.Dir(store.Path())
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	handler, err := NewSetupHTTPHandler(store, readers, SetupHTTPConfig{BootstrapToken: setupBootstrapToken})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), setupBootstrapToken) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var status setupStatusResponse
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Status != "closed" || status.Available {
		t.Fatalf("status=%#v", status)
	}
}
