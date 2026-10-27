package api

import "github.com/filegator/filegator/internal/api/httpx"

// The response envelope, error codes and JSON helpers live in httpx so the
// admin API can share them. These aliases keep call sites in this package
// reading naturally.

const (
	CodeBadRequest     = httpx.CodeBadRequest
	CodeUnauthorized   = httpx.CodeUnauthorized
	CodeForbidden      = httpx.CodeForbidden
	CodeNotFound       = httpx.CodeNotFound
	CodeConflict       = httpx.CodeConflict
	CodeInternal       = httpx.CodeInternal
	CodeTooManyLogin   = httpx.CodeTooManyLogin
	CodeCSRF           = httpx.CodeCSRF
	CodeNotAllowed     = httpx.CodeNotAllowed
	CodeReadOnlyConfig = httpx.CodeReadOnlyConfig
)

// APICodeError is a client-safe error carrying an HTTP status and code.
type APICodeError = httpx.CodedError

// Errorf builds an APICodeError.
var Errorf = httpx.Errorf

// WriteJSON sends payload as {"data": payload}.
var WriteJSON = httpx.WriteJSON

// WriteNoContent sends 204.
var WriteNoContent = httpx.WriteNoContent

// WriteError sends an error response, mapping *APICodeError on the way.
var WriteError = httpx.WriteError

// DecodeJSON reads at most maxBytes from the request body into dst.
var DecodeJSON = httpx.DecodeJSON
