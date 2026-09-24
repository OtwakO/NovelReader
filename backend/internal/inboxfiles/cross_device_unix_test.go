//go:build !windows

package inboxfiles

import (
	"os"
	"syscall"
	"testing"
)

func TestCrossDeviceFallbackDoesNotMaskOtherRenameFailures(t *testing.T) {
	if !IsCrossDevice(&os.LinkError{Op: "rename", Err: syscall.EXDEV}) {
		t.Fatal("cross-device rename not recognized")
	}
	if IsCrossDevice(&os.LinkError{Op: "rename", Err: syscall.EACCES}) {
		t.Fatal("permission error treated as a copy fallback")
	}
}
