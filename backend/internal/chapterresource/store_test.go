package chapterresource

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBundleIdentityPromisesAndRestart(t *testing.T) {
	ctx := context.Background()
	var clock atomic.Int64
	clock.Store(time.Now().UnixNano())
	now := func() time.Time { return time.Unix(0, clock.Load()) }
	path := filepath.Join(t.TempDir(), "cache", "resources.sqlite")
	limits := DefaultLimits()
	limits.TotalBundles = 1
	s, err := open(ctx, path, limits, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	owner := Owner{ReaderID: "reader", Generation: "home", BookID: "book", SourceID: "source", SourceIdentity: "definition", Revision: 1}
	images := Images{Book: map[string]any{"bookUrl": "https://example.invalid/book"}, URLs: []string{"https://example.invalid/one.png"}}
	deadline := now().Add(24 * time.Hour)
	first, err := s.Admit(ctx, owner, images, deadline)
	if err != nil {
		t.Fatal(err)
	}
	reused, err := s.Admit(ctx, owner, images, deadline.Add(-time.Hour))
	if err != nil || reused != first {
		t.Fatalf("reuse shortened promise: %+v %v", reused, err)
	}
	images.URLs[0] = "https://example.invalid/two.png"
	if _, err := s.Admit(ctx, owner, images, deadline); !errors.Is(err, ErrCapacity) {
		t.Fatalf("admission: %v", err)
	}
	for _, field := range []string{"reader", "home", "book", "revision", "definition"} {
		changed := owner
		switch field {
		case "reader":
			changed.ReaderID = "other"
		case "home":
			changed.Generation = "other"
		case "book":
			changed.BookID = "other"
		case "revision":
			changed.Revision++
		case "definition":
			changed.SourceIdentity = "other"
		}
		if _, err := s.Resolve(ctx, changed, first.ID); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%s scope: %v", field, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = open(ctx, path, limits, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Resolve(ctx, owner, first.ID)
	if err != nil || got.URLs[0] != "https://example.invalid/one.png" {
		t.Fatalf("restart/immutability: %+v %v", got, err)
	}
	// An already resolved request owns its recipe even after cleanup.
	clock.Store(deadline.UnixNano())
	if err := s.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, owner, first.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expired: %v", err)
	}
	if got.URLs[0] != "https://example.invalid/one.png" {
		t.Fatal("cleanup altered request-owned recipe")
	}
	second, err := s.Admit(ctx, owner, images, now().Add(time.Hour))
	if err != nil || second.ID == first.ID {
		t.Fatalf("reclaim/change: %+v %v", second, err)
	}
	if err := s.DeleteReader(ctx, owner.ReaderID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(ctx, owner, second.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("deleted reader: %v", err)
	}
}

func TestAdmissionBudgetsAreAtomic(t *testing.T) {
	for _, budget := range []string{"reader-count", "total-count", "reader-bytes", "total-bytes"} {
		t.Run(budget, func(t *testing.T) {
			ctx := context.Background()
			limits := DefaultLimits()
			switch budget {
			case "reader-count":
				limits.ReaderBundles = 1
			case "total-count":
				limits.TotalBundles = 1
			// Each fixture is >700 bytes; two cannot fit in 1400 bytes.
			case "reader-bytes":
				limits.ReaderBytes = 1400
			case "total-bytes":
				limits.TotalBytes = 1400
			}
			s, err := Open(ctx, filepath.Join(t.TempDir(), "resources.sqlite"), limits)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var success atomic.Int64
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					owner := Owner{ReaderID: "reader", Generation: "home", BookID: "book", SourceID: "source", SourceIdentity: "definition", ChapterIndex: i}
					if budget == "total-count" || budget == "total-bytes" {
						owner.ReaderID += string(rune('a' + i))
					}
					_, err := s.Admit(ctx, owner, Images{URLs: []string{"https://example.invalid/" + strings.Repeat("a", 600) + ".png"}}, time.Now().Add(time.Hour))
					if err == nil {
						success.Add(1)
					} else if !errors.Is(err, ErrCapacity) {
						t.Errorf("admit: %v", err)
					}
				}(i)
			}
			wg.Wait()
			if success.Load() != 1 {
				t.Fatalf("admitted %d bundles", success.Load())
			}
		})
	}
}
