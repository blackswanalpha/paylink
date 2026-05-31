// Package autostake is the DEVNET-ONLY bootstrap that makes the validator's signer an active
// on-chain validator, so a single-validator devnet can reach quorum (RequiredValidations=1) and
// settle PayLinks. It is gated by PROOF_VALIDATOR_AUTO_STAKE; in production the validator is staked
// out-of-band and this is disabled. EnsureActive is idempotent (no-op if already active).
package autostake

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/paylink/paylink-chain/pkg/lvm"
	"github.com/paylink/proof-validator/internal/chain"
)

// StakeChain is the chain surface the bootstrap needs (satisfied by *chain.Client).
type StakeChain interface {
	GetValidator(ctx context.Context, address string) (*chain.ValidatorState, bool, error)
	GetAccount(ctx context.Context, address string) (*chain.AccountState, error)
	StakingStats(ctx context.Context) (*chain.StakingStats, error)
	SendTransaction(ctx context.Context, tx *lvm.Transaction) (string, error)
}

// Signer signs the stake transaction and exposes the validator address.
type Signer interface {
	Address() lvm.Address
	SignTx(tx *lvm.Transaction) error
}

// NonceReserver hands out the next nonce (satisfied by *chain.NonceManager).
type NonceReserver interface {
	Reserve(ctx context.Context, address string) (uint64, func(bool), error)
}

// Bootstrapper performs the one-time auto-stake.
type Bootstrapper struct {
	chain   StakeChain
	signer  Signer
	nonce   NonceReserver
	log     *slog.Logger
	amount  uint64        // 0 = use chain minimumStake
	poll    time.Duration // poll interval while waiting to become active
	timeout time.Duration // give up after this long
}

// New builds a Bootstrapper.
func New(c StakeChain, s Signer, n NonceReserver, log *slog.Logger, amount uint64, poll, timeout time.Duration) *Bootstrapper {
	if log == nil {
		log = slog.Default()
	}
	if poll <= 0 {
		poll = time.Second
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Bootstrapper{chain: c, signer: s, nonce: n, log: log, amount: amount, poll: poll, timeout: timeout}
}

// EnsureActive makes the signer an active validator if it is not already. It is idempotent: if the
// validator is already active it returns immediately. Otherwise it submits a TxStake and blocks
// until the validator becomes active or the timeout elapses.
func (b *Bootstrapper) EnsureActive(ctx context.Context) error {
	addr := b.signer.Address().Hex()

	if v, found, err := b.chain.GetValidator(ctx, addr); err != nil {
		return fmt.Errorf("check validator: %w", err)
	} else if found && v.IsActive {
		b.log.Info("validator already active", "address", addr, "staked", v.StakedAmount)
		return nil
	}

	amount := b.amount
	if amount == 0 {
		stats, err := b.chain.StakingStats(ctx)
		if err != nil {
			return fmt.Errorf("staking stats: %w", err)
		}
		amount = stats.MinimumStake
	}

	acc, err := b.chain.GetAccount(ctx, addr)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}
	if acc.Balance < amount {
		return fmt.Errorf("insufficient balance to auto-stake: have %d, need %d (fund %s in the devnet genesis)", acc.Balance, amount, addr)
	}

	if err := b.submitStake(ctx, addr, amount); err != nil {
		return err
	}
	b.log.Info("stake tx broadcast; waiting for validator to become active", "address", addr, "amount", amount)
	return b.waitActive(ctx, addr)
}

func (b *Bootstrapper) submitStake(ctx context.Context, addr string, amount uint64) error {
	nonce, commit, err := b.nonce.Reserve(ctx, addr)
	if err != nil {
		return fmt.Errorf("reserve nonce: %w", err)
	}
	tx, err := lvm.BuildStakeTx(b.signer.Address(), nonce, amount)
	if err != nil {
		commit(false)
		return fmt.Errorf("build stake tx: %w", err)
	}
	if err := b.signer.SignTx(tx); err != nil {
		commit(false)
		return fmt.Errorf("sign stake tx: %w", err)
	}
	_, sendErr := b.chain.SendTransaction(ctx, tx)
	commit(sendErr == nil)
	if sendErr != nil {
		return fmt.Errorf("broadcast stake tx: %w", sendErr)
	}
	return nil
}

func (b *Bootstrapper) waitActive(ctx context.Context, addr string) error {
	deadline := time.NewTimer(b.timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(b.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("validator %s did not become active within %s of staking", addr, b.timeout)
		case <-ticker.C:
			if v, found, err := b.chain.GetValidator(ctx, addr); err == nil && found && v.IsActive {
				b.log.Info("validator active", "address", addr)
				return nil
			}
		}
	}
}
