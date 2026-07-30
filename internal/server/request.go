package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const maxJSONRequestBytes = 2 << 20

// decodeJSONBody applies the common safety and compatibility rules for JSON
// endpoints. An omitted Content-Type remains valid for CLI clients, while an
// explicit media type must be application/json.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			writeJSONError(w, http.StatusUnsupportedMediaType, "content type must be application/json")
			return false
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "json body is too large")
		} else {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "invalid json body")
		return false
	}
	return true
}
