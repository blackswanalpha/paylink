// Unit tests for the pure pieces of the postgres store (no Docker needed; the pgx-backed
// behavior is covered by postgres_integration_test.go behind -tags=integration).
package postgres

import (
	"context"
	"testing"

	"github.com/paylink/escrow-manager/internal/fsm"
)

func TestNewRejectsBadDSN(t *testing.T) {
	if _, err := New(context.Background(), "://not-a-dsn"); err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestStateScanner(t *testing.T) {
	var st fsm.State
	ss := &stateScanner{&st}
	if err := ss.Scan("WAITING"); err != nil || st != fsm.StateWaiting {
		t.Fatalf("string scan: %v / %q", err, st)
	}
	if err := ss.Scan([]byte("RELEASED")); err != nil || st != fsm.StateReleased {
		t.Fatalf("bytes scan: %v / %q", err, st)
	}
	if err := ss.Scan(42); err == nil {
		t.Fatal("unsupported source must error")
	}
}

// fakeRow drives scanEscrow's error branches without a database.
type fakeRow struct {
	err    error
	params []byte
}

func (f fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	if p, ok := dest[8].(*[]byte); ok {
		*p = f.params
	}
	if s, ok := dest[9].(*string); ok {
		*s = "WAITING"
	}
	return nil
}

func TestScanEscrowErrors(t *testing.T) {
	if _, err := scanEscrow(fakeRow{err: context.Canceled}); err == nil {
		t.Fatal("scan error must propagate")
	}
	if _, err := scanEscrow(fakeRow{params: []byte(`{not json`)}); err == nil {
		t.Fatal("invalid condition_params JSON must error")
	}
	e, err := scanEscrow(fakeRow{params: nil}) // NULL/empty params are fine
	if err != nil || e.State != fsm.StateWaiting {
		t.Fatalf("empty params: %v / %+v", err, e)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(context.Canceled) {
		t.Fatal("plain error is not a unique violation")
	}
}
