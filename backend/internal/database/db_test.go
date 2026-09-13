package database

import (
	"path/filepath"
	"testing"
)

func TestOpenAppliesSQLiteConnectionOptions(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "reader.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Keep the first connection borrowed so the second exercises a newly
	// opened connection, not just pragmas applied to the initial one.
	for range 2 {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		var mode string
		if err := conn.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
			t.Fatalf("journal=%q err=%v", mode, err)
		}
		var timeout int
		if err := conn.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
			t.Fatalf("timeout=%d err=%v", timeout, err)
		}
	}
}
