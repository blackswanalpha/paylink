package correlation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paylink/mpesa-adapter/internal/correlation"
)

// fakeConn implements correlation's redis conn (Set/Get) structurally for unit tests.
type fakeConn struct{ m map[string]string }

func (f *fakeConn) Set(_ context.Context, k, v string, _ time.Duration) error {
	f.m[k] = v
	return nil
}

func (f *fakeConn) Get(_ context.Context, k string) (string, bool, error) {
	v, ok := f.m[k]
	return v, ok, nil
}

func TestMemory_PutGet(t *testing.T) {
	ctx := context.Background()
	m := correlation.NewMemory()

	rec := correlation.Record{PayLinkID: "0xabc", Amount: 1500, Receiver: "174379", PayerPhone: "254700000000"}
	if err := m.Put(ctx, "ws_CO_1", rec); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := m.Get(ctx, "ws_CO_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != rec {
		t.Fatalf("Get = %+v, want %+v", got, rec)
	}
}

func TestMemory_NotFound(t *testing.T) {
	_, err := correlation.NewMemory().Get(context.Background(), "missing")
	if !errors.Is(err, correlation.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestRedis_PutGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := correlation.NewRedis(&fakeConn{m: map[string]string{}}, time.Hour)

	rec := correlation.Record{PayLinkID: "0xabc", Amount: 1500, Receiver: "174379", PayerPhone: "254700000000"}
	if err := r.Put(ctx, "ws_CO_1", rec); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := r.Get(ctx, "ws_CO_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != rec {
		t.Fatalf("Get = %+v, want %+v", got, rec)
	}
}

func TestRedis_NotFound(t *testing.T) {
	r := correlation.NewRedis(&fakeConn{m: map[string]string{}}, time.Hour)
	if _, err := r.Get(context.Background(), "missing"); !errors.Is(err, correlation.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
