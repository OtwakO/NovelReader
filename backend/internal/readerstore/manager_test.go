package readerstore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const (
	testUserAlice UserID = "11111111-1111-4111-8111-111111111111"
	testUserBob   UserID = "22222222-2222-4222-8222-222222222222"
	testUserCarol UserID = "33333333-3333-4333-8333-333333333333"
)

func TestParseUserIDRequiresCanonicalUUIDV4(t *testing.T) {
	valid, err := ParseUserID(string(testUserAlice))
	if err != nil || valid != testUserAlice {
		t.Fatalf("valid ID: id=%q err=%v", valid, err)
	}

	invalid := []string{
		"",
		"../alice",
		"11111111111141118111111111111111",
		"11111111-1111-3111-8111-111111111111",
		"11111111-1111-4111-C111-111111111111",
		"11111111-1111-4111-8111-11111111111A",
	}
	for _, raw := range invalid {
		if _, err := ParseUserID(raw); !errors.Is(err, ErrInvalidUserID) {
			t.Errorf("ParseUserID(%q) error = %v", raw, err)
		}
	}
}

func TestManagerSupportsHostileButValidDataRootNames(t *testing.T) {
	for _, name := range []string{"data?query", "data#fragment", "data with spaces", "数据"} {
		t.Run(name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), name)
			manager, err := NewManager(root, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			if err := manager.Create(context.Background(), testUserAlice); err != nil {
				t.Fatal(err)
			}
			home, err := manager.Open(context.Background(), testUserAlice)
			if err != nil {
				t.Fatal(err)
			}
			defer home.Close()

			var journalMode string
			var busyTimeout, cacheSize int
			if err := home.DB().QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
				t.Fatal(err)
			}
			if err := home.DB().QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
				t.Fatal(err)
			}
			if err := home.DB().QueryRow(`PRAGMA cache_size`).Scan(&cacheSize); err != nil {
				t.Fatal(err)
			}
			if journalMode != "wal" || busyTimeout != 5000 || cacheSize != -8000 {
				t.Fatalf("pragmas = %q, %d, %d", journalMode, busyTimeout, cacheSize)
			}
			databasePath := filepath.Join(root, UsersDirectory, string(testUserAlice), ReaderDatabaseName)
			if _, err := os.Stat(databasePath); err != nil {
				t.Fatalf("literal database path missing: %v", err)
			}
		})
	}
}

func TestManagerCreateIsIdempotentAndProducesPortableHome(t *testing.T) {
	manager := newTestManager(t, 2)
	ctx := context.Background()

	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatalf("second create: %v", err)
	}

	homePath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice))
	manifest, err := readHomeManifest(filepath.Join(homePath, HomeManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Format != HomeFormat || manifest.Version != CurrentHomeVersion || manifest.UserID != testUserAlice {
		t.Fatalf("manifest = %#v", manifest)
	}
	for _, relative := range []string{
		ReaderDatabaseName,
		CredentialsDatabaseName,
		filepath.Join(FilesDirectory, FontsDirectory),
		filepath.Join(FilesDirectory, CoversDirectory),
		filepath.Join(FilesDirectory, ChapterAssetsDirectory),
	} {
		if _, err := os.Stat(filepath.Join(homePath, relative)); err != nil {
			t.Errorf("missing %s: %v", relative, err)
		}
	}

	for _, databaseName := range []string{ReaderDatabaseName, CredentialsDatabaseName} {
		db, err := sql.Open("sqlite", filepath.Join(homePath, databaseName))
		if err != nil {
			t.Fatal(err)
		}
		var version int
		err = db.QueryRow(`PRAGMA user_version`).Scan(&version)
		closeErr := db.Close()
		if err != nil || closeErr != nil || version != InitialDatabaseVersion {
			t.Errorf("%s version=%d queryErr=%v closeErr=%v", databaseName, version, err, closeErr)
		}
	}
}

func TestSQLiteFileURIEscapesPaths(t *testing.T) {
	for path, want := range map[string]string{
		"/tmp/data?query/reader.db":    "file:///tmp/data%3Fquery/reader.db",
		"/tmp/data#fragment/reader.db": "file:///tmp/data%23fragment/reader.db",
	} {
		if got := sqliteFileURI(path); got != want {
			t.Errorf("sqliteFileURI(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestManagerRecoversCompleteAndIncompleteStaging(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "incomplete", true: "complete"}[complete], func(t *testing.T) {
			manager := newTestManager(t, 2)
			homePath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice))
			stagingPath := homePath + homeStagingSuffix
			if complete {
				if err := createStagedHome(stagingPath, testUserAlice); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(stagingPath, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(stagingPath, "partial"), []byte("old"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			if err := manager.Create(context.Background(), testUserAlice); err != nil {
				t.Fatal(err)
			}
			if err := validateHome(homePath, testUserAlice); err != nil {
				t.Fatalf("published home: %v", err)
			}
			if _, err := os.Lstat(stagingPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("staging remains: %v", err)
			}
		})
	}
}

func TestManagerRebuildsStructurallyCompleteVersionZeroStaging(t *testing.T) {
	manager := newTestManager(t, 2)
	homePath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice))
	stagingPath := homePath + homeStagingSuffix
	if err := createStagedHome(stagingPath, testUserAlice); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(stagingPath, CredentialsDatabaseName)
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := manager.Create(context.Background(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	if err := validateHome(homePath, testUserAlice); err != nil {
		t.Fatalf("recovered home: %v", err)
	}
}

func TestManagerRejectsSymlinkedStagingWithoutTouchingTarget(t *testing.T) {
	manager := newTestManager(t, 2)
	outside := t.TempDir()
	marker := filepath.Join(outside, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	stagingPath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice)) + homeStagingSuffix
	if err := os.Symlink(outside, stagingPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := manager.Create(context.Background(), testUserAlice); !errors.Is(err, ErrInvalidHome) {
		t.Fatalf("Create error = %v", err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "keep" {
		t.Fatalf("outside target changed: %q err=%v", got, err)
	}
}

func TestManagerRejectsInvalidTypedIDWithoutFilesystemChanges(t *testing.T) {
	manager := newTestManager(t, 2)
	usersPath := filepath.Join(manager.root, UsersDirectory)
	before, err := os.ReadDir(usersPath)
	if err != nil {
		t.Fatal(err)
	}

	invalid := UserID("../escape")
	if err := manager.Create(context.Background(), invalid); !errors.Is(err, ErrInvalidUserID) {
		t.Fatalf("Create error = %v", err)
	}
	if _, err := manager.Open(context.Background(), invalid); !errors.Is(err, ErrInvalidUserID) {
		t.Fatalf("Open error = %v", err)
	}
	after, err := os.ReadDir(usersPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("users directory changed: before=%d after=%d", len(before), len(after))
	}
}

func TestManagerRejectsUnsupportedReaderDatabaseVersion(t *testing.T) {
	manager := newTestManager(t, 2)
	ctx := context.Background()
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice), ReaderDatabaseName)
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 2`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Open(ctx, testUserAlice); !errors.Is(err, ErrNewerDatabaseSchema) {
		t.Fatalf("Open error = %v", err)
	}
}

func TestHomeFilesKeepOperationsWithinReaderFiles(t *testing.T) {
	manager := newTestManager(t, 2)
	ctx := context.Background()
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer home.Close()

	files := home.Files()
	if err := files.WriteFile([]byte("font"), 0o600, FontsDirectory, "font-id"); err != nil {
		t.Fatal(err)
	}
	contents, err := files.ReadFile(FontsDirectory, "font-id")
	if err != nil || string(contents) != "font" {
		t.Fatalf("read contents=%q err=%v", contents, err)
	}
	if err := files.Remove(FontsDirectory, "font-id"); err != nil {
		t.Fatal(err)
	}
	for _, segments := range [][]string{{"..", ReaderDatabaseName}, {FontsDirectory, "../escape"}, {FontsDirectory, "/absolute"}} {
		if err := files.WriteFile([]byte("escape"), 0o600, segments...); !errors.Is(err, ErrInvalidFilePath) {
			t.Errorf("WriteFile(%q) error = %v", segments, err)
		}
	}

	outside := t.TempDir()
	link := filepath.Join(manager.root, UsersDirectory, string(testUserAlice), FilesDirectory, FontsDirectory, "link")
	if err := os.Symlink(outside, link); err == nil {
		if err := files.WriteFile([]byte("escape"), 0o600, FontsDirectory, "link", "outside"); err == nil {
			t.Fatal("write through escaping symlink succeeded")
		}
		if _, err := os.Stat(filepath.Join(outside, "outside")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("outside file created: %v", err)
		}
	}

	filesPath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice), FilesDirectory)
	movedFilesPath := filesPath + "-original"
	if err := os.Rename(filesPath, movedFilesPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filesPath); err == nil {
		if err := files.WriteFile([]byte("escape"), 0o600, "outside"); !errors.Is(err, ErrInvalidFilePath) {
			t.Fatalf("write through replaced files root error = %v", err)
		}
	}
}

func TestManagerKeepsTwoUserDatabasesIsolated(t *testing.T) {
	manager := newTestManager(t, 2)
	ctx := context.Background()
	for _, userID := range []UserID{testUserAlice, testUserBob} {
		if err := manager.Create(ctx, userID); err != nil {
			t.Fatal(err)
		}
	}

	alice, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer alice.Close()
	bob, err := manager.Open(ctx, testUserBob)
	if err != nil {
		t.Fatal(err)
	}
	defer bob.Close()

	if alice.ID() != testUserAlice || bob.ID() != testUserBob {
		t.Fatalf("home IDs = %q / %q", alice.ID(), bob.ID())
	}
	if _, err := alice.DB().Exec(`CREATE TABLE private_value (value TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := alice.DB().Exec(`INSERT INTO private_value(value) VALUES ('alice-only')`); err != nil {
		t.Fatal(err)
	}
	if _, err := bob.DB().Exec(`SELECT value FROM private_value`); err == nil {
		t.Fatal("Bob database can read Alice-only table")
	}
}

func TestManagerReusesIdleCachedHome(t *testing.T) {
	manager := newTestManager(t, 1)
	ctx := context.Background()
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	first, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	db := first.DB()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("idle cached database closed: %v", err)
	}
	second, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.DB() != db {
		t.Fatal("idle cached home was reopened")
	}
}

func TestManagerBoundsCachedHomesAndEvictsIdleEntry(t *testing.T) {
	manager := newTestManager(t, 1)
	ctx := context.Background()
	for _, userID := range []UserID{testUserAlice, testUserBob} {
		if err := manager.Create(ctx, userID); err != nil {
			t.Fatal(err)
		}
	}

	alice, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	waitingContext, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := manager.Open(waitingContext, testUserBob); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Open while cache active error = %v", err)
	}
	aliceDB := alice.DB()
	if err := alice.Close(); err != nil {
		t.Fatal(err)
	}

	bob, err := manager.Open(ctx, testUserBob)
	if err != nil {
		t.Fatal(err)
	}
	defer bob.Close()
	if err := aliceDB.Ping(); err == nil {
		t.Fatal("evicted Alice database remains open")
	}
}

func TestManagerWakesMultipleWaitingOpens(t *testing.T) {
	manager := newTestManager(t, 2)
	ctx := context.Background()
	users := []UserID{testUserAlice, testUserBob, testUserCarol}
	for _, userID := range users {
		if err := manager.Create(ctx, userID); err != nil {
			t.Fatal(err)
		}
	}
	alice, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := manager.Open(ctx, testUserBob)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 2)
	for _, userID := range []UserID{testUserCarol, testUserCarol} {
		go func() {
			waitingContext, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			home, err := manager.Open(waitingContext, userID)
			if err == nil {
				err = home.Close()
			}
			result <- err
		}()
	}
	time.Sleep(20 * time.Millisecond)
	if err := alice.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bob.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-result; err != nil {
			t.Fatalf("waiting open: %v", err)
		}
	}
}

func TestManagerCloseClosesHandlesAndPreventsFurtherUse(t *testing.T) {
	manager := newTestManager(t, 1)
	ctx := context.Background()
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(ctx, testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	db := home.DB()

	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err == nil {
		t.Fatal("database remains open after manager shutdown")
	}
	if _, err := manager.Open(ctx, testUserAlice); !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("Open after shutdown error = %v", err)
	}
	if err := home.Close(); err != nil {
		t.Fatalf("home close after manager shutdown: %v", err)
	}
}

func newTestManager(t *testing.T, capacity int) *Manager {
	t.Helper()
	root := filepath.Join(t.TempDir(), "data")
	manager, err := NewManager(root, capacity)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("close manager: %v", err)
		}
	})
	return manager
}
