// Explore controls mutate only their retained source session and refresh catalogs explicitly.
package book

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/analyzer"
	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/fetcher"
)

func TestPinnedExploreSelectRefreshesCatalogInSameSession(t *testing.T) {
	source := pinnedExploreSource(t, 916)
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	control := findExploreEntry(t, catalog, "选择榜单")
	if control.Value != "完结榜" {
		t.Fatalf("default=%q", control.Value)
	}
	value := "收藏榜"
	refreshed, err := searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: control.ID, Value: &value})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.SessionID != catalog.SessionID || findExploreEntry(t, refreshed, "选择榜单").Value != value || !findExploreEntry(t, refreshed, value).Selectable {
		t.Fatalf("refreshed=%+v", refreshed)
	}
}

func TestExploreControlsValidateAndRetainValues(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://controls.test", BookSourceName: "Controls", EnabledExplore: true,
		ExploreURL: `[
			{title:'Text',type:'text',default:'ignored',action:"infoMap['Mirror']=infoMap['Text']"},
			{title:'Button',type:'button',action:"infoMap['Text']='pressed'"},
			{title:'Toggle',type:'toggle',chars:['A','B']},
			{title:'Select',type:'select',chars:['One','Two'],default:'Two'}
		]`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	if findExploreEntry(t, catalog, "Text").Value != "" || findExploreEntry(t, catalog, "Toggle").Value != "A" || findExploreEntry(t, catalog, "Select").Value != "Two" {
		t.Fatalf("defaults=%+v", catalog.Entries)
	}

	text := "typed"
	catalog, err = searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-0", Value: &text})
	if err != nil || findExploreEntry(t, catalog, "Text").Value != text {
		t.Fatalf("text catalog=%+v err=%v", catalog, err)
	}
	catalog, err = searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-1"})
	if err != nil || findExploreEntry(t, catalog, "Text").Value != "pressed" {
		t.Fatalf("button catalog=%+v err=%v", catalog, err)
	}
	toggle := "B"
	catalog, err = searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-2", Value: &toggle})
	if err != nil || findExploreEntry(t, catalog, "Toggle").Value != toggle {
		t.Fatalf("toggle catalog=%+v err=%v", catalog, err)
	}

	invalid := "missing"
	_, err = searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-3", Value: &invalid})
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "invalid_control_value" {
		t.Fatalf("invalid option error=%T %v", err, err)
	}
	_, err = searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-1", Value: &invalid})
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "invalid_control_value" {
		t.Fatalf("button value error=%T %v", err, err)
	}
}

func TestExploreRefreshInvalidatesQueuedCategoryID(t *testing.T) {
	actionStarted := make(chan struct{})
	releaseAction := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(actionStarted)
		<-releaseAction
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	vm := analyzer.NewJSVM()
	vm.SetFetcher(fetcher.NewWithTimeout(time.Second))
	source := booksource.BookSource{
		BookSourceURL: "https://stale.test", EnabledExplore: true,
		ExploreURL: `@js:JSON.stringify([{title:'Switch',type:'select',chars:['Old','New'],default:'Old',action:"java.ajax('` + server.URL + `');java.refreshExplore()"},{title:String(infoMap['Switch']||'Old'),url:'/books'}])`,
	}
	searcher := NewSearcher(nil, vm, nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	oldCategoryID := findExploreEntry(t, catalog, "Old").ID
	value := "New"
	updated := make(chan ExploreCatalog, 1)
	updateErr := make(chan error, 1)
	go func() {
		result, err := searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-0", Value: &value})
		updated <- result
		updateErr <- err
	}()
	<-actionStarted
	pageErr := make(chan error, 1)
	go func() {
		_, err := searcher.GetExplorePage(t.Context(), ExplorePageRequest{SessionID: catalog.SessionID, CategoryID: oldCategoryID, Page: 1})
		pageErr <- err
	}()
	deadline := time.After(time.Second)
	for {
		searcher.explore.mu.Lock()
		leases := searcher.explore.leases[catalog.SessionID]
		searcher.explore.mu.Unlock()
		if leases >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("queued page did not acquire session lease")
		default:
			runtime.Gosched()
		}
	}
	close(releaseAction)
	refreshed := <-updated
	if err := <-updateErr; err != nil {
		t.Fatal(err)
	}
	if findExploreEntry(t, refreshed, "New").ID == oldCategoryID {
		t.Fatalf("refresh reused stale category ID %q", oldCategoryID)
	}
	if err := <-pageErr; err == nil {
		t.Fatal("queued stale category request succeeded")
	} else if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "invalid_category" {
		t.Fatalf("queued page error=%T %v", err, err)
	}
}

func TestExploreControlRejectsMissingSessionControlAndURL(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://validation.test", EnabledExplore: true,
		ExploreURL: `[{title:'Books',url:'/books'},{title:'Text',type:'text'}]`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	value := "x"
	for _, test := range []struct {
		request ExploreControlRequest
		code    string
	}{
		{request: ExploreControlRequest{SessionID: "missing", ControlID: "entry-1", Value: &value}, code: "session_not_found"},
		{request: ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "missing", Value: &value}, code: "control_not_found"},
		{request: ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-0", Value: &value}, code: "invalid_control_type"},
		{request: ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-1"}, code: "invalid_control_value"},
	} {
		_, err := searcher.UpdateExploreControl(t.Context(), test.request)
		if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != test.code {
			t.Fatalf("request=%+v error=%T %v, want %s", test.request, err, err, test.code)
		}
	}
}

func TestExploreControlRefreshFailureKeepsPriorCatalog(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://refresh-failure.test", EnabledExplore: true,
		ExploreURL: `@js:infoMap['Text'] ? '[' : JSON.stringify([{title:'Text',type:'text',action:'java.refreshExplore()'},{title:'Before',url:'/books'}])`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVM(), nil, exploreSourceFixtureStore{source: source}, nil)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	value := "retain"
	_, err = searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-0", Value: &value})
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "category_parse_failed" {
		t.Fatalf("error=%T %v", err, err)
	}
	session, release := searcher.explore.acquire(catalog.SessionID)
	defer release()
	if session.categories["entry-1"].Title != "Before" || exploreInfoMap(session.state)["Text"] != value {
		t.Fatalf("categories=%+v values=%+v", session.categories, exploreInfoMap(session.state))
	}
}

func TestExploreControlActionTimeoutIsTypedAndKeepsValue(t *testing.T) {
	source := booksource.BookSource{
		BookSourceURL: "https://timeout.test", EnabledExplore: true,
		ExploreURL: `[{title:'Text',type:'text',action:'while(true){}'}]`,
	}
	searcher := NewSearcher(nil, analyzer.NewJSVMWithPoolSize(1), nil, exploreSourceFixtureStore{source: source}, nil)
	searcher.SetWorkflowTimeout(20 * time.Millisecond)
	catalog, err := searcher.OpenExplore(t.Context(), source.BookSourceURL)
	if err != nil {
		t.Fatal(err)
	}
	value := "retained"
	_, err = searcher.UpdateExploreControl(t.Context(), ExploreControlRequest{SessionID: catalog.SessionID, ControlID: "entry-0", Value: &value})
	if exploreErr, ok := err.(*ExploreError); !ok || exploreErr.Code != "control_action_failed" {
		t.Fatalf("error=%T %v", err, err)
	}
	session, release := searcher.explore.acquire(catalog.SessionID)
	defer release()
	if got := exploreInfoMap(session.state)["Text"]; got != value {
		t.Fatalf("retained value=%v", got)
	}
}

func findExploreEntry(t *testing.T, catalog ExploreCatalog, title string) ExploreEntry {
	t.Helper()
	for _, entry := range catalog.Entries {
		if entry.Title == title {
			return entry
		}
	}
	t.Fatalf("entry %q missing from %+v", title, catalog.Entries)
	return ExploreEntry{}
}
