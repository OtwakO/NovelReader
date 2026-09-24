package backup

import (
	"errors"
	"io"
)

const (
	maximumCompressedBytes = 2 << 30
	maximumExpandedBytes   = 8 << 30
	maximumArchiveEntries  = 100_000
	maximumManifestBytes   = 64 << 10
)

var errArchiveLimit = errors.New("backup: archive limits exceeded")

// The same format limits apply in both directions. Internal parameters allow
// boundary tests without multi-gigabyte fixtures; these are not runtime settings.
type archiveLimits struct {
	compressed, expanded int64
	entries              int
}

func portableArchiveLimits() archiveLimits {
	return archiveLimits{maximumCompressedBytes, maximumExpandedBytes, maximumArchiveEntries}
}

type archiveBudget struct {
	limits   archiveLimits
	expanded int64
	entries  int
}

// Count logical tar entries and their payloads, as tar.Reader exposes them.
// Padding and PAX headers are not payload; all encoded bytes count at gzip output.
func (b *archiveBudget) add(size int64) error {
	if b.entries >= b.limits.entries || size < 0 || size > b.limits.expanded-b.expanded {
		return errArchiveLimit
	}
	b.entries++
	b.expanded += size
	return nil
}

type archiveLimitWriter struct {
	output    io.Writer
	remaining int64
}

func (w *archiveLimitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, errArchiveLimit
	}
	n, err := w.output.Write(p)
	w.remaining -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
