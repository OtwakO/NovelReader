package sourceinteraction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

type MutableProfileStore interface {
	ProfileStore
	SaveSettings(context.Context, string, json.RawMessage) error
	SaveAuthentication(context.Context, string, json.RawMessage) error
	ClearAuthentication(context.Context, string) error
	ResetSettings(context.Context, string) error
	Reset(context.Context, string) error
}

type Effect struct {
	Type             string `json:"type"`
	Message          string `json:"message,omitempty"`
	URL              string `json:"url,omitempty"`
	Title            string `json:"title,omitempty"`
	Await            bool   `json:"await,omitempty"`
	BrowserRequestID string `json:"browserRequestId,omitempty"`
}

type ActionRequest struct {
	Revision    string            `json:"revision"`
	ActionID    string            `json:"actionId"`
	Values      map[string]string `json:"values"`
	IsLongClick bool              `json:"isLongClick,omitempty"`
}

type ActionResult struct {
	View    View     `json:"view"`
	Effects []Effect `json:"effects"`
}

type ResetScope string

const (
	ResetLogin    ResetScope = "login"
	ResetSettings ResetScope = "settings"
	ResetAll      ResetScope = "all"
)

func (d *Describer) Reset(ctx context.Context, sourceID string, scope ResetScope) (View, error) {
	profiles, ok := d.profiles.(MutableProfileStore)
	if !ok {
		return View{}, fmt.Errorf("sourceinteraction: profile store is read-only")
	}
	if _, err := d.describe(ctx, sourceID); err != nil {
		return View{}, err
	}
	var err error
	switch scope {
	case ResetLogin:
		err = profiles.ClearAuthentication(ctx, sourceID)
	case ResetSettings:
		err = profiles.ResetSettings(ctx, sourceID)
	case ResetAll:
		err = profiles.Reset(ctx, sourceID)
	default:
		return View{}, fmt.Errorf("sourceinteraction: invalid reset scope")
	}
	if err != nil {
		return View{}, err
	}
	return d.Describe(ctx, sourceID)
}

func (d *Describer) Act(ctx context.Context, sourceID string, request ActionRequest) (ActionResult, error) {
	profiles, ok := d.profiles.(MutableProfileStore)
	if !ok {
		return ActionResult{}, fmt.Errorf("sourceinteraction: profile store is read-only")
	}
	current, err := d.describe(ctx, sourceID)
	if err != nil {
		return ActionResult{}, err
	}
	if request.Revision != current.view.Revision {
		return ActionResult{}, ErrStaleRevision
	}
	row, err := actionRow(current.rawControls, current.view.Controls, request.ActionID)
	if err != nil {
		return ActionResult{}, err
	}
	action := strings.TrimSpace(analyzer.ToString(row["action"]))
	settings := sourceprofile.DecodeSettings(current.profile.Settings)
	authentication := sourceprofile.DecodeAuthentication(current.profile.Authentication)
	originalLoginInfo := cloneValues(authentication.LoginInfo)
	session := sourceexec.NewSourceSession()
	sourceprofile.ApplySettings(session, current.source.BookSourceURL, settings)
	sourceprofile.ApplyAuthentication(session, authentication)
	effects := make([]Effect, 0, 2)

	if strings.HasPrefix(action, "http://") || strings.HasPrefix(action, "https://") {
		effects = append(effects, Effect{Type: "open_external", URL: action})
	} else if action != "" {
		if d.jsVM == nil {
			return ActionResult{}, fmt.Errorf("sourceinteraction: JavaScript engine unavailable")
		}
		bridge := interactionBridge(&authentication, originalLoginInfo, &effects)
		bindings := analyzer.URLBindings(&analyzer.URLContext{JSLib: current.source.JSLib}, current.source.BookSourceURL, session)
		bindings["source"] = map[string]interface{}{
			"bookSourceUrl":  current.source.BookSourceURL,
			"bookSourceName": current.source.BookSourceName,
		}
		bindings["jsBridge"] = bridge
		bindings["isLongClick"] = request.IsLongClick
		script := current.source.JSLib + "\n" + current.source.LoginURL + "\n" + action
		if _, err := d.jsVM.EvalContext(ctx, script, request.Values, current.source.BookSourceURL, bindings); err != nil {
			return ActionResult{}, fmt.Errorf("sourceinteraction: execute action: %w", err)
		}
	}

	settings = sourceprofile.CaptureSettings(session, current.source.BookSourceURL, settings)
	if sameValues(authentication.LoginInfo, originalLoginInfo) {
		authentication.LoginInfo = cloneValues(request.Values)
	}
	authentication = sourceprofile.CaptureAuthentication(session, authentication)
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return ActionResult{}, err
	}
	authenticationJSON, err := json.Marshal(authentication)
	if err != nil {
		return ActionResult{}, err
	}
	settingsChanged := !sameDocument(settingsJSON, current.profile.Settings)
	authenticationChanged := !sameDocument(authenticationJSON, current.profile.Authentication)
	if settingsChanged {
		if err := profiles.SaveSettings(ctx, sourceID, settingsJSON); err != nil {
			return ActionResult{}, err
		}
	}
	if authenticationChanged {
		if err := profiles.SaveAuthentication(ctx, sourceID, authenticationJSON); err != nil {
			return ActionResult{}, err
		}
	}
	view, err := d.Describe(ctx, sourceID)
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{View: view, Effects: effects}, nil
}

// RegisterBrowserRequests replaces source URLs with one-use opaque Reader-runtime references.
func RegisterBrowserRequests(effects []Effect, sessions *BrowserSessions) []Effect {
	if sessions == nil {
		return effects
	}
	for index := range effects {
		if effects[index].Type != "browser_required" || effects[index].URL == "" {
			continue
		}
		effects[index].BrowserRequestID = sessions.Register(BrowserRequest{URL: effects[index].URL, Title: effects[index].Title})
		effects[index].URL = ""
	}
	return effects
}

func actionRow(values []interface{}, controls []Control, actionID string) (map[string]interface{}, error) {
	const prefix = "action-"
	if !strings.HasPrefix(actionID, prefix) {
		return nil, ErrActionNotFound
	}
	exposed := false
	for _, control := range controls {
		if control.ActionID == actionID {
			exposed = true
			break
		}
	}
	if !exposed {
		return nil, ErrActionNotFound
	}
	index, err := strconv.Atoi(strings.TrimPrefix(actionID, prefix))
	if err != nil || index < 0 || index >= len(values) {
		return nil, ErrActionNotFound
	}
	row, ok := values[index].(map[string]interface{})
	if !ok {
		return nil, ErrActionNotFound
	}
	return row, nil
}

func sameDocument(left, right json.RawMessage) bool {
	var leftValue, rightValue interface{}
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return string(left) == string(right)
	}
	leftJSON, _ := json.Marshal(leftValue)
	rightJSON, _ := json.Marshal(rightValue)
	return bytes.Equal(leftJSON, rightJSON)
}

func cloneValues(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func sameValues(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func interactionBridge(authentication *sourceprofile.Authentication, originalLoginInfo map[string]string, effects *[]Effect) *analyzer.JSBridge {
	return &analyzer.JSBridge{
		Toast:     func(message interface{}) { appendNotice(effects, message) },
		LongToast: func(message interface{}) { appendNotice(effects, message) },
		RefreshExplore: func() {
			*effects = append(*effects, Effect{Type: "refresh_explore"})
		},
		StartBrowser: func(url, title string, await bool) {
			*effects = append(*effects, Effect{Type: "browser_required", URL: url, Title: title, Await: await})
		},
		Open: func(url string) {
			*effects = append(*effects, Effect{Type: "open_external", URL: url})
		},
		SearchBook: func(keyword string) {
			*effects = append(*effects, Effect{Type: "search", Message: keyword})
		},
		GetLoginInfo: func() string {
			if originalLoginInfo == nil {
				return ""
			}
			value, _ := json.Marshal(originalLoginInfo)
			return string(value)
		},
		GetLoginInfoMap: func() map[string]string { return cloneValues(originalLoginInfo) },
		PutLoginInfo: func(value string) bool {
			var loginInfo map[string]string
			if err := json.Unmarshal([]byte(value), &loginInfo); err != nil {
				return false
			}
			authentication.LoginInfo = loginInfo
			return true
		},
		RemoveLoginInfo: func() { authentication.LoginInfo = nil },
	}
}

func appendNotice(effects *[]Effect, message interface{}) {
	*effects = append(*effects, Effect{Type: "notice", Message: fmt.Sprint(message)})
}
