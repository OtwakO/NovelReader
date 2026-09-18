package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
)

func TestArchiveBoundary(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"../outside", "/absolute", `dir\file`, "dir/../file", "dir/./file", "file\x00"} {
		t.Run(name, func(t *testing.T) {
			data := fixtureArchive(t, []fixtureEntry{{name, "x"}})
			if _, err := openArchive(ctx, bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrArchive) {
				t.Fatalf("got %v", err)
			}
		})
	}
	data := fixtureArchive(t, []fixtureEntry{{"duplicate", "one"}, {"duplicate", "two"}})
	if _, err := openArchive(ctx, bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrArchive) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := openArchive(ctx, bytes.NewReader(nil), MaxInputBytes+1); !errors.Is(err, ErrLimit) {
		t.Fatalf("input size: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := openArchive(canceled, bytes.NewReader(nil), 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
}

func TestArchiveReadBudgetsAndCancellation(t *testing.T) {
	ctx := context.Background()
	data := fixtureArchive(t, []fixtureEntry{{"metadata", string(bytes.Repeat([]byte("a"), int(maxMetadataBytes)+1))}})
	a, err := openArchive(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.readMetadata(ctx, "metadata"); !errors.Is(err, ErrLimit) {
		t.Fatalf("declared size: %v", err)
	}
	// Deliberately lie about the declared length to exercise the actual-byte
	// guard independently of ZIP metadata. No allocation uses declared size.
	a.files["metadata"].UncompressedSize64 = 0
	if _, err = a.readMetadata(ctx, "metadata"); !errors.Is(err, ErrLimit) && !errors.Is(err, zip.ErrFormat) {
		t.Fatalf("false size: %v", err)
	}
	data = fixtureArchive(t, []fixtureEntry{{"metadata", "data"}})
	a, err = openArchive(ctx, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	a.readBytes = maxMetadataReadBytes - 2
	if _, err = a.readMetadata(ctx, "metadata"); !errors.Is(err, ErrLimit) {
		t.Fatalf("total read budget: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = a.readMetadata(canceled, "metadata"); !errors.Is(err, context.Canceled) {
		t.Fatalf("read cancellation: %v", err)
	}
	// Cancellation between chunks is observed before touching the source again.
	r := contextReader{ctx: canceled, source: bytes.NewReader([]byte("data"))}
	if _, err = r.Read(make([]byte, 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("stream cancellation: %v", err)
	}
}

func TestArchiveDeclaredExpansionBudget(t *testing.T) {
	data := fixtureArchive(t, []fixtureEntry{{"entry", "data"}})
	central := bytes.Index(data, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("missing fixture central directory")
	}
	binary.LittleEndian.PutUint32(data[central+24:], uint32(maxExpandedBytes+1))
	if _, err := openArchive(context.Background(), bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrLimit) {
		t.Fatalf("expanded size: %v", err)
	}
}

func TestResolveReference(t *testing.T) {
	for _, tc := range []struct{ href, path, fragment string }{
		{"../text/part%201.xhtml#note%202", "Book/text/part 1.xhtml", "note 2"},
		{"#here", "Book/nav/toc.xhtml", "here"},
		{"../../notes.xhtml", "notes.xhtml", ""},
		{"percent%2520.xhtml", "Book/nav/percent%20.xhtml", ""},
	} {
		ref, err := resolveReference("Book/nav/toc.xhtml", tc.href)
		if err != nil || ref.Path != tc.path || ref.Fragment != tc.fragment {
			t.Fatalf("%q: %+v, %v", tc.href, ref, err)
		}
	}
	for _, href := range []string{"../../../escape", "%2fabsolute", "%2e%2e/%2e%2e/%2e%2e/escape", "//example.invalid/x", "https://example.invalid/x", "file:///x", "a?query", `a%5cb`, "a%00b", "%zz"} {
		if _, err := resolveReference("Book/nav/toc.xhtml", href); !errors.Is(err, ErrReference) {
			t.Fatalf("%q: %v", href, err)
		}
	}
}
