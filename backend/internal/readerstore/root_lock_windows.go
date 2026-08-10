//go:build windows

package readerstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

const rootLockName = ".novelreader.lock"

var ErrRootInUse = errors.New("data root is already in use by another NovelReader process")

type RootLock struct {
	file *os.File
}

// LockRoot exclusively reserves a prepared data root until the returned lock is closed.
func LockRoot(root string) (*RootLock, error) {
	path := filepath.Join(root, rootLockName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("readerstore: open data root lock: %w", err)
	}
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped); err != nil {
		_ = file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrRootInUse
		}
		return nil, fmt.Errorf("readerstore: lock data root: %w", err)
	}
	return &RootLock{file: file}, nil
}

func (l *RootLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	var overlapped windows.Overlapped
	unlockErr := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
	return errors.Join(unlockErr, file.Close())
}
