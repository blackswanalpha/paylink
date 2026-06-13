package server

import (
	"crypto/subtle"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/paylink/settlement-service/internal/domain"
	"github.com/paylink/settlement-service/internal/httpx"
	"github.com/paylink/settlement-service/internal/ingest"
)

const (
	ingestTokenHeader = "X-Internal-Token"
	railHeader        = "X-Rail"
	fileIDHeader      = "X-File-Id"
	maxIngestBytes    = 8 << 20 // 8 MiB
)

// ingestResultView is the API representation of a rail-file ingest result.
type ingestResultView struct {
	FileID    string `json:"file_id"`
	Rail      string `json:"rail"`
	LineCount int    `json:"line_count"`
	Matched   int    `json:"matched"`
	Unmatched int    `json:"unmatched"`
}

// ingestRailFile handles POST /settlements/files/ingest — internal/trusted-network only. It accepts
// a JSON or CSV rail settlement file (body), parses it, matches lines to payouts (marking them
// PAID), and leaves unmatched lines for work27. Guarded by the SETTLEMENT_INGEST_TOKEN shared secret.
func (s *Server) ingestRailFile(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeIngest(r) {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeForbidden, "invalid or missing internal token", nil))
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIngestBytes))
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "could not read request body", nil))
		return
	}
	file, err := ingest.Parse(body, r.Header.Get(railHeader))
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "could not parse rail file: "+err.Error(), nil))
		return
	}
	fileID := r.Header.Get(fileIDHeader)
	if fileID == "" {
		fileID = uuid.NewString()
	}
	res, err := s.svc.IngestRailFile(r.Context(), domain.RailFileInput{
		Rail: file.Rail, FileID: fileID, Lines: file.Lines,
	})
	if err != nil {
		httpx.WriteError(w, r, mapErr(err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ingestResultView{
		FileID: res.FileID, Rail: res.Rail, LineCount: res.LineCount,
		Matched: res.Matched, Unmatched: res.Unmatched,
	})
}

// authorizeIngest checks the internal token (constant-time). An empty configured token disables the
// check (local dev only) — production sets SETTLEMENT_INGEST_TOKEN and terminates mTLS upstream.
func (s *Server) authorizeIngest(r *http.Request) bool {
	if s.ingestToken == "" {
		return true
	}
	got := r.Header.Get(ingestTokenHeader)
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.ingestToken)) == 1
}
