package readerstore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestWriteWorkFileBoundaries(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	n, err := WriteWorkFile(t.Context(), root, "work/exact", bytes.NewBufferString("1234"), 4)
	if err != nil || n != 4 {
		t.Fatalf("exact: %d %v", n, err)
	}
	if _, err = WriteWorkFile(t.Context(), root, "work/exact", bytes.NewBufferString("other"), 8); !errors.Is(err, os.ErrExist) {
		t.Fatal("overwrote existing file", err)
	}
	if data, err := root.ReadFile("work/exact"); err != nil || string(data) != "1234" {
		t.Fatal(err)
	}
	n, err = WriteWorkFile(t.Context(), root, "work/large", bytes.NewBufferString("123456789"), 4)
	if !errors.Is(err, ErrFileTooLarge) || n != 5 {
		t.Fatalf("limit: %d %v", n, err)
	}
	if info, err := root.Stat("work/large"); err != nil || info.Size() != 5 {
		t.Fatal("unbounded output", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = WriteWorkFile(ctx, root, "work/cancelled", bytes.NewReader(nil), 4); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = root.Stat("work/cancelled"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
