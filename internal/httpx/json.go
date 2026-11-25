package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJSONBody = 64 << 10

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// DecodeJSON decodes the request body into dst (64 KiB cap, unknown fields rejected).
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return E(http.StatusBadRequest, "bad_request", "empty body")
		}
		return E(http.StatusBadRequest, "bad_request", "invalid JSON: "+err.Error())
	}
	return nil
}
