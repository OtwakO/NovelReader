package api

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/otwako/novelreader/internal/chapterresource"
)

func TestChapterResourcesOutliveRuntimeAndStayOutsidePortableHome(t *testing.T) {
	server, _, readers, id, cleanup := newOwnershipServer(t)
	defer cleanup()
	ctx := t.Context()
	home, err := readers.Open(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	owner := chapterresource.Owner{ReaderID: string(id), Generation: home.Generation(), BookID: "book", SourceID: "source", SourceIdentity: "definition"}
	home.Close()
	store := server.services.chapterResources
	if store == nil {
		t.Fatal("shared chapter resource store not started")
	}
	ref, err := store.Admit(ctx, owner, chapterresource.Images{URLs: []string{"https://example.invalid/private-image-recipe.png"}}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Quiescing/closing a runtime during restore must not close shared storage or
	// discard old resources: a failed restore may resume the original home.
	if err := server.quiesceReader(ctx, id); err != nil {
		t.Fatal(err)
	}
	server.resumeReader(id)
	if _, err := store.Resolve(ctx, owner, ref.ID); err != nil {
		t.Fatalf("runtime retirement lost promise: %v", err)
	}
	snapshot := filepath.Join(t.TempDir(), "portable")
	if err := readers.SnapshotHome(ctx, id, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(snapshot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("private-image-recipe")) || entry.Name() == "chapter-resources.sqlite" {
			t.Errorf("portable snapshot contains resource cache: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := server.quiesceReader(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := readers.Remove(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := server.forgetReader(id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, owner, ref.ID); !errors.Is(err, chapterresource.ErrUnavailable) {
		t.Fatalf("deleted reader retained recipes: %v", err)
	}
}
