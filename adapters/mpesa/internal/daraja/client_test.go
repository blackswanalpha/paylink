package daraja_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/paylink/mpesa-adapter/internal/daraja"
	"github.com/paylink/mpesa-adapter/internal/httpx"
)

func TestHTTPClient_STKPush_Success(t *testing.T) {
	var gotToken string
	var gotBody daraja.STKPushParams
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stk" {
			t.Errorf("path = %s, want /stk", r.URL.Path)
		}
		gotToken = r.Header.Get(daraja.InternalTokenHeader)
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"checkout_request_id":"ws_CO_1","merchant_request_id":"m1","response_code":"0","customer_message":"ok"}`))
	}))
	defer srv.Close()

	c := daraja.NewHTTPClient(srv.URL, "secret-token", &http.Client{Timeout: 2 * time.Second})
	res, err := c.STKPush(context.Background(), daraja.STKPushParams{
		ShortCode: "600111", PayerPhone: "254700000000", Amount: 1500, AccountRef: "abc123", PayLinkID: "0xabc",
	})
	if err != nil {
		t.Fatalf("STKPush: %v", err)
	}
	if res.CheckoutRequestID != "ws_CO_1" {
		t.Fatalf("checkout_request_id = %q", res.CheckoutRequestID)
	}
	if gotToken != "secret-token" {
		t.Fatalf("internal token header = %q, want secret-token", gotToken)
	}
	if gotBody.ShortCode != "600111" || gotBody.Amount != 1500 {
		t.Fatalf("forwarded body wrong: %+v", gotBody)
	}
}

func TestHTTPClient_STKPush_RailError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := daraja.NewHTTPClient(srv.URL, "", &http.Client{Timeout: 2 * time.Second})
	_, err := c.STKPush(context.Background(), daraja.STKPushParams{ShortCode: "1", PayerPhone: "254", Amount: 1})
	if err == nil {
		t.Fatal("expected error on 502")
	}
	var ae *httpx.AppError
	if !errors.As(err, &ae) || ae.Code != httpx.CodeDarajaUnavailable {
		t.Fatalf("want DARAJA_UNAVAILABLE, got %v", err)
	}
}
