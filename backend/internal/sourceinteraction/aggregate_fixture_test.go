package sourceinteraction

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func TestDescribeUnmodifiedAggregateFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../test-booksources/test_光遇聚合_aggregated_booksource.json")
	if err != nil {
		t.Fatal(err)
	}
	var sources []booksource.BookSource
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	source := sources[0]
	source.ID = "aggregate-fixture"
	describer := NewDescriber(describerSourceStore{&source}, describerProfileStore{sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`)}}, analyzer.NewJSVM())
	view, err := describer.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Controls) != 34 {
		t.Fatalf("controls=%d", len(view.Controls))
	}
	for _, control := range view.Controls {
		if control.Unsupported != "" {
			t.Fatalf("unsupported control=%+v", control)
		}
	}
}

func TestActUnmodifiedAggregateFallbackSettings(t *testing.T) {
	raw, err := os.ReadFile("../../../test-booksources/test_光遇聚合_aggregated_booksource.json")
	if err != nil {
		t.Fatal(err)
	}
	var sources []booksource.BookSource
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	source := sources[0]
	source.ID = "aggregate-fixture"
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`), Authentication: json.RawMessage(`{}`)}}
	service := NewDescriber(describerSourceStore{&source}, profiles, analyzer.NewJSVM())

	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	listen := interactionActionID(t, view, "🎧搜索听书")
	result, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: view.Revision, ActionID: listen})
	if err != nil {
		t.Fatal(err)
	}
	settings := sourceprofile.DecodeSettings(profiles.profile.Settings)
	variable := map[string]interface{}{}
	if err := json.Unmarshal([]byte(settings.Variable), &variable); err != nil {
		t.Fatal(err)
	}
	more, _ := variable["更多设置"].(map[string]interface{})
	if more["搜索模式"] != "听书" {
		t.Fatalf("settings=%s effects=%+v", settings.Variable, result.Effects)
	}

	findSource := interactionActionID(t, result.View, "设置发现页来源")
	_, err = service.Act(t.Context(), source.ID, ActionRequest{
		Revision: result.View.Revision,
		ActionID: findSource,
		Values:   map[string]string{"发现页来源(支持的平台请前往源变量中查看)": "七猫"},
	})
	if err != nil {
		t.Fatal(err)
	}
	settings = sourceprofile.DecodeSettings(profiles.profile.Settings)
	variable = map[string]interface{}{}
	if err := json.Unmarshal([]byte(settings.Variable), &variable); err != nil {
		t.Fatal(err)
	}
	if variable["发现页来源"] != "七猫" {
		t.Fatalf("settings=%s", settings.Variable)
	}
}

func interactionActionID(t *testing.T, view View, label string) string {
	t.Helper()
	for _, control := range view.Controls {
		if control.Label == label {
			return control.ActionID
		}
	}
	t.Fatalf("action %q not found", label)
	return ""
}

func TestUnmodifiedAggregateSettingsAwaitPersistsReturnedSelection(t *testing.T) {
	raw, err := os.ReadFile("../../../test-booksources/test_光遇聚合_aggregated_booksource.json")
	if err != nil {
		t.Fatal(err)
	}
	var sources []booksource.BookSource
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	source := sources[0]
	source.ID = "aggregate-fixture"
	settings := json.RawMessage(`{"variable":"{\"云端配置\":{\"hosts\":[\"https://v1.gyks.cf\"]},\"线路\":\"https://v1.gyks.cf\"}"}`)
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: settings, Authentication: json.RawMessage(`{}`)}}
	service := NewDescriber(describerSourceStore{&source}, profiles, analyzer.NewJSVM())
	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: view.Revision, ActionID: interactionActionID(t, view, "⚙️ 书源设置")})
	if err != nil || len(result.Effects) == 0 || result.Effects[0].continuation == nil {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	body := `<html><span id="searchModeValue">小说</span><span id="showImageSwitchValue">true</span><span id="fullDescSwitchValue">true</span><span id="bookDiscussionSwitchValue">false</span><span id="syncBookshelfSwitchValue">false</span><span id="syncPopupSwitchValue">true</span><span id="networkModeValue">服务器</span><span id="forceSearchSwitchValue">false</span><span id="showSourceInTocValue">true</span><span id="novelSource">七猫</span><span id="audioSource">全部</span><span id="comicSource">全部</span><span id="dramaSource">全部</span></html>`
	if _, err := service.Resume(t.Context(), source.ID, *result.Effects[0].continuation, body); err != nil {
		t.Fatal(err)
	}
	captured := sourceprofile.DecodeSettings(profiles.profile.Settings)
	var values map[string]interface{}
	if err := json.Unmarshal([]byte(captured.Variable), &values); err != nil {
		t.Fatal(err)
	}
	more, _ := values["更多设置"].(map[string]interface{})
	if more["小说"] != "七猫" {
		t.Fatalf("settings=%s", captured.Variable)
	}
}

func TestUnmodifiedAggregateSettingsEmitsBoundedHTMLDataDocument(t *testing.T) {
	raw, err := os.ReadFile("../../../test-booksources/test_光遇聚合_aggregated_booksource.json")
	if err != nil {
		t.Fatal(err)
	}
	var sources []booksource.BookSource
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	source := sources[0]
	source.ID = "aggregate-fixture"
	settings := json.RawMessage(`{"variable":"{\"云端配置\":{\"hosts\":[\"https://v1.gyks.cf\"]},\"线路\":\"https://v1.gyks.cf\"}"}`)
	profiles := &mutableProfileStore{profile: sourceprofile.Profile{SourceID: source.ID, Settings: settings, Authentication: json.RawMessage(`{}`)}}
	service := NewDescriber(describerSourceStore{&source}, profiles, analyzer.NewJSVM())
	view, err := service.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Act(t.Context(), source.ID, ActionRequest{Revision: view.Revision, ActionID: interactionActionID(t, view, "⚙️ 书源设置")})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Effects) == 0 || result.Effects[0].Type != "browser_required" || !strings.HasPrefix(result.Effects[0].URL, "data:text/html;base64,") {
		t.Fatalf("effects=%+v", result.Effects)
	}
	if err := validateBrowserURL(result.Effects[0].URL); err != nil {
		t.Fatal(err)
	}
}
