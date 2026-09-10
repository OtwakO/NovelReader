package fontstore

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/otwako/novelreader/internal/readerstore"
)

func TestFontMutationsRespectSnapshotBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		manager, err := readerstore.NewManager(t.TempDir(), 1, ReaderSchema())
		if err != nil {
			t.Fatal(err)
		}
		defer manager.Close()
		if err := manager.Create(t.Context(), fontAlice); err != nil {
			t.Fatal(err)
		}
		home, err := manager.Open(t.Context(), fontAlice)
		if err != nil {
			t.Fatal(err)
		}
		defer home.Close()
		store := NewStore(home.DB(), home.Files())
		if _, err := store.Add(t.Context(), "Fixture", "font", []byte("original")); err != nil {
			t.Fatal(err)
		}

		for _, operation := range []func(context.Context) error{
			func(ctx context.Context) error {
				_, err := store.Add(ctx, "Fixture", "new", []byte("replacement"))
				return err
			},
			func(ctx context.Context) error { return store.Delete(ctx, "font") },
			store.Cleanup,
		} {
			unlock, err := home.Files().LockMutation(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			go func() { done <- operation(ctx) }()
			synctest.Wait()
			select {
			case err := <-done:
				t.Fatalf("font mutation bypassed snapshot gate: %v", err)
			default:
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled mutation: %v", err)
			}
			unlock()
		}
		if _, data, err := store.Read("font"); err != nil || string(data) != "original" {
			t.Fatalf("cancelled mutation changed font: %q, %v", data, err)
		}
		// Cancellation must not leave the gate occupied.
		if _, err := store.Add(t.Context(), "Fixture", "new", []byte("replacement")); err != nil {
			t.Fatal(err)
		}
	})
}
