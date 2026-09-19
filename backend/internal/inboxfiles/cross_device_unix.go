//go:build !windows

package inboxfiles

import (
	"errors"
	"syscall"
)

func IsCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }
