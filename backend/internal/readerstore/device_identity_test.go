package readerstore

import "testing"

func TestDeviceIDIsStableOpaqueAndReaderScoped(t *testing.T) {
	first := UserID("11111111-1111-4111-8111-111111111111")
	second := UserID("22222222-2222-4222-8222-222222222222")
	firstID := DeviceID(first)
	if firstID != DeviceID(first) {
		t.Fatal("device identity was not stable")
	}
	if len(firstID) != 16 || firstID == string(first) {
		t.Fatalf("device identity=%q", firstID)
	}
	if firstID == DeviceID(second) {
		t.Fatal("different readers shared a device identity")
	}
}
