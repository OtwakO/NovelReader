package readerstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestPrepareAnchoredRootRejectsOrdinaryDirectorySwapBeforeHandleOpen(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	if state, err := PrepareRoot(root); err != nil || state != RootCurrent {
		t.Fatalf("prepare root: state=%q error=%v", state, err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement")
	if state, err := PrepareRoot(replacement); err != nil || state != RootCurrent {
		t.Fatalf("prepare replacement: state=%q error=%v", state, err)
	}
	replacementHome := filepath.Join(replacement, UsersDirectory, string(testUserAlice))
	if err := os.Mkdir(replacementHome, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(replacementHome, "keep")
	if err := os.WriteFile(marker, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	movedRoot := root + ".moved"
	openAfterSwap := func(name string) (*os.Root, error) {
		if err := os.Rename(root, movedRoot); err != nil {
			return nil, err
		}
		if err := os.Rename(replacement, root); err != nil {
			return nil, err
		}
		return os.OpenRoot(name)
	}

	rootHandle, _, err := prepareAnchoredRoot(root, openAfterSwap)
	if rootHandle != nil {
		_ = rootHandle.Close()
		t.Fatal("anchored preparation accepted swapped data root")
	}
	if !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("prepare anchored root error=%v", err)
	}
	if value, err := os.ReadFile(filepath.Join(root, UsersDirectory, string(testUserAlice), "keep")); err != nil || string(value) != "safe" {
		t.Fatalf("replacement target changed: %q error=%v", value, err)
	}
	if _, err := os.Stat(filepath.Join(movedRoot, RootManifestName)); err != nil {
		t.Fatalf("genuine root changed: %v", err)
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

func TestManagerInitializesCurrentReaderSchema(t *testing.T) {
	first := ReaderSchema{Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE schema_sources (id TEXT PRIMARY KEY)`)
		return err
	}}
	second := ReaderSchema{Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE schema_books (id TEXT PRIMARY KEY)`)
		return err
	}}
	manager, err := NewManager(t.TempDir(), 1, first, second)
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
	var version int
	if err := home.DB().QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != CurrentReaderSchemaVersion {
		t.Fatalf("reader database version = %d", version)
	}
	for _, table := range []string{"schema_sources", "schema_books"} {
		var count int
		if err := home.DB().QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s count=%d error=%v", table, count, err)
		}
	}
	var historyTables int
	if err := home.DB().QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='readerstore_migrations'`).Scan(&historyTables); err != nil || historyTables != 0 {
		t.Fatalf("schema history tables=%d error=%v", historyTables, err)
	}
}

func TestInitializeReaderDatabaseRollsBackFailedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), ReaderDatabaseName)
	failure := errors.New("schema failed")
	err := initializeReaderDatabase(path, []ReaderSchema{
		{Initialize: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE partial_schema (id TEXT PRIMARY KEY)`)
			return err
		}},
		{Initialize: func(*sql.Tx) error { return failure }},
	})
	if !errors.Is(err, failure) {
		t.Fatalf("initialization error=%v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var tableCount, version int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='partial_schema'`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 || version != 0 {
		t.Fatalf("partial schema committed: tables=%d version=%d", tableCount, version)
	}
}

func TestManagerRejectsMissingReaderSchemaInitializer(t *testing.T) {
	if _, err := NewManager(t.TempDir(), 1, ReaderSchema{}); err == nil {
		t.Fatal("expected missing schema initializer error")
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

	for databaseName, expectedVersion := range map[string]int{
		ReaderDatabaseName: CurrentReaderSchemaVersion, CredentialsDatabaseName: CurrentCredentialsSchemaVersion,
	} {
		db, err := sql.Open("sqlite", filepath.Join(homePath, databaseName))
		if err != nil {
			t.Fatal(err)
		}
		var version int
		err = db.QueryRow(`PRAGMA user_version`).Scan(&version)
		closeErr := db.Close()
		if err != nil || closeErr != nil || version != expectedVersion {
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
				if err := createStagedHome(stagingPath, testUserAlice, manager.schemas); err != nil {
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
			if err := validateHome(homePath, testUserAlice, manager.schemas); err != nil {
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
	if err := createStagedHome(stagingPath, testUserAlice, manager.schemas); err != nil {
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
	if err := validateHome(homePath, testUserAlice, manager.schemas); err != nil {
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

func TestManagerRejectsIncompleteCurrentReaderSchemaWithoutMutation(t *testing.T) {
	schema := ReaderSchema{Initialize: func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE required_table (id TEXT PRIMARY KEY, value TEXT)`)
		return err
	}}
	manager, err := NewManager(t.TempDir(), 2, schema)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ctx := context.Background()
	if err := manager.Create(ctx, testUserAlice); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice), ReaderDatabaseName)
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE required_table`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	for operation, err := range map[string]error{
		"create": manager.Create(ctx, testUserAlice),
		"open":   func() error { _, err := manager.Open(ctx, testUserAlice); return err }(),
	} {
		if !errors.Is(err, ErrReaderSchemaMismatch) || !strings.Contains(err.Error(), "declared object") {
			t.Fatalf("%s error=%v", operation, err)
		}
	}
	db, err = sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, tableCount int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='required_table'`).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if version != CurrentReaderSchemaVersion || tableCount != 0 {
		t.Fatalf("home repaired or restamped: version=%d tables=%d", version, tableCount)
	}
}

func TestManagerRejectsMismatchedReaderSchemaWithoutMutation(t *testing.T) {
	for _, version := range []int{CurrentReaderSchemaVersion - 1, CurrentReaderSchemaVersion + 1} {
		t.Run(fmt.Sprintf("version-%d", version), func(t *testing.T) {
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
			if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}

			for operation, err := range map[string]error{
				"create": manager.Create(ctx, testUserAlice),
				"open":   func() error { _, err := manager.Open(ctx, testUserAlice); return err }(),
			} {
				if !errors.Is(err, ErrReaderSchemaMismatch) {
					t.Fatalf("%s error = %v", operation, err)
				}
				for _, guidance := range []string{"remove or rename", "DATA_DIR", "re-import test BookSources"} {
					if !strings.Contains(err.Error(), guidance) {
						t.Errorf("%s error %q missing %q", operation, err, guidance)
					}
				}
			}
			db, err = sql.Open("sqlite", databasePath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var storedVersion, historyTables int
			if err := db.QueryRow(`PRAGMA user_version`).Scan(&storedVersion); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='readerstore_migrations'`).Scan(&historyTables); err != nil {
				t.Fatal(err)
			}
			if storedVersion != version || historyTables != 0 {
				t.Fatalf("home mutated: version=%d historyTables=%d", storedVersion, historyTables)
			}
		})
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

func TestManagerRemoveDrainsOpenHomesRejectsNewOpensAndDeletesContainedPath(t *testing.T) {
	manager := newTestManager(t, 2)
	if err := manager.Create(context.Background(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	home, err := manager.Open(context.Background(), testUserAlice)
	if err != nil {
		t.Fatal(err)
	}
	removeDone := make(chan error, 1)
	go func() { removeDone <- manager.Remove(context.Background(), testUserAlice) }()
	deadline := time.Now().Add(time.Second)
	for {
		probe, err := manager.Open(context.Background(), testUserAlice)
		if errors.Is(err, ErrHomeDeleting) {
			break
		}
		if err != nil {
			t.Fatalf("probe open: %v", err)
		}
		if err := probe.Close(); err != nil {
			t.Fatalf("probe close: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("remove did not block new opens")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case err := <-removeDone:
		t.Fatalf("remove completed before open home drained: %v", err)
	default:
	}
	if err := home.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-removeDone; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(manager.root, UsersDirectory, string(testUserAlice))
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reader home remains: %v", err)
	}
	if _, err := manager.Open(context.Background(), testUserAlice); !errors.Is(err, ErrHomeDeleting) {
		t.Fatalf("post-delete open error=%v", err)
	}
	if err := manager.Remove(context.Background(), testUserAlice); err != nil {
		t.Fatalf("idempotent remove: %v", err)
	}
}

func TestManagerRemoveRetriesAfterFilesystemFailure(t *testing.T) {
	manager := newTestManager(t, 2)
	if err := manager.Create(context.Background(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	realRemoveHome := manager.removeHome
	failed := true
	manager.removeHome = func(root *os.Root, name string) error {
		if failed {
			return errors.New("simulated filesystem failure")
		}
		return realRemoveHome(root, name)
	}
	if err := manager.Remove(context.Background(), testUserAlice); err == nil {
		t.Fatal("remove unexpectedly succeeded")
	}
	path := filepath.Join(manager.root, UsersDirectory, string(testUserAlice))
	quarantinePath := path + ".deleting"
	if _, err := os.Stat(quarantinePath); err != nil {
		t.Fatalf("quarantined home missing after failed remove: %v", err)
	}
	if err := manager.Create(context.Background(), testUserAlice); !errors.Is(err, ErrHomeDeleting) {
		t.Fatalf("create while deleting error=%v", err)
	}
	failed = false
	if err := manager.Remove(context.Background(), testUserAlice); err != nil {
		t.Fatalf("retry remove: %v", err)
	}
	if _, err := os.Lstat(quarantinePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("quarantined home remains after retry: %v", err)
	}
}

func TestManagerRejectsPostConstructionDataRootSwap(t *testing.T) {
	manager := newTestManager(t, 2)
	root := manager.root
	movedRoot := root + ".moved"
	replacement := t.TempDir()
	if state, err := PrepareRoot(replacement); err != nil || state != RootCurrent {
		t.Fatalf("prepare replacement: state=%q error=%v", state, err)
	}
	if err := os.Rename(root, movedRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, root); err != nil {
		t.Fatal(err)
	}
	if err := manager.Create(context.Background(), testUserAlice); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("create after root swap error=%v", err)
	}
	if _, err := manager.Open(context.Background(), testUserAlice); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("open after root swap error=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, UsersDirectory, string(testUserAlice))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("replacement root was modified: %v", err)
	}
}

func TestManagerRemoveQuarantineDoesNotDeleteLaterCanonicalReplacement(t *testing.T) {
	manager := newTestManager(t, 2)
	if err := manager.Create(context.Background(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	usersPath := filepath.Join(manager.root, UsersDirectory)
	homePath := filepath.Join(usersPath, string(testUserAlice))
	marker := filepath.Join(homePath, "replacement-marker")
	realRenameHome := manager.renameHome
	manager.renameHome = func(root *os.Root, oldName, newName string) error {
		if err := realRenameHome(root, oldName, newName); err != nil {
			return err
		}
		if err := os.Mkdir(homePath, 0o700); err != nil {
			return err
		}
		return os.WriteFile(marker, []byte("safe"), 0o600)
	}
	if err := manager.Remove(context.Background(), testUserAlice); !errors.Is(err, ErrInvalidHome) {
		t.Fatalf("remove with canonical replacement error=%v", err)
	}
	if value, err := os.ReadFile(marker); err != nil || string(value) != "safe" {
		t.Fatalf("later canonical replacement changed: %q error=%v", value, err)
	}
}

func TestManagerRemoveRejectsParentSymlinkBeforeUsersHandleOpens(t *testing.T) {
	manager := newTestManager(t, 2)
	if err := manager.Create(context.Background(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideHome := filepath.Join(outside, string(testUserAlice))
	if err := os.Mkdir(outsideHome, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(outsideHome, "keep")
	if err := os.WriteFile(marker, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	usersPath := filepath.Join(manager.root, UsersDirectory)
	movedUsersPath := usersPath + ".moved"
	if err := os.Rename(usersPath, movedUsersPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, usersPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := manager.Remove(context.Background(), testUserAlice); err == nil {
		t.Fatal("remove followed outside users symlink")
	}
	if value, err := os.ReadFile(marker); err != nil || string(value) != "safe" {
		t.Fatalf("outside target changed: %q error=%v", value, err)
	}
	if _, err := os.Stat(filepath.Join(movedUsersPath, string(testUserAlice))); err != nil {
		t.Fatalf("genuine home changed: %v", err)
	}
}

func TestManagerRemoveAnchorsDeletionAgainstParentSymlinkSwap(t *testing.T) {
	manager := newTestManager(t, 2)
	if err := manager.Create(context.Background(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideHome := filepath.Join(outside, string(testUserAlice))
	if err := os.Mkdir(outsideHome, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(outsideHome, "keep")
	if err := os.WriteFile(marker, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	usersPath := filepath.Join(manager.root, UsersDirectory)
	movedUsersPath := usersPath + ".moved"
	realRemoveHome := manager.removeHome
	manager.removeHome = func(root *os.Root, name string) error {
		if err := os.Rename(usersPath, movedUsersPath); err != nil {
			return err
		}
		if err := os.Symlink(outside, usersPath); err != nil {
			return err
		}
		return realRemoveHome(root, name)
	}
	if err := manager.Remove(context.Background(), testUserAlice); err != nil {
		t.Fatal(err)
	}
	if value, err := os.ReadFile(marker); err != nil || string(value) != "safe" {
		t.Fatalf("outside target changed: %q error=%v", value, err)
	}
	if _, err := os.Lstat(filepath.Join(movedUsersPath, string(testUserAlice))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("anchored home remains: %v", err)
	}
}

func TestManagerRemoveRejectsSymlinkWithoutTouchingTarget(t *testing.T) {
	manager := newTestManager(t, 2)
	outside := t.TempDir()
	marker := filepath.Join(outside, "keep")
	if err := os.WriteFile(marker, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	homePath := filepath.Join(manager.root, UsersDirectory, string(testUserAlice))
	if err := os.Symlink(outside, homePath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := manager.Remove(context.Background(), testUserAlice); !errors.Is(err, ErrInvalidHome) {
		t.Fatalf("remove error=%v", err)
	}
	if value, err := os.ReadFile(marker); err != nil || string(value) != "safe" {
		t.Fatalf("outside target changed: %q error=%v", value, err)
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
