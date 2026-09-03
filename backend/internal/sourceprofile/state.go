package sourceprofile

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/otwako/novelreader/internal/sourceexec"
)

// Settings is NovelReader's portable source-state document.
// Values are opaque to NovelReader and interpreted only by the BookSource program.
type Settings struct {
	Variable string            `json:"variable,omitempty"`
	Values   map[string]string `json:"values,omitempty"`
}

// Authentication is NovelReader's backup-excluded source login document.
type Authentication struct {
	LoginInfo   map[string]string `json:"loginInfo,omitempty"`
	LoginHeader string            `json:"loginHeader,omitempty"`
	Cookies     map[string]string `json:"cookies,omitempty"`
}

func DecodeSettings(document json.RawMessage) Settings {
	var settings Settings
	if err := json.Unmarshal(document, &settings); err != nil {
		return Settings{}
	}
	return settings
}

func DecodeAuthentication(document json.RawMessage) Authentication {
	var authentication Authentication
	if err := json.Unmarshal(document, &authentication); err != nil {
		return Authentication{}
	}
	return authentication
}

func ApplySettings(session *sourceexec.SourceSession, sourceKey string, settings Settings) {
	if session == nil {
		return
	}
	if settings.Variable != "" {
		session.PutVariable(sourceKey, settings.Variable)
	}
	for key, value := range settings.Values {
		session.PutMemory(key, value)
	}
}

func CaptureSettings(session *sourceexec.SourceSession, sourceKey string, previous Settings) Settings {
	if session == nil {
		return previous
	}
	previous.Variable = session.GetVariable(sourceKey)
	previous.Values = session.StringMemory()
	return previous
}

func ApplyAuthentication(session *sourceexec.SourceSession, authentication Authentication) {
	if session == nil {
		return
	}
	session.SetLoginInfo(authentication.LoginInfo)
	if authentication.LoginHeader != "" {
		session.SetLoginHeader(authentication.LoginHeader)
	}
	for rawURL, cookie := range authentication.Cookies {
		pairs := strings.Split(cookie, ";")
		cookies := make([]*http.Cookie, 0, len(pairs))
		for _, pair := range pairs {
			parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(parts) == 2 {
				cookies = append(cookies, &http.Cookie{Name: strings.TrimSpace(parts[0]), Value: strings.TrimSpace(parts[1]), Path: "/"})
			}
		}
		_ = session.RemoveCookies(rawURL)
		_ = session.SetCookies(rawURL, cookies)
	}
}

func CaptureAuthentication(session *sourceexec.SourceSession, previous Authentication) Authentication {
	if session == nil {
		return previous
	}
	previous.LoginInfo = session.LoginInfo()
	previous.LoginHeader = session.LoginHeader()
	if previous.Cookies == nil {
		previous.Cookies = make(map[string]string)
	}
	for _, rawURL := range session.CookieURLs() {
		previous.Cookies[rawURL] = ""
	}
	for rawURL := range previous.Cookies {
		if cookie := session.JarCookieHeader(rawURL); cookie == "" {
			delete(previous.Cookies, rawURL)
		} else {
			previous.Cookies[rawURL] = cookie
		}
	}
	return previous
}
