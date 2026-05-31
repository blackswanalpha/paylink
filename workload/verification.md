# Verification — how to prove a change works end-to-end

Tests passing is necessary, not sufficient. Verify behavior. Commands reuse those already
documented in [`../CLAUDE.md`](../CLAUDE.md). Report what you actually ran and its result.

---

## Chain (`paylink-chain/`)
```bash
cd paylink-chain
go build ./...                      # compiles
go test ./... -count=1              # all unit + integration tests, no cache
make lint                           # go vet
make fmt                            # gofmt
```
**Live sanity:**
```bash
go run ./cmd/paylinkd                # single-validator node on :8545 (RPC), WS on by default
```
- Query chain info / a known RPC method and confirm a sane response.
- For a new tx type: submit it, confirm the expected event on the WebSocket datastream and
  the expected state change via RPC.
- For consensus changes: `go test ./internal/consensus/... -count=1` and observe block
  production under `--block-interval`.

## Backend service (`linkmint-backend/<name>`) — Python/FastAPI or Go/chi per ADR-003

**Python/FastAPI:**
```bash
cd linkmint-backend/<name>
ruff check . && black --check . && mypy .   # lint + format + types
pytest --cov --cov-fail-under=80            # unit + integration (testcontainers)
uvicorn app.main:app --reload               # run locally
```
**Go/chi:**
```bash
cd linkmint-backend/<name>
go build ./... && go vet ./... && gofmt -l .
go test ./... -count=1 -cover               # unit + integration; check coverage ≥80%
go run ./cmd/<name>                          # run locally
```
For either stack:
- Hit a `/v1/...` endpoint; confirm success shape and the standard error envelope on failure.
- Confirm `/internal/healthz`, `/internal/readyz`, `/metrics` respond.
- Confirm config comes only from env vars (no hard-coded hosts/secrets).
- Re-send a mutating request with the same `Idempotency-Key`; confirm it's idempotent.

## Adapter (`adapters/<rail>`)
- Replay a captured (or representative sandbox) rail callback into the adapter.
- Confirm it normalizes to exactly `{pl_id, rail, tx_id, amount, timestamp, sender, receiver, proof_signature}`.
- Confirm the proof is signed and broadcast to the validator.
- End-to-end: the corresponding PayLink settles on-chain (verify via RPC).

## SDK (`sdks/javascript`)
- Run the SDK's tests against a mock or the local stack.
- Exercise a create→read→settle flow; confirm typed responses and error-envelope handling.

## App (`apps/web`)
```bash
cd apps/web
npm run dev
```
- Drive the create-PayLink → pay (MPesa) → settlement flow against the local stack.
- Use the `/run` skill to launch and (optionally) screenshot; `/verify` to confirm behavior.

## Full stack (local)
```bash
docker-compose up -d                # start in-scope services
# ... exercise the end-to-end flow ...
docker-compose down
```

## The workload system itself
- Structure: `find workload .claude -type f | sort` — counts match the plan (31 + 9).
- Cross-links: every `workNN` ↔ `flowNN` reference resolves; `backlog.md` lists all items.
- Config loads: `.claude/settings.json` is valid JSON; `/agents`, `/help` show the project
  subagents, skills, and commands without error.
- Dry-run: `/work 01` loads `work01.md` + `flow01.md`; `/check-invariants` runs the auditor.

---

**Always include in your report:** exact command(s) run, pass/fail, and any failing output.
If you skipped a step, say which and why.
