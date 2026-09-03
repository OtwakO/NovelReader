package sourceprofile

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/otwako/novelreader/internal/sourceexec"
)

func TestApplyAuthenticationReplacesPersistedCookies(t *testing.T) {
	session := sourceexec.NewSourceSession()
	if err := session.SetCookie("https://source.test", "stale", "old"); err != nil {
		t.Fatal(err)
	}
	ApplyAuthentication(session, Authentication{Cookies: map[string]string{"https://source.test": "current=value"}})
	if got := session.JarCookieHeader("https://source.test"); got != "current=value" {
		t.Fatalf("cookies=%q", got)
	}
}

func TestAuthenticationRoundTripPreservesCookiePathScope(t *testing.T) {
	session := sourceexec.NewSourceSession()
	if err := session.SetCookies("https://identity.example.test/account", []*http.Cookie{{Name: "sid", Value: "secret", Path: "/account"}}); err != nil {
		t.Fatal(err)
	}
	authentication := CaptureAuthentication(session, Authentication{})
	fresh := sourceexec.NewSourceSession()
	ApplyAuthentication(fresh, authentication)
	if got := fresh.GetCookie("https://identity.example.test/account/status", "sid"); got != "secret" {
		t.Fatalf("account cookie=%q authentication=%+v", got, authentication)
	}
	if got := fresh.GetCookie("https://identity.example.test/public", "sid"); got != "" {
		t.Fatalf("path-scoped cookie leaked to public path: %q", got)
	}
}

func TestCaptureStateKeepsSettingsSeparateFromAuthentication(t *testing.T) {
	session := sourceexec.NewSourceSession()
	session.PutVariable("source-key", "portable")
	session.PutMemory("mode", "cloud")
	if err := session.SetCookie("https://source.test", "sid", "secret"); err != nil {
		t.Fatal(err)
	}
	settings := CaptureSettings(session, "source-key", Settings{})
	authentication := CaptureAuthentication(session, Authentication{})
	settingsJSON, _ := json.Marshal(settings)
	if string(settingsJSON) != `{"variable":"portable","values":{"mode":"cloud"}}` {
		t.Fatalf("settings=%s", settingsJSON)
	}
	if authentication.Cookies["https://source.test"] != "sid=secret" {
		t.Fatalf("authentication=%+v", authentication)
	}
}
