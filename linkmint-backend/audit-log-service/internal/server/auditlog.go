package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/paylink/audit-log-service/internal/domain"
	"github.com/paylink/audit-log-service/internal/httpx"
	idempotency "github.com/paylink/idempotency-go"
)

// idemError maps an idempotency-library error to this service's HTTP envelope: a conflict becomes
// 409 IDEMPOTENT_CONFLICT, any other (backend) error a 500. The library is transport-free, so the
// status mapping lives here at the service boundary.
func idemError(err error) error {
	if errors.Is(err, idempotency.ErrConflict) {
		return httpx.NewError(httpx.CodeIdempotentConflict, err.Error(), nil)
	}
	return httpx.NewError(httpx.CodeInternalError, err.Error(), nil)
}

const (
	maxBodyBytes = 1 << 20 // 1 MiB
	idemHeader   = "Idempotency-Key"
	appendRoute  = "append"
)

// actorDTO is the canonical actor shape {id?, kind}.
type actorDTO struct {
	ID   *string `json:"id"`
	Kind string  `json:"kind"`
}

// postEntryRequest is the POST /v1/audit-log body. actor accepts {id,kind} or a bare string (the
// admin-backoffice AuditRecord sends the JWT sub as a string — a documented compatibility shim).
type postEntryRequest struct {
	Actor      json.RawMessage `json:"actor"`
	Action     string          `json:"action"`
	Resource   string          `json:"resource"`
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
	Context    json.RawMessage `json:"context"`
	OccurredAt *time.Time      `json:"occurred_at,omitempty"`
}

type postEntryResponse struct {
	EntryID int64  `json:"entry_id"`
	Hash    string `json:"hash"`
}

func (req postEntryRequest) toAppendInput() (domain.AppendInput, error) {
	actor, err := parseActor(req.Actor)
	if err != nil {
		return domain.AppendInput{}, err
	}
	in := domain.AppendInput{
		Action:   req.Action,
		Resource: req.Resource,
		Before:   req.Before,
		After:    req.After,
		Context:  req.Context,
		Actor:    actor,
	}
	if req.OccurredAt != nil {
		in.OccurredAt = *req.OccurredAt
	}
	return in, nil
}

// parseActor accepts {"id","kind"} (canonical) or a bare JSON string (compat → kind=user, id parsed
// if it is a UUID).
func parseActor(raw json.RawMessage) (domain.Actor, error) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return domain.Actor{}, httpx.NewError(httpx.CodeInvalidPayload, "actor is required", nil)
	}
	if t[0] == '"' { // bare string actor
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return domain.Actor{}, httpx.NewError(httpx.CodeInvalidPayload, "actor string is invalid", nil)
		}
		a := domain.Actor{Kind: domain.ActorUser}
		if u, err := uuid.Parse(s); err == nil {
			a.ID = &u
		}
		return a, nil
	}
	var dto actorDTO
	dec := json.NewDecoder(bytes.NewReader(t))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&dto); err != nil {
		return domain.Actor{}, httpx.NewError(httpx.CodeInvalidPayload, "actor must be an object {id,kind} or a string", nil)
	}
	a := domain.Actor{Kind: domain.ActorKind(dto.Kind)}
	if dto.ID != nil && *dto.ID != "" {
		u, err := uuid.Parse(*dto.ID)
		if err != nil {
			return domain.Actor{}, httpx.NewError(httpx.CodeInvalidPayload, "actor.id must be a UUID", nil)
		}
		a.ID = &u
	}
	return a, nil
}

// postEntry handles POST /v1/audit-log. The Idempotency-Key header is honored when present (a
// producer retry replays the cached {entry_id,hash}); it is NOT required — an audit signal must
// never be dropped for a missing header.
func (s *Server) postEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "could not read request body", nil))
		return
	}
	var req postEntryRequest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "request body is not valid JSON: "+err.Error(), nil))
		return
	}
	in, err := req.toAppendInput()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	idemKey := r.Header.Get(idemHeader)
	if idemKey == "" {
		e, err := s.svc.Append(ctx, in)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, postEntryResponse{EntryID: e.EntryID, Hash: hex.EncodeToString(e.EntryHash)})
		return
	}

	fp := idempotency.Fingerprint(raw)
	cached, err := s.idem.Begin(ctx, appendRoute, idemKey, fp)
	if err != nil {
		httpx.WriteError(w, r, idemError(err))
		return
	}
	if cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cached.Status)
		_, _ = w.Write(cached.Body)
		return
	}
	e, err := s.svc.Append(ctx, in)
	if err != nil {
		s.idem.Release(ctx, appendRoute, idemKey)
		httpx.WriteError(w, r, err)
		return
	}
	resp := postEntryResponse{EntryID: e.EntryID, Hash: hex.EncodeToString(e.EntryHash)}
	body, _ := json.Marshal(resp) // postEntryResponse is statically marshalable — cannot fail
	if err := s.idem.Complete(ctx, appendRoute, idemKey, fp, http.StatusCreated, body); err != nil {
		s.log.Warn("idempotency_complete_failed", "err", err.Error())
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// getEntry handles GET /v1/audit-log/{entry_id} — the entry plus its inclusion proof.
func (s *Server) getEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "entry_id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidPayload, "entry_id must be a positive integer", nil))
		return
	}
	e, proof, err := s.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"entry": toView(e), "proof": proof})
}

// listEntries handles GET /v1/audit-log?actor=&resource=&from=&to=&cursor=&limit= (newest-first).
func (s *Server) listEntries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f domain.QueryFilter

	if a := strings.TrimSpace(q.Get("actor")); a != "" {
		u, err := uuid.Parse(a)
		if err != nil {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidQuery, "actor must be a UUID", nil))
			return
		}
		f.Actor = &u
	}
	f.Resource = strings.TrimSpace(q.Get("resource"))

	from, err := parseTimeParam(q.Get("from"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	f.From = from
	to, err := parseTimeParam(q.Get("to"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	f.To = to

	if c := strings.TrimSpace(q.Get("cursor")); c != "" {
		cur, err := strconv.ParseInt(c, 10, 64)
		if err != nil || cur < 0 {
			httpx.WriteError(w, r, httpx.NewError(httpx.CodeInvalidQuery, "cursor must be a non-negative integer", nil))
			return
		}
		f.Cursor = cur
	}
	f.Limit = parseLimit(q.Get("limit"), 20, 100)

	page, err := s.svc.Query(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	views := make([]entryView, 0, len(page.Items))
	for _, e := range page.Items {
		views = append(views, toView(e))
	}
	resp := map[string]any{"items": views, "next_cursor": nil}
	if page.NextCursor != nil {
		resp["next_cursor"] = strconv.FormatInt(*page.NextCursor, 10)
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// verifyChain handles GET /v1/audit-log/verify?from=&to= → {ok, broken_at?}.
func (s *Server) verifyChain(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err := parseTimeParam(q.Get("from"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	to, err := parseTimeParam(q.Get("to"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := s.svc.Verify(r.Context(), from, to)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp := map[string]any{"ok": res.OK}
	if res.BrokenAt != nil {
		resp["broken_at"] = *res.BrokenAt
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

type actorView struct {
	ID   *string `json:"id"`
	Kind string  `json:"kind"`
}

type entryView struct {
	EntryID    int64           `json:"entry_id"`
	OccurredAt string          `json:"occurred_at"`
	Actor      actorView       `json:"actor"`
	Action     string          `json:"action"`
	Resource   string          `json:"resource"`
	Before     json.RawMessage `json:"before,omitempty"`
	After      json.RawMessage `json:"after,omitempty"`
	Context    json.RawMessage `json:"context"`
	PrevHash   string          `json:"prev_hash"`
	EntryHash  string          `json:"entry_hash"`
}

func toView(e domain.Entry) entryView {
	var idp *string
	if e.Actor.ID != nil {
		s := e.Actor.ID.String()
		idp = &s
	}
	return entryView{
		EntryID:    e.EntryID,
		OccurredAt: e.OccurredAt.UTC().Format(time.RFC3339Nano),
		Actor:      actorView{ID: idp, Kind: string(e.Actor.Kind)},
		Action:     e.Action,
		Resource:   e.Resource,
		Before:     e.Before,
		After:      e.After,
		Context:    e.Context,
		PrevHash:   hex.EncodeToString(e.PrevHash),
		EntryHash:  hex.EncodeToString(e.EntryHash),
	}
}

func parseTimeParam(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, httpx.NewError(httpx.CodeInvalidQuery, "from/to must be RFC3339 timestamps", nil)
	}
	return &t, nil
}

func parseLimit(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
