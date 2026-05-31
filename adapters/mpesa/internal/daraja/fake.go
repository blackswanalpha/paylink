package daraja

import "context"

// FakeClient is a test double for Client: it records the last STKPush params and returns a canned
// result (or a configured error). No network. Used by domain/server tests.
type FakeClient struct {
	Result STKPushResult
	Err    error

	// Calls captures every STKPush invocation for assertions.
	Calls []STKPushParams
}

// STKPush records the params and returns the configured Result/Err.
func (f *FakeClient) STKPush(_ context.Context, p STKPushParams) (STKPushResult, error) {
	f.Calls = append(f.Calls, p)
	if f.Err != nil {
		return STKPushResult{}, f.Err
	}
	res := f.Result
	if res.CheckoutRequestID == "" {
		res.CheckoutRequestID = "ws_CO_fake_" + p.AccountRef
	}
	return res, nil
}
