package sourceexec

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const maxDataPayloadBytes = 4 << 20

type dataTransport struct{}

func (dataTransport) Do(ctx context.Context, spec RequestSpec) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}

	payload, err := decodeDataPayload(spec.URL)
	if err != nil {
		return Response{}, err
	}
	return Response{
		StatusCode: http.StatusOK,
		Body:       hex.EncodeToString(payload),
		FinalURL:   spec.URL,
		Transport:  "data",
	}, nil
}

func isTypedDataRequest(spec RequestSpec) bool {
	return strings.TrimSpace(spec.Type) != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(spec.URL)), "data:")
}

func decodeDataPayload(rawURL string) ([]byte, error) {
	comma := strings.IndexByte(rawURL, ',')
	if comma < len("data:") {
		return nil, fmt.Errorf("sourceexec: malformed data URI")
	}
	metadata := rawURL[len("data:"):comma]
	if !hasBase64Marker(metadata) {
		return nil, fmt.Errorf("sourceexec: typed data URI must be base64 encoded")
	}

	encoded := rawURL[comma+1:]
	if decoded, err := url.PathUnescape(encoded); err == nil {
		encoded = decoded
	} else {
		return nil, fmt.Errorf("sourceexec: decode data URI escaping: %w", err)
	}
	if base64.StdEncoding.DecodedLen(len(encoded)) > maxDataPayloadBytes {
		return nil, fmt.Errorf("sourceexec: data payload exceeds %d bytes", maxDataPayloadBytes)
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("sourceexec: decode base64 data payload: %w", err)
	}
	if len(payload) > maxDataPayloadBytes {
		return nil, fmt.Errorf("sourceexec: data payload exceeds %d bytes", maxDataPayloadBytes)
	}
	return payload, nil
}

func hasBase64Marker(metadata string) bool {
	for _, part := range strings.Split(metadata, ";") {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			return true
		}
	}
	return false
}
