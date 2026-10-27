// Package httpx holds the shared HTTP plumbing for the JSON API: the response
// envelope, stable error codes, JSON decoding and session/CSRF helpers.
//
// It exists so both the user-facing API (internal/api) and the admin API
// (internal/api/admin) can share one implementation without either importing
// the other.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Error codes returned to clients. They are stable identifiers the frontend
// can branch on; the message is for humans.
const (
	CodeBadRequest   = "bad_request"
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeInternal     = "internal_error"
	CodeTooManyLogin = "too_many_attempts"
	CodeCSRF         = "csrf_failed"
	CodeNotAllowed   = "method_not_allowed"
	// CodeReadOnlyConfig reports that a setting could not be saved because
	// the configuration file is not writable.
	CodeReadOnlyConfig = "read_only_config"
)

// ErrorBody is the body of an error response.
type ErrorBody struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ErrorResponse wraps ErrorBody.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// CodedError associates an HTTP status with a stable client code.
type CodedError struct {
	Status  int
	Code    string
	Message string
}

// Error implements the error interface.
func (e *CodedError) Error() string { return e.Message }

// Errorf builds a CodedError.
func Errorf(status int, code, message string) *CodedError {
	return &CodedError{Status: status, Code: code, Message: message}
}

// WriteJSON sends payload as {"data": payload}.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(struct {
		Data any `json:"data"`
	}{Data: payload})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}

// WriteNoContent sends 204.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteError sends an error response. Errors of type *CodedError keep their
// status and code; anything else is reported as a generic 500.
func WriteError(w http.ResponseWriter, err error) {
	var coded *CodedError
	if errors.As(err, &coded) {
		writeErrorBody(w, coded.Status, coded.Code, coded.Message)
		return
	}
	writeErrorBody(w, http.StatusInternalServerError, CodeInternal, "internal error")
}

func writeErrorBody(w http.ResponseWriter, status int, code, message string) {
	body, _ := json.Marshal(ErrorResponse{Error: ErrorBody{Message: message, Code: code}})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}

// MaxJSONBody bounds a state-changing request body.
const MaxJSONBody = 64 * 1024

// DecodeJSON reads at most maxBytes from the request body into dst.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return Errorf(http.StatusBadRequest, CodeBadRequest, "invalid JSON body")
	}
	return nil
}
