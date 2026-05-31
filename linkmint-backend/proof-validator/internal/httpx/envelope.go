// Package httpx holds HTTP plumbing shared by all routes: the standard LinkMint error envelope,
// request middleware (correlation id, logging, panic recovery), and JSON helpers. Mirrors the
// work02 reference template (payment-orchestrator/internal/httpx).
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// ErrorCode is a stable, machine-readable error code returned in the envelope.
type ErrorCode string

const (
	CodeInvalidPayload        ErrorCode = "INVALID_PAYLOAD"
	CodeInvalidProofShape     ErrorCode = "INVALID_PROOF_SHAPE"
	CodeInvalidProofSignature ErrorCode = "INVALID_PROOF_SIGNATURE"
	CodePayLinkNotFound       ErrorCode = "PAYLINK_NOT_FOUND"
	CodePayLinkNotPayable     ErrorCode = "PAYLINK_NOT_PAYABLE"
	CodePayLinkExpired        ErrorCode = "PAYLINK_EXPIRED"
	CodeProofAmountMismatch   ErrorCode = "PROOF_AMOUNT_MISMATCH"
	CodeProofExists           ErrorCode = "PROOF_EXISTS"
	CodeProofNotFound         ErrorCode = "PROOF_NOT_FOUND"
	CodeIdempotentConflict    ErrorCode = "IDEMPOTENT_CONFLICT"
	CodeChainUnavailable      ErrorCode = "CHAIN_UNAVAILABLE"
	CodeServiceNotReady       ErrorCode = "SERVICE_NOT_READY"
	CodeInternalError         ErrorCode = "INTERNAL_ERROR"
)

var httpStatus = map[ErrorCode]int{
	CodeInvalidPayload:        http.StatusBadRequest,
	CodeInvalidProofShape:     http.StatusBadRequest,
	CodeInvalidProofSignature: http.StatusUnauthorized,
	CodePayLinkNotFound:       http.StatusNotFound,
	CodePayLinkNotPayable:     http.StatusConflict,
	CodePayLinkExpired:        http.StatusConflict,
	CodeProofAmountMismatch:   http.StatusConflict,
	CodeProofExists:           http.StatusConflict,
	CodeProofNotFound:         http.StatusNotFound,
	CodeIdempotentConflict:    http.StatusConflict,
	CodeChainUnavailable:      http.StatusBadGateway,
	CodeServiceNotReady:       http.StatusServiceUnavailable,
	CodeInternalError:         http.StatusInternalServerError,
}

// StatusFor returns the HTTP status mapped to a code (500 for unknown codes).
func StatusFor(code ErrorCode) int {
	if s, ok := httpStatus[code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

// AppError is a domain error that serializes to the standard envelope.
type AppError struct {
	Code    ErrorCode
	Message string
	Details map[string]any
	// HTTPStatus overrides the default status mapped from Code when non-zero.
	HTTPStatus int
}

func (e *AppError) Error() string { return string(e.Code) + ": " + e.Message }

// NewError builds an AppError with the default status for its code.
func NewError(code ErrorCode, message string, details map[string]any) *AppError {
	return &AppError{Code: code, Message: message, Details: details}
}

// Status returns the resolved HTTP status for the error.
func (e *AppError) Status() int {
	if e.HTTPStatus != 0 {
		return e.HTTPStatus
	}
	return StatusFor(e.Code)
}

// AsAppError unwraps err to an *AppError, or wraps it as an opaque INTERNAL_ERROR.
func AsAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return NewError(CodeInternalError, "internal error", nil)
}

// envelopeBody is the exact LinkMint error envelope shape.
type envelopeBody struct {
	Error errorObject `json:"error"`
}

type errorObject struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
	TraceID string         `json:"trace_id"`
}

// Envelope builds the standard error body for the given request context.
func Envelope(ctx context.Context, code ErrorCode, message string, details map[string]any) any {
	if details == nil {
		details = map[string]any{}
	}
	return envelopeBody{Error: errorObject{
		Code:    string(code),
		Message: message,
		Details: details,
		TraceID: TraceIDFromContext(ctx),
	}}
}

// WriteError writes err as the standard envelope. Unknown errors become INTERNAL_ERROR.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	ae := AsAppError(err)
	WriteJSON(w, ae.Status(), Envelope(r.Context(), ae.Code, ae.Message, ae.Details))
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
