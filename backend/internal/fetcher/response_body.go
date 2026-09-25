package fetcher

import (
	"errors"
	"fmt"
	"io"
)

var ErrResponseTooLarge = errors.New("fetcher: text response exceeds 10 MiB")

// ReadTextBody admits a complete raw text response before charset decoding.
// Read one extra byte so hitting the limit cannot masquerade as end-of-input.
func ReadTextBody(reader io.Reader) ([]byte, error) {
	const limit = 10 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("fetcher: read body: %w", err)
	}
	if len(body) > limit {
		return nil, ErrResponseTooLarge
	}
	return body, nil
}
