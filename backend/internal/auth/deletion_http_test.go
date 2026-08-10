package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestReaderDeletionHTTPRequiresAdministratorExactUsernameAndStrictBody(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers, err := readerstore.NewManager(filepath.Dir(store.Path()), 4)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
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
	handler.ConfigureDeletionQuiescer(readers, func(context.Context, readerstore.UserID) error { return nil })

	for _, test := range []struct {
		session string
		target  string
		body    string
		origin  string
		want    int
	}{
		{readerSession.Token, string(secondTestUserID), `{"username":"Bob"}`, "http://reader.local", http.StatusForbidden},
		{adminSession.Token, string(testUserID), `{"username":"Administrator"}`, "http://reader.local", http.StatusForbidden},
		{adminSession.Token, "not-a-uuid", `{"username":"Bob"}`, "http://reader.local", http.StatusBadRequest},
		{adminSession.Token, string(secondTestUserID), `{"username":"bob"}`, "http://reader.local", http.StatusBadRequest},
		{adminSession.Token, string(secondTestUserID), `{"username":"Bob","extra":1}`, "http://reader.local", http.StatusBadRequest},
		{adminSession.Token, string(secondTestUserID), `{"username":"Bob","username":"Bob"}`, "http://reader.local", http.StatusBadRequest},
		{adminSession.Token, string(secondTestUserID), `{}`, "http://reader.local", http.StatusBadRequest},
		{adminSession.Token, string(secondTestUserID), `{"username":1}`, "http://reader.local", http.StatusBadRequest},
		{adminSession.Token, string(secondTestUserID), `{"username":"Bob"} {}`, "http://reader.local", http.StatusBadRequest},
		{adminSession.Token, string(secondTestUserID), `{"username":"` + strings.Repeat("x", maxLoginRequestSize) + `"}`, "http://reader.local", http.StatusRequestEntityTooLarge},
		{adminSession.Token, string(secondTestUserID), `{"username":"Bob"}`, "http://evil.local", http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodDelete, "http://reader.local/api/auth/admin/readers/"+test.target, bytes.NewBufferString(test.body))
		request.Header.Set("Origin", test.origin)
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: test.session})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("target=%s body length=%d status=%d response=%s", test.target, len(test.body), response.Code, response.Body.String())
		}
	}
}

func TestReaderDeletionHTTPSuccessAndCompletedRetry(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	readers, err := readerstore.NewManager(filepath.Dir(store.Path()), 4)
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	insertTestUserWithID(t, store, testUserID, "Administrator", RoleAdmin, StatusActive)
	insertTestUserWithID(t, store, secondTestUserID, "Bob", RoleReader, StatusActive)
	if err := readers.Create(context.Background(), secondTestUserID); err != nil {
		t.Fatal(err)
	}
	admin, _ := accountByID(context.Background(), store.db, testUserID)
	adminSession, _ := NewSessionService(store).CreateAuthenticated(context.Background(), admin, 101)
	handler, err := NewHTTPHandler(store, HTTPConfig{})
	if err != nil {
		t.Fatal(err)
	}
	handler.now = func() time.Time { return time.Unix(200, 0) }
	handler.ConfigureDeletionQuiescer(readers, func(context.Context, readerstore.UserID) error { return nil })
	for _, username := range []string{"Bob", "ignored after completion"} {
		request := httptest.NewRequest(http.MethodDelete, "http://reader.local/api/auth/admin/readers/"+string(secondTestUserID), bytes.NewBufferString(`{"username":"`+username+`"}`))
		request.Header.Set("Origin", "http://reader.local")
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: adminSession.Token})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("username=%q status=%d body=%s", username, response.Code, response.Body.String())
		}
		var result map[string]string
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result["status"] != "complete" {
			t.Fatalf("result=%v error=%v", result, err)
		}
	}
}
