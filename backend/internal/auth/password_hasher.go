package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemoryKiB   uint32 = 19 * 1024
	argonIterations  uint32 = 2
	argonParallelism uint8  = 1
	argonSaltBytes          = 16
	argonHashBytes   uint32 = 32

	dummyPasswordHash = "$argon2id$v=19$m=19456,t=2,p=1$Tm92ZWxSZWFkZXJEdW1teQ$jDc1ZPjDV93wz6Dpo5PkaqhoDztB5hF+CwzxUnXfmLo"
)

var (
	ErrInvalidPasswordHash    = errors.New("auth: invalid password hash")
	ErrPasswordWorkOverloaded = errors.New("auth: password work capacity is full")

	passwordWorkAdmission = make(chan struct{}, 2)
)

type passwordKeyDeriver func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte

type PasswordHasher struct {
	derive passwordKeyDeriver
}

func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{derive: argon2.IDKey}
}

func (h *PasswordHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	if err := acquirePasswordWork(ctx); err != nil {
		return "", err
	}
	defer releasePasswordWork()

	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate password salt: %w", err)
	}
	hash := h.derive([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonHashBytes)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return encodePasswordHash(salt, hash), nil
}

func (h *PasswordHasher) Verify(ctx context.Context, password, encoded string) (bool, error) {
	salt, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	if err := acquirePasswordWork(ctx); err != nil {
		return false, err
	}
	defer releasePasswordWork()

	actual := h.derive([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonHashBytes)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

// DummyVerify performs normal password work for an unknown username and never authenticates it.
func (h *PasswordHasher) DummyVerify(ctx context.Context, password string) error {
	_, err := h.Verify(ctx, password, dummyPasswordHash)
	return err
}

func acquirePasswordWork(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case passwordWorkAdmission <- struct{}{}:
		return nil
	default:
		return ErrPasswordWorkOverloaded
	}
}

func releasePasswordWork() {
	<-passwordWorkAdmission
}

func encodePasswordHash(salt, hash []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemoryKiB,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

func parsePasswordHash(encoded string) ([]byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return nil, nil, ErrInvalidPasswordHash
	}
	if parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", argonMemoryKiB, argonIterations, argonParallelism) {
		return nil, nil, ErrInvalidPasswordHash
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != argonSaltBytes {
		return nil, nil, ErrInvalidPasswordHash
	}
	hash, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(hash) != int(argonHashBytes) {
		return nil, nil, ErrInvalidPasswordHash
	}
	return salt, hash, nil
}
