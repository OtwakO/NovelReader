package readerstore

import (
	"errors"
	"testing"
)

func TestLockRootExcludesAnotherProcessStyleOwnerUntilClose(t *testing.T) {
	root := t.TempDir()
	if _, err := PrepareRoot(root); err != nil {
		t.Fatal(err)
	}
	first, err := LockRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := LockRoot(root); !errors.Is(err, ErrRootInUse) {
		if second != nil {
			second.Close()
		}
		first.Close()
		t.Fatalf("second lock error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := LockRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}
