package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestRegistrationHTTPPolicyAndInviteAdmission(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`UPDATE setup_state SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2 WHERE id = 1`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()

	disabled, err := NewHTTPHandler(store, HTTPConfig{Readers: readers})
	if err != nil {
		t.Fatal(err)
	}
	policy := httptest.NewRecorder()
	disabled.ServeHTTP(policy, httptest.NewRequest(http.MethodGet, "/api/auth/registration", nil))
	if policy.Code != http.StatusOK || policy.Body.String() != "{\"enabled\":false,\"inviteRequired\":false}\n" {
		t.Fatalf("disabled policy status=%d body=%s", policy.Code, policy.Body.String())
	}

	enabled, err := NewHTTPHandler(store, HTTPConfig{Readers: readers, RegistrationEnabled: true, RegistrationInviteCode: "private-admission"})
	if err != nil {
		t.Fatal(err)
	}
	policy = httptest.NewRecorder()
	enabled.ServeHTTP(policy, httptest.NewRequest(http.MethodGet, "/api/auth/registration", nil))
	if policy.Code != http.StatusOK || policy.Body.String() != "{\"enabled\":true,\"inviteRequired\":true}\n" {
		t.Fatalf("enabled policy status=%d body=%s", policy.Code, policy.Body.String())
	}
	for _, invite := range []string{"", "wrong-admission"} {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"Alice","password":"correct horse battery staple","inviteCode":"`+invite+`"}`))
		request.Header.Set("Origin", "http://reader.local")
		response := httptest.NewRecorder()
		enabled.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("invite=%q status=%d body=%s", invite, response.Code, response.Body.String())
		}
	}
}

func TestRegistrationHTTPReturnsAtDeadlineWhileProvisioningFinishes(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	handler, err := NewHTTPHandler(store, HTTPConfig{LoginTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	handler.registration = NewRegistrationService(store, nil, true, "")
	started := make(chan struct{})
	release := make(chan struct{})
	handler.registerAccount = func(context.Context, string, string, string, time.Time) (Account, error) {
		close(started)
		<-release
		return Account{}, context.Canceled
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"Alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()
	<-started
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		close(release)
		t.Fatal("registration response exceeded its deadline")
	}
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" {
		close(release)
		t.Fatalf("status=%d retry=%q body=%s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
	}
	close(release)
}

func TestRegistrationServiceDoesNotResumeAdministratorDisabledAccount(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`UPDATE setup_state SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2 WHERE id = 1`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	accounts := NewAccountService(store)
	disabledID, err := randomUserID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.createAccount(context.Background(), disabledID, "Alice", "correct horse battery staple", RoleReader, StatusDisabled, 1, 5); err != nil {
		t.Fatal(err)
	}
	_, err = NewRegistrationService(store, readers, true, "").Register(context.Background(), "alice", "correct horse battery staple", "", time.Unix(10, 0))
	if !errors.Is(err, ErrUsernameUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestRegistrationHTTPResumesDurableReservationAfterInterruption(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`UPDATE setup_state SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2 WHERE id = 1`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	reservedID, err := randomUserID()
	if err != nil {
		t.Fatal(err)
	}
	accounts := NewAccountService(store)
	if _, err := accounts.createAccount(context.Background(), reservedID, "Alice", "correct horse battery staple", RoleReader, StatusDisabled, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := readers.Create(context.Background(), reservedID); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{Readers: readers, RegistrationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"Alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var account accountResponse
	if err := json.Unmarshal(response.Body.Bytes(), &account); err != nil {
		t.Fatal(err)
	}
	if account.ID != string(reservedID) {
		t.Fatalf("account id=%q want=%q", account.ID, reservedID)
	}
}

func TestRegistrationHTTPKeepsRecoverableReservationWhenHomeCreationFails(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`UPDATE setup_state SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2 WHERE id = 1`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	closedReaders, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := closedReaders.Close(); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPHandler(store, HTTPConfig{Readers: closedReaders, RegistrationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"Alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("failed provisioning status=%d body=%s", response.Code, response.Body.String())
	}

	readers, err := readerstore.NewManager(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	handler, err = NewHTTPHandler(store, HTTPConfig{Readers: readers, RegistrationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"Alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("retry status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRegistrationHTTPRejectsMalformedAndConflictingAccounts(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`UPDATE setup_state SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2 WHERE id = 1`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	handler, err := NewHTTPHandler(store, HTTPConfig{Readers: readers, RegistrationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"username":"Alice","password":"correct horse battery staple","extra":true}`,
		`{"username":"Alice","username":"Mallory","password":"correct horse battery staple"}`,
		`{"username":"Alice","password":"first candidate password","password":"second candidate password"}`,
		`{"username":"Alice"}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(body))
		request.Header.Set("Origin", "http://reader.local")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%s", body, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"Alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("initial status=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"alice","password":"different candidate password"}`))
	request.Header.Set("Origin", "http://reader.local")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRegistrationServiceConcurrentSameCredentialsConvergeOnOneAccount(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`UPDATE setup_state SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2 WHERE id = 1`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	service := NewRegistrationService(store, readers, true, "")
	start := make(chan struct{})
	results := make(chan Account, 2)
	errResults := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			account, err := service.Register(context.Background(), "Alice", "correct horse battery staple", "", time.Unix(10, 0))
			results <- account
			errResults <- err
		}()
	}
	close(start)
	accounts := []Account{<-results, <-results}
	registrationErrors := []error{<-errResults, <-errResults}
	successes := make([]Account, 0, 2)
	conflicts := 0
	for index, err := range registrationErrors {
		if err == nil {
			successes = append(successes, accounts[index])
		} else if errors.Is(err, ErrUsernameUnavailable) {
			conflicts++
		}
	}
	if len(successes) == 0 || len(successes)+conflicts != 2 || len(successes) == 2 && successes[0].ID != successes[1].ID {
		t.Fatalf("accounts=%+v errors=%v", accounts, registrationErrors)
	}
}

func TestRegistrationHTTPCreatesReaderHomeAndAuthenticatedSession(t *testing.T) {
	root := prepareTestRoot(t)
	store, err := OpenSystemStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.db.Exec(`UPDATE setup_state SET status = 'closed', proposed_user_id = ?, claimed_at = 1, claim_expires_at = 2, closed_at = 2 WHERE id = 1`, string(testUserID)); err != nil {
		t.Fatal(err)
	}
	readers, err := readerstore.NewManager(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	handler, err := NewHTTPHandler(store, HTTPConfig{Readers: readers, RegistrationEnabled: true})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/register", bytes.NewBufferString(`{"username":"Alice","password":"correct horse battery staple"}`))
	request.Header.Set("Origin", "http://reader.local")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var account accountResponse
	if err := json.Unmarshal(response.Body.Bytes(), &account); err != nil {
		t.Fatal(err)
	}
	userID, err := readerstore.ParseUserID(account.ID)
	if err != nil || account.Username != "Alice" || account.Role != RoleReader {
		t.Fatalf("account=%+v err=%v", account, err)
	}
	if exists, err := readers.Exists(context.Background(), userID); err != nil || !exists {
		t.Fatalf("reader home exists=%v err=%v", exists, err)
	}
	cookie := response.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != SessionCookieName || cookie[0].Value == "" || !cookie[0].HttpOnly {
		t.Fatalf("cookies=%v", cookie)
	}
	authenticated, err := NewSessionService(store).Authenticate(context.Background(), cookie[0].Value, 2)
	if err != nil || authenticated.ID != userID {
		t.Fatalf("authenticated=%+v err=%v", authenticated, err)
	}
}
