//go:build !windows

package txtstore

import (
	"os"
	"syscall"
	"testing"
)

func TestCrossDeviceFallbackDoesNotMaskOtherRenameFailures(t *testing.T) {
	if !isCrossDevice(&os.LinkError{Op: "rename", Err: syscall.EXDEV}) {
		t.Fatal("cross-device rename not recognized")
	}
	if isCrossDevice(&os.LinkError{Op: "rename", Err: syscall.EACCES}) {
		t.Fatal("permission error treated as a copy fallback")
	}
}
