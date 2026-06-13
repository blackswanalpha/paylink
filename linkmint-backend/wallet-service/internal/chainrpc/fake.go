package chainrpc

import "context"

// FakeClient is an in-memory ChainReader for unit tests (domain/server). It avoids needing a live
// chain: set the maps/fields, or set Err to simulate the chain being unreachable (ErrUnavailable).
type FakeClient struct {
	Accounts   map[string]Account
	Nonces     map[string]uint64
	Validators map[string]Validator
	Stats      StakingStats
	Tokens     TokenStats
	Info       ChainInfo
	Height     uint64
	// Err, when non-nil, is returned by every method (use ErrUnavailable to simulate chain-down).
	Err error
}

// NewFake builds an empty FakeClient with initialized maps.
func NewFake() *FakeClient {
	return &FakeClient{
		Accounts:   map[string]Account{},
		Nonces:     map[string]uint64{},
		Validators: map[string]Validator{},
	}
}

func (f *FakeClient) GetAccount(_ context.Context, addr string) (Account, error) {
	if f.Err != nil {
		return Account{}, f.Err
	}
	if a, ok := f.Accounts[addr]; ok {
		return a, nil
	}
	// The real chain returns zeros for an unknown address (never not-found).
	return Account{Address: addr}, nil
}

func (f *FakeClient) GetNonce(_ context.Context, addr string) (uint64, error) {
	if f.Err != nil {
		return 0, f.Err
	}
	return f.Nonces[addr], nil
}

func (f *FakeClient) GetValidator(_ context.Context, addr string) (Validator, bool, error) {
	if f.Err != nil {
		return Validator{}, false, f.Err
	}
	v, ok := f.Validators[addr]
	return v, ok, nil
}

func (f *FakeClient) StakingStats(_ context.Context) (StakingStats, error) {
	if f.Err != nil {
		return StakingStats{}, f.Err
	}
	return f.Stats, nil
}

func (f *FakeClient) TokenStats(_ context.Context) (TokenStats, error) {
	if f.Err != nil {
		return TokenStats{}, f.Err
	}
	return f.Tokens, nil
}

func (f *FakeClient) ChainInfo(_ context.Context) (ChainInfo, error) {
	if f.Err != nil {
		return ChainInfo{}, f.Err
	}
	return f.Info, nil
}

func (f *FakeClient) ChainHeight(_ context.Context) (uint64, error) {
	if f.Err != nil {
		return 0, f.Err
	}
	return f.Height, nil
}

func (f *FakeClient) Ping(_ context.Context) error { return f.Err }
