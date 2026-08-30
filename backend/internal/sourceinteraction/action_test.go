package sourceinteraction

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

type mutableProfileStore struct {
	profile             sourceprofile.Profile
	settingsSaves       int
	authenticationSaves int
}

func (s *mutableProfileStore) Load(context.Context, string) (sourceprofile.Profile, error) {
	return s.profile, nil
}

func (s *mutableProfileStore) SaveSettings(_ context.Context, _ string, value json.RawMessage) error {
	s.settingsSaves++
	s.profile.Settings = append(json.RawMessage(nil), value...)
	return nil
}

func (s *mutableProfileStore) SaveAuthentication(_ context.Context, _ string, value json.RawMessage) error {
	s.authenticationSaves++
	s.profile.Authentication = append(json.RawMessage(nil), value...)
	return nil
}

func TestActPersistsCurrentValuesVariablesAndTypedEffects(t *testing.T) {
	source := &booksource.BookSource{
		ID: "source-a", BookSourceURL: "https://source.test", BookSourceName: "Fixture", UpdatedAt: 1,
		LoginURL: `function save(){source.putVariable(JSON.stringify({mode:result.Mode})); source.putLoginInfo(JSON.stringify(result)); java.longToast("saved"); java.refreshExplore(); java.searchBook("query");}`,
		LoginUI:  `[{"name":"Mode","type":"text"},{"name":"Save","type":"button","action":"save()"}]`,
	}
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{}`)}}
	service := NewDescriber(describerSourceStore{source}, profiles, analyzer.NewJSVM())
	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Act(t.Context(), source.ID, ActionRequest{
		Revision: view.Revision, ActionID: "action-1", Values: map[string]string{"Mode": "cloud"},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings := sourceprofile.DecodeSettings(profiles.profile.Settings)
	authentication := sourceprofile.DecodeAuthentication(profiles.profile.Authentication)
	if settings.Variable != `{"mode":"cloud"}` || authentication.LoginInfo["Mode"] != "cloud" {
		t.Fatalf("settings=%+v authentication=%+v", settings, authentication)
	}
	if len(result.Effects) != 3 || result.Effects[0].Type != "notice" || result.Effects[1].Type != "refresh_explore" || result.Effects[2].Type != "search" {
		t.Fatalf("effects=%+v", result.Effects)
	}
	if result.View.Revision == view.Revision {
		t.Fatal("revision did not change after settings mutation")
	}
}

func TestActReadsPreviousLoginInfoWhileResultCarriesSubmittedValues(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", UpdatedAt: 1,
		LoginUI:  `[{"name":"Save","type":"button","action":"save()"}]`,
		LoginURL: `function save(){let old=JSON.parse(source.getLoginInfo()); source.put("seen", old.Mode + ":" + result.Mode);}`}
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{"loginInfo":{"Mode":"old"}}`)}}
	service := NewDescriber(describerSourceStore{source}, profiles, analyzer.NewJSVM())
	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: view.Revision, ActionID: "action-0", Values: map[string]string{"Mode": "new"}}); err != nil {
		t.Fatal(err)
	}
	settings := sourceprofile.DecodeSettings(profiles.profile.Settings)
	authentication := sourceprofile.DecodeAuthentication(profiles.profile.Authentication)
	if settings.Values["seen"] != "old:new" || authentication.LoginInfo["Mode"] != "new" {
		t.Fatalf("settings=%+v authentication=%+v", settings, authentication)
	}
}

func TestActPersistsControlValuesWithoutJavaScriptAction(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", UpdatedAt: 1,
		LoginUI: `[{"name":"Enabled","type":"toggle","value":"0","chars":["off","on"]}]`}
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{}`)}}
	service := NewDescriber(describerSourceStore{source}, profiles, analyzer.NewJSVM())
	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: view.Revision, ActionID: "action-0", Values: map[string]string{"Enabled": "1"}}); err != nil {
		t.Fatal(err)
	}
	authentication := sourceprofile.DecodeAuthentication(profiles.profile.Authentication)
	if authentication.LoginInfo["Enabled"] != "1" {
		t.Fatalf("authentication=%+v", authentication)
	}
}

func TestActDoesNotRewriteSemanticallyUnchangedDocuments(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", UpdatedAt: 1,
		LoginUI: `[{"name":"Save","type":"button","action":"source.putLoginInfo(JSON.stringify(result))"}]`}
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{"loginInfo":{"Mode":"same"}}`)}}
	service := NewDescriber(describerSourceStore{source}, profiles, analyzer.NewJSVM())
	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: view.Revision, ActionID: "action-0", Values: map[string]string{"Mode": "same"}}); err != nil {
		t.Fatal(err)
	}
	if profiles.settingsSaves != 0 || profiles.authenticationSaves != 0 {
		t.Fatalf("settings saves=%d authentication saves=%d", profiles.settingsSaves, profiles.authenticationSaves)
	}
}

func TestActRejectsStaleOrUnexposedActions(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", UpdatedAt: 1,
		LoginUI: `[{"name":"Save","type":"button","action":"save()"}]`, LoginURL: `function save(){}`}
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{}`)}}
	service := NewDescriber(describerSourceStore{source}, profiles, analyzer.NewJSVM())
	if _, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: "stale", ActionID: "action-0"}); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale error=%v", err)
	}
	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: view.Revision, ActionID: "action-99"}); !errors.Is(err, ErrActionNotFound) {
		t.Fatalf("action error=%v", err)
	}
}
