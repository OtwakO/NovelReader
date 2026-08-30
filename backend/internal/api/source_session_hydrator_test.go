package api

import (
	"testing"

	"github.com/otwako/novelreader/internal/booksource"
	"github.com/otwako/novelreader/internal/readerstore"
	"github.com/otwako/novelreader/internal/sourceexec"
	"github.com/otwako/novelreader/internal/sourceprofile"
)

func TestSourceSessionHydratorAppliesReaderOwnedState(t *testing.T) {
	readers, err := readerstore.NewManager(t.TempDir(), 1, booksource.ReaderSchema(), sourceprofile.ReaderSchema())
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
	source := booksource.BookSource{ID: "source-a", BookSourceURL: "https://source.test", BookSourceName: "Source", Enabled: true}
	if err := booksource.NewStore(home.DB()).Upsert(&source); err != nil {
		t.Fatal(err)
	}
	profiles := sourceprofile.NewStore(home.DB(), home.CredentialsDB())
	if err := profiles.SaveSettings(t.Context(), source.ID, []byte(`{"variable":"durable","values":{"mode":"cloud"}}`)); err != nil {
		t.Fatal(err)
	}
	if err := profiles.SaveAuthentication(t.Context(), source.ID, []byte(`{"loginHeader":"{\"Authorization\":\"Bearer token\"}","cookies":{"https://source.test":"sid=secret"}}`)); err != nil {
		t.Fatal(err)
	}
	session := sourceexec.NewSourceSession()
	if err := sourceSessionHydrator(profiles)(t.Context(), source, session); err != nil {
		t.Fatal(err)
	}
	if session.GetVariable(source.BookSourceURL) != "durable" || session.GetMemory("mode") != "cloud" {
		t.Fatalf("variable=%q mode=%v", session.GetVariable(source.BookSourceURL), session.GetMemory("mode"))
	}
	if session.RequestHeaders()["Authorization"] != "Bearer token" || session.JarCookieHeader(source.BookSourceURL) != "sid=secret" {
		t.Fatalf("headers=%v cookie=%q", session.RequestHeaders(), session.JarCookieHeader(source.BookSourceURL))
	}
}
