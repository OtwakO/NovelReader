package sourceinteraction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

const maxControls = 200

var (
	ErrUnsupportedControl = errors.New("sourceinteraction: unsupported control type")
	ErrStaleRevision      = errors.New("sourceinteraction: stale description revision")
	ErrActionNotFound     = errors.New("sourceinteraction: action not found")
)

type SourceStore interface {
	GetByID(string) (*booksource.BookSource, error)
}

type ProfileStore interface {
	Load(context.Context, string) (sourceprofile.Profile, error)
}

type Control struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Label       string   `json:"label"`
	Value       string   `json:"value,omitempty"`
	Options     []string `json:"options,omitempty"`
	ActionID    string   `json:"actionId,omitempty"`
	Unsupported string   `json:"unsupported,omitempty"`
}

type View struct {
	SourceID string    `json:"sourceId"`
	Title    string    `json:"title"`
	Revision string    `json:"revision"`
	Controls []Control `json:"controls"`
}

type description struct {
	view        View
	source      *booksource.BookSource
	profile     sourceprofile.Profile
	rawControls []interface{}
}

type Describer struct {
	sources  SourceStore
	profiles ProfileStore
	jsVM     *analyzer.JSVM
}

func NewDescriber(sources SourceStore, profiles ProfileStore, jsVM *analyzer.JSVM) *Describer {
	return &Describer{sources: sources, profiles: profiles, jsVM: jsVM}
}

func (d *Describer) Describe(ctx context.Context, sourceID string) (View, error) {
	current, err := d.describe(ctx, sourceID)
	if err != nil {
		return View{}, err
	}
	return current.view, nil
}

func (d *Describer) describe(ctx context.Context, sourceID string) (description, error) {
	source, err := d.sources.GetByID(sourceID)
	if err != nil {
		return description{}, err
	}
	if source == nil {
		return description{}, sourceprofile.ErrSourceNotInstalled
	}
	profile, err := d.profiles.Load(ctx, sourceID)
	if err != nil {
		return description{}, err
	}
	settings := sourceprofile.DecodeSettings(profile.Settings)
	session := sourceexec.NewSourceSession()
	sourceprofile.ApplySettings(session, source.BookSourceURL, settings)
	rawControls, err := d.evaluate(ctx, *source, session)
	if err != nil {
		return description{}, executionError("interaction_ui_failed", "Could not load source interaction controls", err, sourceexec.FailureJavaScriptRuntime)
	}
	controls, err := normalizeControls(rawControls)
	if err != nil {
		return description{}, executionError("interaction_ui_invalid", "Source interaction controls are invalid", err, sourceexec.FailureInvalidResult)
	}
	revisionInput, err := json.Marshal(struct {
		SourceID  string
		UpdatedAt int64
		Settings  json.RawMessage
		UI        string
	}{source.ID, source.UpdatedAt, profile.Settings, source.LoginUI})
	if err != nil {
		return description{}, err
	}
	digest := sha256.Sum256(revisionInput)
	view := View{SourceID: source.ID, Title: source.BookSourceName, Revision: hex.EncodeToString(digest[:12]), Controls: controls}
	return description{view: view, source: source, profile: profile, rawControls: rawControls}, nil
}

func (d *Describer) evaluate(ctx context.Context, source booksource.BookSource, session *sourceexec.SourceSession) ([]interface{}, error) {
	script := strings.TrimSpace(source.LoginUI)
	if script == "" {
		return []interface{}{}, nil
	}
	if json.Valid([]byte(script)) {
		var values []interface{}
		if err := json.Unmarshal([]byte(script), &values); err != nil {
			return nil, err
		}
		return values, nil
	}
	if d.jsVM == nil {
		return nil, fmt.Errorf("sourceinteraction: JavaScript engine unavailable")
	}
	dynamic := strings.HasPrefix(strings.ToLower(script), "@js:")
	if dynamic {
		script = strings.TrimSpace(script[len("@js:"):])
	}
	if !dynamic {
		script = "(" + script + ")"
	}
	wrapped := source.JSLib + "\n" + script
	bindings := analyzer.URLBindings(&analyzer.URLContext{Source: source.ScriptData(), JSLib: source.JSLib}, source.BookSourceURL, session)
	value, err := d.jsVM.EvalContext(ctx, wrapped, "", source.BookSourceURL, bindings)
	if err != nil {
		return nil, fmt.Errorf("sourceinteraction: evaluate loginUi: %w", err)
	}
	values, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("sourceinteraction: loginUi must evaluate to an array")
	}
	return values, nil
}

func normalizeControls(values []interface{}) ([]Control, error) {
	if len(values) > maxControls {
		return nil, fmt.Errorf("sourceinteraction: loginUi contains too many controls")
	}
	controls := make([]Control, 0, len(values))
	for index, value := range values {
		row, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		controlType := strings.ToLower(strings.TrimSpace(analyzer.ToString(row["type"])))
		label := strings.TrimSpace(analyzer.ToString(row["name"]))
		control := Control{ID: fmt.Sprintf("control-%d", index), Type: controlType, Label: label}
		switch controlType {
		case "text", "password", "input":
			control.Value = analyzer.ToString(row["value"])
		case "button":
			control.ActionID = fmt.Sprintf("action-%d", index)
		case "toggle":
			control.ActionID = fmt.Sprintf("action-%d", index)
			control.Options = stringList(row["chars"])
			control.Value = analyzer.ToString(row["value"])
		case "select":
			control.ActionID = fmt.Sprintf("action-%d", index)
			control.Options = stringList(row["chars"])
			control.Value = analyzer.ToString(row["value"])
		case "", "label":
			control.Type = "label"
		default:
			control.Unsupported = fmt.Sprintf("unsupported source control type %q", controlType)
		}
		controls = append(controls, control)
	}
	return controls, nil
}

func stringList(value interface{}) []string {
	values, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, analyzer.ToString(value))
	}
	return result
}
