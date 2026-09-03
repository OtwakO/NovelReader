package readerstore

import (
	"crypto/sha256"
	"encoding/hex"
)

// DeviceID derives the opaque Legado-compatible device identity for one immutable reader.
func DeviceID(userID UserID) string {
	digest := sha256.Sum256([]byte("novelreader:reader-device:" + string(userID)))
	return hex.EncodeToString(digest[:8])
}
