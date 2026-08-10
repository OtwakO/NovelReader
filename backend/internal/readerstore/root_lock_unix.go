//go:build unix

package readerstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
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
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
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
	unlockErr := unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return errors.Join(unlockErr, file.Close())
}
