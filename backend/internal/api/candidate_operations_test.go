package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/candidate"
)

func TestCandidateOperationHTTPStartSnapshotCancel(t *testing.T) {
	fixture := candidateFixtureServer(t, credibleFixtureContent("operation"), 30*time.Millisecond)
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	importWorkflowSource(t, server, fixture.URL)

	started := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions", candidateRequestBody(fixture.URL, nil))
	if started.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", started.Code, started.Body.String())
	}
	var snapshot candidate.Snapshot
	if err := json.Unmarshal(started.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.ID == "" || snapshot.State != candidate.StateRunning {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if snapshot.AutomaticCommit {
		t.Fatalf("detail operation unexpectedly marked automatic: %+v", snapshot)
	}

	current := performAPIRequest(server, http.MethodGet, "/api/candidate-resolutions/"+snapshot.ID, nil)
	if current.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", current.Code, current.Body.String())
	}
	cancelled := performAPIRequest(server, http.MethodDelete, "/api/candidate-resolutions/"+snapshot.ID, nil)
	if cancelled.Code != http.StatusNoContent {
		t.Fatalf("cancel status=%d body=%s", cancelled.Code, cancelled.Body.String())
	}
}

func TestCandidateOperationSSEPublishesAutomaticCommit(t *testing.T) {
	fixture := candidateFixtureServer(t, credibleFixtureContent("automatic"), 0)
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	importWorkflowSource(t, server, fixture.URL)
	body := candidateRequestBody(fixture.URL, nil)
	var input map[string]any
	if err := json.Unmarshal(body, &input); err != nil {
		t.Fatal(err)
	}
	input["shelveBookId"] = "stored"
	body, _ = json.Marshal(input)
	started := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions", body)
	var snapshot candidate.Snapshot
	if err := json.Unmarshal(started.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !snapshot.AutomaticCommit {
		t.Fatalf("automatic shelf intent missing from snapshot: %+v", snapshot)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/candidate-resolutions/"+snapshot.ID+"/events", nil)
	request.SetPathValue("id", snapshot.ID)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.standalone.handleStreamCandidateResolution(response, request)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("candidate SSE did not terminate after automatic commit")
	}
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"committed"`) {
		t.Fatalf("stream status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCandidateOperationHTTPCommitDoesNotRecrawl(t *testing.T) {
	fixture := candidateFixtureServer(t, credibleFixtureContent("verified"), 0)
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	importWorkflowSource(t, server, fixture.URL)
	var input map[string]any
	if err := json.Unmarshal(candidateRequestBody(fixture.URL, nil), &input); err != nil {
		t.Fatal(err)
	}
	input["variableMap"] = `{"request":"original"}`
	requestBody, _ := json.Marshal(input)
	started := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions", requestBody)
	var snapshot candidate.Snapshot
	if err := json.Unmarshal(started.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current := performAPIRequest(server, http.MethodGet, "/api/candidate-resolutions/"+snapshot.ID, nil)
		if err := json.Unmarshal(current.Body.Bytes(), &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.State == candidate.StateVerified {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if snapshot.State != candidate.StateVerified {
		t.Fatalf("state=%s message=%s", snapshot.State, snapshot.Message)
	}
	body, _ := json.Marshal(map[string]string{"bookId": "stored"})
	committed := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions/"+snapshot.ID+"/shelve", body)
	if committed.Code != http.StatusCreated {
		t.Fatalf("commit status=%d body=%s", committed.Code, committed.Body.String())
	}
	if err := json.Unmarshal(committed.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.StoredBook == nil || snapshot.StoredBook.VariableMap != input["variableMap"] || snapshot.Attempts[0].VariableMap != input["variableMap"] {
		t.Fatalf("HTTP handoff lost variables: %+v", snapshot)
	}
	retried := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions/"+snapshot.ID+"/shelve", body)
	if retried.Code != http.StatusCreated && retried.Code != http.StatusOK {
		t.Fatalf("retry status=%d body=%s", retried.Code, retried.Body.String())
	}
}

func candidateFixtureServer(t *testing.T, content string, delay time.Duration) *httptest.Server {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if delay > 0 {
			time.Sleep(delay)
		}
		switch r.URL.Path {
		case "/book":
			_, _ = fmt.Fprint(w, `<h1 class="name">Fixture Novel</h1><span class="author">Fixture Author</span><div class="intro">Verified introduction</div><a class="toc" href="/toc">目录</a>`)
		case "/toc":
			_, _ = fmt.Fprint(w, `<a class="chapter" href="/chapter/1">Chapter 1</a><a class="chapter" href="/chapter/2">Chapter 2</a>`)
		case "/chapter/1":
			_, _ = fmt.Fprintf(w, `<article class="content">%s</article>`, content)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func credibleFixtureContent(label string) string {
	return fmt.Sprintf("%s chapter prose begins here. This paragraph contains enough meaningful narrative text to prove that the source returned actual readable content rather than a login page, browser warning, empty extraction, or short compatibility notice.", label)
}

func candidateRequestBody(sourceURL string, alternates []map[string]string) []byte {
	body, _ := json.Marshal(map[string]any{
		"name": "Fixture Novel", "author": "Fixture Author", "sourceName": "Primary",
		"sourceUrl": sourceURL, "bookUrl": sourceURL + "/book", "alternateSources": alternates,
	})
	return body
}

func importWorkflowSource(t *testing.T, server *Server, sourceURL string) {
	t.Helper()
	rawSource, _ := json.Marshal([]map[string]any{{
		"bookSourceUrl": sourceURL, "bookSourceName": "Fixture Source", "bookSourceType": 0, "enabled": true,
		"ruleBookInfo": map[string]string{"name": ".name@text", "author": ".author@text", "intro": ".intro@html", "tocUrl": ".toc@href"},
		"ruleToc":      map[string]string{"chapterList": ".chapter", "chapterName": "text", "chapterUrl": "href"},
		"ruleContent":  map[string]string{"content": ".content@text"},
	}})
	response := performAPIRequest(server, http.MethodPost, "/api/sources", rawSource)
	if response.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCandidateOperationRejectsInvalidVariableSnapshots(t *testing.T) {
	server, closeDB := newWorkflowAPIServer(t)
	defer closeDB()
	for _, raw := range []string{`not-json`, `null`, `[]`, `{"token":null}`, `{"token":42}`} {
		for _, alternate := range []bool{false, true} {
			input := map[string]any{"name": "Fixture", "sourceUrl": "https://example.test", "bookUrl": "/book"}
			if alternate {
				input["alternateSources"] = []map[string]string{{"sourceUrl": "https://other.test", "bookUrl": "/book", "variableMap": raw}}
			} else {
				input["variableMap"] = raw
			}
			body, _ := json.Marshal(input)
			response := performAPIRequest(server, http.MethodPost, "/api/candidate-resolutions", body)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "variableMap must encode") {
				t.Fatalf("alternate=%v invalid snapshot %q: status=%d body=%s", alternate, raw, response.Code, response.Body.String())
			}
		}
	}
}
