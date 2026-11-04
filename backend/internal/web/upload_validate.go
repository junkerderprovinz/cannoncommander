package web

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
)

// uploadTempPath returns the temporary path used while assembling an upload for realPath.
// sessionID isolates concurrent uploads targeting the same destination.
func uploadTempPath(realPath, sessionID string) string {
	hasher := md5.New()
	hasher.Write([]byte(realPath))
	hasher.Write([]byte{0})
	hasher.Write([]byte(sessionID))
	return fmt.Sprintf("%s.%s.uploading.tmp", realPath, hex.EncodeToString(hasher.Sum(nil)))
}

// parsePutTotalSize returns the declared body size for PUT / in-place writes.
// Uses X-File-Total-Size when set, otherwise Content-Length when positive.
func parsePutTotalSize(r *http.Request) (total int64, ok bool, err error) {
	total, ok, err = parseUploadTotalSize(r)
	if err != nil || ok {
		return total, ok, err
	}
	if r.ContentLength > 0 {
		return r.ContentLength, true, nil
	}
	return 0, false, nil
}

// parseUploadTotalSize reads optional X-File-Total-Size.
// ok is false when the header is absent.
func parseUploadTotalSize(r *http.Request) (total int64, ok bool, err error) {
	s := r.Header.Get("X-File-Total-Size")
	if s == "" {
		return 0, false, nil
	}
	total, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid total size: %v", err)
	}
	if total < 0 {
		return 0, false, fmt.Errorf("invalid total size: %d", total)
	}
	return total, true, nil
}

// validateReceivedBytes compares bytes written to optional declared totals.
// expectedTotal is checked only when hasExpected is true.
// contentLength is checked when greater than 0 (known Content-Length).
func validateReceivedBytes(written, expectedTotal int64, hasExpected bool, contentLength int64) error {
	if hasExpected && written != expectedTotal {
		return fmt.Errorf("upload incomplete: expected %d bytes, received %d", expectedTotal, written)
	}
	if contentLength > 0 && written != contentLength {
		return fmt.Errorf("upload incomplete: expected %d bytes, received %d", contentLength, written)
	}
	return nil
}
