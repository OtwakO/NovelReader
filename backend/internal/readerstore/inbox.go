package readerstore

import (
	"errors"
	"os"
	"path"
	"strings"
)

const InboxDirectory = "inbox"

// OpenInbox opens this reader's nonportable ingress outside the replaceable home.
// The caller holds its Home lease and closes the returned root.
func (f FileStore) OpenInbox() (*os.Root, error) {
	root, err := f.openInboxAnchor()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.OpenRoot(path.Join(InboxDirectory, string(f.readerID)))
}

func (f FileStore) openInboxAnchor() (*os.Root, error) {
	if err := validateUserID(f.readerID); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(f.dataRoot)
	if err != nil {
		return nil, err
	}
	for _, directory := range []string{InboxDirectory, path.Join(InboxDirectory, string(f.readerID))} {
		if err := root.Mkdir(directory, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			root.Close()
			return nil, err
		}
		info, err := root.Lstat(directory)
		if err != nil {
			root.Close()
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, ErrInvalidFilePath
		}
	}
	return root, nil
}

// MoveInboxTo moves a single ingress entry into this reader's files. The caller
// must persist acquisition intent and hold LockMutation before using this method.
// Both paths stay under one anchored root, including when inbox is a bind mount.
func (f FileStore) MoveInboxTo(name, destination string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return ErrInvalidFilePath
	}
	if strings.ContainsAny(destination, "\\\x00") {
		return ErrInvalidFilePath
	}
	relative, err := filePath(strings.Split(destination, "/"))
	if err != nil {
		return err
	}
	root, err := f.openInboxAnchor()
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Rename(path.Join(InboxDirectory, string(f.readerID), name), path.Join(UsersDirectory, string(f.readerID), FilesDirectory, relative))
}
