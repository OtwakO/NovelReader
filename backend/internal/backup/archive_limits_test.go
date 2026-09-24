package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchiveLimitsMatchExportAndRestore(t *testing.T) {
	home := t.TempDir()
	// Long names exercise automatic PAX metadata, which tar.Reader does not count
	// as a separate logical entry. Empty directories still count.
	relative := filepath.Join("files", strings.Repeat("chapter-", 20), "sections.jsonl")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(home, relative)), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("synthetic prepared prose 中文\n")
	if err := os.WriteFile(filepath.Join(home, relative), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(home, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	manifest := NewManifest("synthetic", created)
	var baseline bytes.Buffer
	if err := writeArchive(t.Context(), &baseline, home, manifest, created); err != nil {
		t.Fatal(err)
	}
	exact := archiveLimits{compressed: int64(baseline.Len())}
	gz, err := gzip.NewReader(bytes.NewReader(baseline.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		exact.entries++
		exact.expanded += header.Size
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"exact", "entries", "expanded", "compressed"} {
		t.Run(kind, func(t *testing.T) {
			limits := exact
			switch kind {
			case "entries":
				limits.entries--
			case "expanded":
				limits.expanded--
			case "compressed":
				limits.compressed--
			}
			var output bytes.Buffer
			err := writeArchiveWithLimits(t.Context(), &output, home, manifest, created, limits)
			if kind != "exact" {
				if !errors.Is(err, errArchiveLimit) {
					t.Fatalf("export error=%v", err)
				}
				if int64(output.Len()) > limits.compressed {
					t.Fatal("wrote beyond compressed limit")
				}
				if _, _, err := extractArchiveWithLimits(t.Context(), bytes.NewReader(baseline.Bytes()), t.TempDir(), limits); err == nil {
					t.Fatal("restore accepted over-limit archive")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			restored, payload, err := extractArchiveWithLimits(t.Context(), bytes.NewReader(output.Bytes()), t.TempDir(), limits)
			if err != nil || restored != manifest {
				t.Fatalf("restore: %+v %v", restored, err)
			}
			got, err := os.ReadFile(filepath.Join(payload, relative))
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("payload=%q error=%v", got, err)
			}
			info, err := os.Stat(filepath.Join(payload, "empty"))
			if err != nil || !info.IsDir() {
				t.Fatal("empty directory lost", err)
			}
		})
	}
}

func TestArchiveBudgetRejectsOverflowingPayload(t *testing.T) {
	budget := archiveBudget{limits: portableArchiveLimits()}
	if err := budget.add(1); err != nil {
		t.Fatal(err)
	}
	if err := budget.add(math.MaxInt64); !errors.Is(err, errArchiveLimit) {
		t.Fatal("overflowing size accepted", err)
	}
	if budget.entries != 1 || budget.expanded != 1 {
		t.Fatal("rejected entry changed budget")
	}
}

type failingArchiveOutput struct{ err error }

func (w failingArchiveOutput) Write([]byte) (int, error) { return 0, w.err }

func TestArchiveExportPreservesOutputFailure(t *testing.T) {
	failure := errors.New("synthetic output failure")
	created := time.Now()
	if err := writeArchive(t.Context(), failingArchiveOutput{failure}, t.TempDir(), NewManifest("synthetic", created), created); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	// A nil-error short write must not result in a supposedly complete download.
	writer := archiveLimitWriter{output: failingArchiveOutput{}, remaining: 100}
	if _, err := writer.Write([]byte("payload")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

func TestArchiveExportRejectsOversizedManifest(t *testing.T) {
	created := time.Now()
	manifest := NewManifest(strings.Repeat("a", maximumManifestBytes), created)
	if err := writeArchive(t.Context(), io.Discard, t.TempDir(), manifest, created); err == nil || !strings.Contains(err.Error(), "manifest is too large") {
		t.Fatal(err)
	}
}
