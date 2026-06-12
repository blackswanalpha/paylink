package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// maxBodyBytes caps request bodies to avoid unbounded reads.
const maxBodyBytes = 1 << 20 // 1 MiB

// DecodeJSON strictly decodes the request body into v, returning an INVALID_PAYLOAD
// AppError on malformed JSON, unknown fields, or trailing data.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return NewError(CodeInvalidPayload, "request body is not valid JSON: "+err.Error(), nil)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return NewError(CodeInvalidPayload, "request body must contain a single JSON object", nil)
	}
	return nil
}
