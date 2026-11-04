package v1

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// maxBody caps JSON request bodies. Uploads go through their own endpoints.
const maxBody = 4 << 20

// WriteJSON answers with v as JSON. Successful GET answers carry a weak ETag
// over the body, and a matching If-None-Match gets a 304 instead, so a
// client can poll a listing cheaply.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, v any) error {
	return WriteJSONTagged(w, r, status, v, nil)
}

// WriteJSONTagged is WriteJSON with the ETag taken over tagged instead of
// the whole body, for answers that carry a part that changes all the time
// without mattering, such as free disk space. A nil tagged means the body.
func WriteJSONTagged(w http.ResponseWriter, r *http.Request, status int, v, tagged any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	h := w.Header()
	h.Set("Content-Type", "application/json")
	if status == http.StatusOK && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		tagBody := body
		if tagged != nil {
			if tagBody, err = json.Marshal(tagged); err != nil {
				return err
			}
		}
		sum := sha256.Sum256(tagBody)
		tag := `W/"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
		h.Set("ETag", tag)
		h.Set("Cache-Control", "no-cache")
		if etagMatches(r.Header.Get("If-None-Match"), tag) {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, err = w.Write(body)
	}
	return err
}

func etagMatches(header, tag string) bool {
	for _, t := range strings.Split(header, ",") {
		t = strings.TrimSpace(t)
		if t == "*" || strings.TrimPrefix(t, "W/") == strings.TrimPrefix(tag, "W/") {
			return true
		}
	}
	return false
}

// DecodeJSON reads the request body into v and rejects unknown fields, so
// a typo in a field name shows up as an error instead of being ignored.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Errorf(http.StatusBadRequest, "invalid JSON body: "+err.Error())
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return Errorf(http.StatusBadRequest, "invalid JSON body: trailing data")
	}
	return nil
}
