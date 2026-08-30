package sourceinteraction

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

type describerSourceStore struct{ source *booksource.BookSource }

func (s describerSourceStore) GetByID(id string) (*booksource.BookSource, error) {
	if s.source == nil || s.source.ID != id {
		return nil, nil
	}
	copy := *s.source
	return &copy, nil
}

type describerProfileStore struct{ profile sourceprofile.Profile }

func (s describerProfileStore) Load(context.Context, string) (sourceprofile.Profile, error) {
	return s.profile, nil
}

func TestDescribeNormalizesStaticControlsWithoutExposingAuthentication(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", BookSourceName: "Fixture", UpdatedAt: 12,
		LoginUI: `[{"name":"账号","type":"text"},{"name":"密码","type":"password"},{"name":"登录","type":"button","action":"login()"}]`}
	describer := NewDescriber(describerSourceStore{source}, describerProfileStore{sourceprofile.Profile{
		SourceID: source.ID, Settings: json.RawMessage(`{"values":{"account":"portable"}}`), Authentication: json.RawMessage(`{"password":"secret"}`),
	}}, analyzer.NewJSVM())
	view, err := describer.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.SourceID != source.ID || view.Title != "Fixture" || view.Revision == "" || len(view.Controls) != 3 {
		t.Fatalf("view=%+v", view)
	}
	if view.Controls[0].Type != "text" || view.Controls[1].Type != "password" || view.Controls[2].ActionID != "action-2" {
		t.Fatalf("controls=%+v", view.Controls)
	}
	encoded, _ := json.Marshal(view)
	if string(encoded) == "" || strings.Contains(string(encoded), "secret") {
		t.Fatalf("response exposed authentication: %s", encoded)
	}
}

func TestDescribeEvaluatesObjectLiteralAndDynamicSourceState(t *testing.T) {
	tests := []struct {
		name    string
		loginUI string
		want    string
	}{
		{name: "object literal", loginUI: `[{name:'Mode',type:'select',chars:['A','B'],value:'B'}]`, want: "B"},
		{name: "dynamic", loginUI: `@js:[{name:source.get('mode') + source.getVariable(),type:'button',action:'refresh()'}]`, want: "Cloudcatalog"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", BookSourceName: "Fixture", LoginUI: test.loginUI}
			describer := NewDescriber(describerSourceStore{source}, describerProfileStore{sourceprofile.Profile{
				SourceID: source.ID, Settings: json.RawMessage(`{"variable":"catalog","values":{"mode":"Cloud"}}`), Authentication: json.RawMessage(`{}`),
			}}, analyzer.NewJSVM())
			view, err := describer.Describe(t.Context(), source.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(view.Controls) != 1 {
				t.Fatalf("controls=%+v", view.Controls)
			}
			if test.name == "object literal" {
				if view.Controls[0].Value != test.want || len(view.Controls[0].Options) != 2 {
					t.Fatalf("control=%+v", view.Controls[0])
				}
			} else if view.Controls[0].Label != test.want {
				t.Fatalf("control=%+v", view.Controls[0])
			}
		})
	}
}

func TestDescribeReportsUnsupportedControls(t *testing.T) {
	source := &booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", LoginUI: `[{"name":"Custom","type":"slider"}]`}
	describer := NewDescriber(describerSourceStore{source}, describerProfileStore{sourceprofile.Profile{SourceID: source.ID, Settings: json.RawMessage(`{}`)}}, analyzer.NewJSVM())
	view, err := describer.Describe(t.Context(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Controls) != 1 || view.Controls[0].Unsupported == "" {
		t.Fatalf("controls=%+v", view.Controls)
	}
}
