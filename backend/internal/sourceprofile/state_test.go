package sourceprofile

import (
	"encoding/json"
	"testing"

	"github.com/otwako/novelreader/internal/sourceexec"
)

func TestApplyAuthenticationHydratesLoginInfo(t *testing.T) {
	session := sourceexec.NewSourceSession()
	ApplyAuthentication(session, Authentication{LoginInfo: map[string]string{"Mode": "cloud"}})
	if got := session.LoginInfo()["Mode"]; got != "cloud" {
		t.Fatalf("login info=%q", got)
	}

	authentication := CaptureAuthentication(session, Authentication{})
	if authentication.LoginInfo["Mode"] != "cloud" {
		t.Fatalf("captured authentication=%+v", authentication)
	}
}

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
