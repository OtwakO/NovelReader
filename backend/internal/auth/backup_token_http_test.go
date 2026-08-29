package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBackupTokenHTTPRequiresPasswordForRestoreScope(t *testing.T) {
	store := openTestStore(t)
	defer store.Close()
	account, err := NewAccountService(store).CreateReaderAccount(context.Background(), testUserID, "Alice", "correct horse battery staple", 100)
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

	perform := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://reader.local/api/auth/backup-tokens", bytes.NewBufferString(body))
		request.Header.Set("Origin", "http://reader.local")
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session.Token})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	exportOnly := perform(`{"name":"Nightly","canExport":true,"canRestore":false}`)
	if exportOnly.Code != http.StatusCreated {
		t.Fatalf("export status=%d body=%s", exportOnly.Code, exportOnly.Body.String())
	}
	wrongPassword := perform(`{"name":"Restore","canRestore":true,"currentPassword":"wrong password value"}`)
	if wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status=%d body=%s", wrongPassword.Code, wrongPassword.Body.String())
	}
	restore := perform(`{"name":"Restore","canRestore":true,"currentPassword":"correct horse battery staple"}`)
	if restore.Code != http.StatusCreated {
		t.Fatalf("restore status=%d body=%s", restore.Code, restore.Body.String())
	}
	var credential BackupTokenCredential
	if err := json.Unmarshal(restore.Body.Bytes(), &credential); err != nil || credential.Token == "" || !credential.CanRestore {
		t.Fatalf("credential=%#v error=%v", credential, err)
	}
}
