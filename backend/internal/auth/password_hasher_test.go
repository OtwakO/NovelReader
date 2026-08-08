package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPasswordHasherUsesLightArgon2idAndFreshSalts(t *testing.T) {
	hasher := NewPasswordHasher()
	password := "correct horse battery staple"

	first, err := hasher.Hash(context.Background(), password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.Hash(context.Background(), password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("equal passwords produced equal hashes")
	}
	if !strings.HasPrefix(first, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash parameters = %q", first)
	}

	matched, err := hasher.Verify(context.Background(), password, first)
	if err != nil || !matched {
		t.Fatalf("correct password: matched=%v err=%v", matched, err)
	}
	matched, err = hasher.Verify(context.Background(), "wrong password value", first)
	if err != nil || matched {
		t.Fatalf("wrong password: matched=%v err=%v", matched, err)
	}
}

func TestPasswordHasherRejectsInvalidPasswordAndStoredHash(t *testing.T) {
	hasher := NewPasswordHasher()
	if _, err := hasher.Hash(context.Background(), "too short"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("short password error = %v", err)
	}

	invalid := []string{
		"",
		"not-a-phc-hash",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$aGFzaA",
		"$argon2id$v=16$m=19456,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$aGFzaA",
		"$argon2id$v=19$m=999999,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$aGFzaA",
	}
	for _, encoded := range invalid {
		matched, err := hasher.Verify(context.Background(), "correct horse battery staple", encoded)
		if matched || !errors.Is(err, ErrInvalidPasswordHash) {
			t.Errorf("Verify(%q): matched=%v err=%v", encoded, matched, err)
		}
	}
}

func TestPasswordHasherAdmissionIsProcessWideAndFailsQuickly(t *testing.T) {
	first := NewPasswordHasher()
	second := NewPasswordHasher()

	passwordWorkAdmission <- struct{}{}
	passwordWorkAdmission <- struct{}{}
	defer func() {
		<-passwordWorkAdmission
		<-passwordWorkAdmission
	}()

	if _, err := first.Hash(context.Background(), "correct horse battery staple"); !errors.Is(err, ErrPasswordWorkOverloaded) {
		t.Fatalf("hash overload error = %v", err)
	}
	if _, err := second.Verify(context.Background(), "correct horse battery staple", dummyPasswordHash); !errors.Is(err, ErrPasswordWorkOverloaded) {
		t.Fatalf("verify overload error = %v", err)
	}
	if err := first.DummyVerify(context.Background(), "unknown account password"); !errors.Is(err, ErrPasswordWorkOverloaded) {
		t.Fatalf("dummy overload error = %v", err)
	}
}

func TestPasswordHasherHonorsCanceledContextAndDummyNeverAuthenticates(t *testing.T) {
	hasher := NewPasswordHasher()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hasher.Hash(ctx, "correct horse battery staple"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled hash error = %v", err)
	}
	if err := hasher.DummyVerify(context.Background(), "correct horse battery staple"); err != nil {
		t.Fatalf("dummy verify: %v", err)
	}
}

func TestPasswordHasherDiscardsResultWhenCanceledDuringWork(t *testing.T) {
	hasher := NewPasswordHasher()
	original := hasher.derive

	started := make(chan struct{})
	release := make(chan struct{})
	hasher.derive = func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
		close(started)
		<-release
		return original(password, salt, time, memory, threads, keyLen)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := hasher.Hash(ctx, "correct horse battery staple")
		done <- err
	}()
	<-started
	cancel()
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled in-flight hash error = %v", err)
	}
}
