// Package httpx holds HTTP plumbing shared by every handler.
package httpx

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Error is an API error rendered as {"error":{code,message,fields}}.
type Error struct {
	Status     int
	Code       string
	Message    string
	Fields     map[string]string
	RetryAfter time.Duration
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// E builds an Error.
func E(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Field records a per-field validation reason and returns e.
func (e *Error) Field(name, reason string) *Error {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[name] = reason
	return e
}

// Validation returns an empty 422 validation error to add fields to.
func Validation() *Error { return E(http.StatusUnprocessableEntity, "validation", "validation failed") }

var (
	ErrNotFound     = E(http.StatusNotFound, "not_found", "not found")
	ErrForbidden    = E(http.StatusForbidden, "forbidden", "forbidden")
	ErrUnauthorized = E(http.StatusUnauthorized, "unauthorized", "sign in required")
)

type errorBody struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields,omitempty"`
	} `json:"error"`
}

// WriteError renders err; anything that is not an *Error becomes a logged 500.
func WriteError(w http.ResponseWriter, err error) {
	var e *Error
	if !errors.As(err, &e) {
		slog.Error("internal error", "err", err)
		e = E(http.StatusInternalServerError, "internal", "internal error")
	}
	var body errorBody
	body.Error.Code = e.Code
	body.Error.Message = e.Message
	if len(e.Fields) > 0 {
		body.Error.Fields = e.Fields
	}
	if e.RetryAfter > 0 {
		secs := int(e.RetryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
	}
	WriteJSON(w, e.Status, body)
}

// HandlerFunc is an http handler that may return an error.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := f(w, r); err != nil {
		WriteError(w, err)
	}
}
