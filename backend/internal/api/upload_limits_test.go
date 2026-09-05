package api

import (
	"bytes"
	"github.com/otwako/novelreader/internal/fontstore"
	"github.com/otwako/novelreader/internal/readerstore"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
)

type uploadPadding struct{}

func (uploadPadding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestSourceReplacementRejectsOversizedBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/api/sources?id=source", io.LimitReader(uploadPadding{}, booksource.MaxCollectionDocumentBytes+1))
	response := httptest.NewRecorder()
	(&Server{}).handleUpdateSource(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestFontUploadRejectsOversizedFileAndRequest(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)
	for _, tc := range []struct {
		name string
		size int64
	}{{"file limit", 20<<20 + 1}, {"request limit", 21<<20 + 1}} {
		t.Run(tc.name, func(t *testing.T) {
			var envelope bytes.Buffer
			writer := multipart.NewWriter(&envelope)
			if _, err := writer.CreateFormFile("file", "fixture.woff2"); err != nil {
				t.Fatal(err)
			}
			split := envelope.Len()
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			body := io.MultiReader(bytes.NewReader(envelope.Bytes()[:split]), io.LimitReader(uploadPadding{}, tc.size), bytes.NewReader(envelope.Bytes()[split:]))
			request := httptest.NewRequest(http.MethodPost, "/api/fonts", body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			(&Server{}).handleUploadFont(response, request)
			entries, err := os.ReadDir(tempDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary files retained: %v, %v", entries, err)
			}
			if response.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestFontUploadPersistsWithinLimit(t *testing.T) {
	readers, err := readerstore.NewManager(t.TempDir(), 1, fontstore.ReaderSchema())
	if err != nil {
		t.Fatal(err)
	}
	defer readers.Close()
	if err := readers.Create(t.Context(), runtimeTestUser); err != nil {
		t.Fatal(err)
	}
	home, err := readers.Open(t.Context(), runtimeTestUser)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()
	server := &Server{fontStore: fontstore.NewStore(home.DB(), home.Files())}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "fixture.woff2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "synthetic font"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/fonts", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	server.handleUploadFont(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	fonts, err := server.fontStore.List()
	if err != nil || len(fonts) != 1 {
		t.Fatalf("fonts=%v error=%v", fonts, err)
	}
	_, data, err := server.fontStore.Read(fonts[0].ID)
	if err != nil || string(data) != "synthetic font" {
		t.Fatalf("data=%q error=%v", data, err)
	}
}
