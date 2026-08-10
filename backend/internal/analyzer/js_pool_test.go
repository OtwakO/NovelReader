package analyzer

import "testing"

func TestNewJSVMWithPoolSizeBoundsRuntimePool(t *testing.T) {
	if got := cap(NewJSVMWithPoolSize(4).executor.pool); got != 4 {
		t.Fatalf("pool capacity=%d, want 4", got)
	}
	if got := cap(NewJSVMWithPoolSize(0).executor.pool); got != 1 {
		t.Fatalf("clamped pool capacity=%d, want 1", got)
	}
}
