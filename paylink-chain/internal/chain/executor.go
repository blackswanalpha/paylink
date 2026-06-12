package chain

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/events"
	"github.com/paylink/paylink-chain/internal/fee"
	"github.com/paylink/paylink-chain/internal/fsm"
	"github.com/paylink/paylink-chain/internal/rules"
	"github.com/paylink/paylink-chain/internal/slashing"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// TxReceipt records the result of a transaction execution.
type TxReceipt struct {
	TxHash      types.Hash `json:"txHash"`
	Success     bool       `json:"success"`
	Error       string     `json:"error,omitempty"`
	BlockHeight uint64     `json:"blockHeight,omitempty"`
	TxIndex     int        `json:"txIndex"`
}

// Executor processes transactions against the state.
type Executor struct {
	state      *state.StateDB
	eventBus   *events.Bus // nil = no events emitted
	plFSM      *fsm.Machine
	valFSM     *fsm.Machine
	pendingEvt []*events.Event // events emitted by the tx currently executing
	blockEvt   []*events.Event // events from committed txs, awaiting block commit
}

// NewExecutor creates a new transaction executor.
// Pass nil for bus to disable event emission.
func NewExecutor(s *state.StateDB, bus *events.Bus) *Executor {
	return &Executor{
		state:    s,
		eventBus: bus,
		plFSM:    fsm.NewPayLinkFSM(),
		valFSM:   fsm.NewValidatorFSM(),
	}
}

// emit buffers an event for later publishing (on tx success).
func (e *Executor) emit(evt *events.Event) {
	if e.eventBus != nil {
		e.pendingEvt = append(e.pendingEvt, evt)
	}
}

// commitTxEvents moves the current tx's events into the block buffer.
func (e *Executor) commitTxEvents() {
	e.blockEvt = append(e.blockEvt, e.pendingEvt...)
	e.pendingEvt = e.pendingEvt[:0]
}

// discardTxEvents clears the current tx's events without publishing.
func (e *Executor) discardTxEvents() {
	e.pendingEvt = e.pendingEvt[:0]
}

// FlushEvents publishes all events buffered for the current block and clears the buffer.
// Callers invoke this only after the block has been durably committed, so subscribers
// never observe events for state that was rolled back.
func (e *Executor) FlushEvents(blockHeight uint64) {
	if e.eventBus == nil {
		e.blockEvt = e.blockEvt[:0]
		return
	}
	for _, evt := range e.blockEvt {
		evt.BlockHeight = blockHeight
		e.eventBus.Publish(evt)
	}
	e.blockEvt = e.blockEvt[:0]
}

// DiscardEvents clears all buffered events (tx-level and block-level) without publishing.
// Callers invoke this when a block fails to commit and its state has been reverted.
func (e *Executor) DiscardEvents() {
	e.pendingEvt = e.pendingEvt[:0]
	e.blockEvt = e.blockEvt[:0]
}

// evaluateRules parses and evaluates rules attached to a PayLink.
// Returns nil if no rules are attached or all rules pass.
func (e *Executor) evaluateRules(pl *types.PayLink, ctx *rules.EvalContext) error {
	if len(pl.Rules) == 0 || string(pl.Rules) == "null" {
		return nil
	}
	var ruleSet []rules.Rule
	if err := json.Unmarshal(pl.Rules, &ruleSet); err != nil {
		return fmt.Errorf("corrupt rules: %w", err)
	}
	return rules.Evaluate(ruleSet, ctx)
}

// ExecuteTx executes a single transaction and returns a receipt.
// The block timestamp is used for time-dependent operations.
// It does NOT verify the transaction signature — callers must verify first
// (ExecuteBlock does; so do the RPC and P2P admission paths).
func (e *Executor) ExecuteTx(tx *types.Transaction, blockTimestamp int64) TxReceipt {
	receipt := TxReceipt{TxHash: tx.Hash, Success: false}

	// Verify nonce
	expectedNonce := e.state.GetNonce(tx.From)
	if tx.Nonce != expectedNonce {
		receipt.Error = fmt.Sprintf("invalid nonce: expected %d, got %d", expectedNonce, tx.Nonce)
		return receipt
	}

	// Execute based on type
	var err error
	switch tx.Type {
	case types.TxCreatePayLink:
		err = e.executeCreatePayLink(tx, blockTimestamp)
	case types.TxSubmitValidation:
		err = e.executeSubmitValidation(tx, blockTimestamp)
	case types.TxCancelPayLink:
		err = e.executeCancelPayLink(tx, blockTimestamp)
	case types.TxFailPayLink:
		err = e.executeFailPayLink(tx)
	case types.TxTransfer:
		err = e.executeTransfer(tx)
	case types.TxStake:
		err = e.executeStake(tx, blockTimestamp)
	case types.TxInitiateUnstake:
		err = e.executeInitiateUnstake(tx, blockTimestamp)
	case types.TxCompleteUnstake:
		err = e.executeCompleteUnstake(tx, blockTimestamp)
	case types.TxSlash:
		err = e.executeSlash(tx)
	case types.TxDistributeReward:
		err = e.executeDistributeReward(tx)
	case types.TxRegisterVRFKey:
		err = e.executeRegisterVRFKey(tx)
	case types.TxSubmitEvidence:
		err = e.executeSubmitEvidence(tx)
	case types.TxTransferPayLink:
		err = e.executeTransferPayLink(tx, blockTimestamp)
	case types.TxApprovePayLink:
		err = e.executeApprovePayLink(tx)
	case types.TxSetApprovalForAll:
		err = e.executeSetApprovalForAll(tx)
	default:
		err = fmt.Errorf("unknown tx type: %d", tx.Type)
	}

	if err != nil {
		receipt.Error = err.Error()
		return receipt
	}

	// Increment nonce on success
	e.state.IncrementNonce(tx.From)
	receipt.Success = true
	return receipt
}

// ── PayLink Protocol ──

func (e *Executor) executeCreatePayLink(tx *types.Transaction, blockTimestamp int64) error {
	var p types.CreatePayLinkPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if p.Receiver.IsZero() {
		return fmt.Errorf("invalid receiver: zero address")
	}
	if p.Amount == 0 {
		return fmt.Errorf("invalid amount: zero")
	}
	if p.Expiry <= blockTimestamp {
		return fmt.Errorf("invalid expiry: already expired")
	}

	// Validate rules if present
	if len(p.Rules) > 0 && string(p.Rules) != "null" {
		var ruleSet []rules.Rule
		if err := json.Unmarshal(p.Rules, &ruleSet); err != nil {
			return fmt.Errorf("invalid rules: %w", err)
		}
		if err := rules.ValidateRules(ruleSet); err != nil {
			return fmt.Errorf("invalid rules: %w", err)
		}
	}

	pl := &types.PayLink{
		ID:           p.PayLinkID,
		Creator:      tx.From,
		Receiver:     p.Receiver,
		Owner:        tx.From,
		Amount:       p.Amount,
		Expiry:       p.Expiry,
		Status:       types.StatusCreated,
		MetadataHash: p.MetadataHash,
		CreatedAt:    blockTimestamp,
		Rules:        p.Rules,
	}

	if err := e.state.CreatePayLink(pl); err != nil {
		return err
	}

	e.emit(events.NewEvent(events.EventPayLinkCreated, events.EntityPayLink, pl.ID.Hex(), 0).
		WithTransition(fsm.PayLinkNone, fsm.PayLinkCreated, fsm.PayLinkCreate).
		WithTx(tx.Hash.Hex()).
		WithData(events.PayLinkCreatedData{
			Creator:      tx.From.Hex(),
			Receiver:     p.Receiver.Hex(),
			Amount:       p.Amount,
			Expiry:       p.Expiry,
			MetadataHash: p.MetadataHash.Hex(),
		}))

	return nil
}

func (e *Executor) executeSubmitValidation(tx *types.Transaction, blockTimestamp int64) error {
	var p types.SubmitValidationPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	// Check validator is active
	if !e.state.IsActiveValidator(tx.From) {
		return fmt.Errorf("not an active validator: %s", tx.From)
	}

	// Check proof not already used globally
	if e.state.IsProofUsed(p.ProofHash) {
		return fmt.Errorf("proof already used: %s", p.ProofHash)
	}

	// Check paylink exists and is in CREATED status
	pl := e.state.GetPayLink(p.PayLinkID)
	if pl == nil {
		return fmt.Errorf("paylink not found: %s", p.PayLinkID)
	}
	if pl.Status != types.StatusCreated {
		return fmt.Errorf("paylink not in CREATED status: %s (status: %s)", p.PayLinkID, pl.Status)
	}

	// Check not expired
	if blockTimestamp > pl.Expiry {
		return fmt.Errorf("paylink expired: %s", p.PayLinkID)
	}

	// Check no double vote
	if e.state.HasVoted(p.PayLinkID, tx.From) {
		return fmt.Errorf("already voted: %s by %s", p.PayLinkID, tx.From)
	}

	// Check proof hash consistency (all validators must submit same proof)
	if existing, ok := e.state.GetSubmittedProof(p.PayLinkID); ok {
		if existing != p.ProofHash {
			return fmt.Errorf("proof hash mismatch: expected %s, got %s", existing, p.ProofHash)
		}
	} else {
		e.state.SetSubmittedProof(p.PayLinkID, p.ProofHash)
	}

	// Record vote
	e.state.RecordVote(p.PayLinkID, tx.From)

	// Increment vote count
	voteCount, err := e.state.IncrementVoteCount(p.PayLinkID)
	if err != nil {
		return err
	}

	required := e.state.RequiredValidations()

	// Emit vote event
	e.emit(events.NewEvent(events.EventPayLinkVoted, events.EntityPayLink, p.PayLinkID.Hex(), 0).
		WithTx(tx.Hash.Hex()).
		WithData(events.PayLinkVotedData{
			Validator: tx.From.Hex(),
			ProofHash: p.ProofHash.Hex(),
			VoteCount: voteCount,
			Required:  required,
		}))

	// Check quorum
	if voteCount >= required {
		// Evaluate rules before settlement
		voters := e.state.GetVotersForPayLink(p.PayLinkID)
		if err := e.evaluateRules(pl, &rules.EvalContext{
			Action:         rules.ActionSettle,
			BlockTimestamp: blockTimestamp,
			Sender:         tx.From,
			PayLinkOwner:   pl.Owner,
			PayLinkCreator: pl.Creator,
			Amount:         pl.Amount,
			Approvals:      voters,
		}); err != nil {
			return fmt.Errorf("rules rejected settlement: %w", err)
		}

		// Settle: mark as VERIFIED and record proof as used
		if err := e.state.SetPayLinkStatus(p.PayLinkID, types.StatusVerified); err != nil {
			return err
		}
		e.state.MarkProofUsed(p.ProofHash)

		e.emit(events.NewEvent(events.EventPayLinkVerified, events.EntityPayLink, p.PayLinkID.Hex(), 0).
			WithTransition(fsm.PayLinkCreated, fsm.PayLinkVerified, fsm.PayLinkSettle).
			WithTx(tx.Hash.Hex()).
			WithData(events.PayLinkSettledData{
				ProofHash: p.ProofHash.Hex(),
				VoteCount: voteCount,
			}))

		// Phase 2: Collect and distribute fees via PLN inflation
		e.collectAndDistributeFees(p.PayLinkID, pl.Amount, tx.Hash)
	}

	return nil
}

func (e *Executor) executeCancelPayLink(tx *types.Transaction, blockTimestamp int64) error {
	var p types.CancelPayLinkPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	pl := e.state.GetPayLink(p.PayLinkID)
	if pl == nil {
		return fmt.Errorf("paylink not found: %s", p.PayLinkID)
	}
	if pl.Status != types.StatusCreated {
		return fmt.Errorf("paylink not in CREATED status: %s (status: %s)", p.PayLinkID, pl.Status)
	}
	// Allow owner or creator to cancel
	if pl.Creator != tx.From && pl.Owner != tx.From {
		return fmt.Errorf("not creator or owner: %s", tx.From)
	}

	// Evaluate rules before cancellation
	if err := e.evaluateRules(pl, &rules.EvalContext{
		Action:         rules.ActionCancel,
		BlockTimestamp: blockTimestamp,
		Sender:         tx.From,
		PayLinkOwner:   pl.Owner,
		PayLinkCreator: pl.Creator,
		Amount:         pl.Amount,
	}); err != nil {
		return fmt.Errorf("rules rejected cancellation: %w", err)
	}

	if err := e.state.SetPayLinkStatus(p.PayLinkID, types.StatusCancelled); err != nil {
		return err
	}

	e.emit(events.NewEvent(events.EventPayLinkCancelled, events.EntityPayLink, p.PayLinkID.Hex(), 0).
		WithTransition(fsm.PayLinkCreated, fsm.PayLinkCancelled, fsm.PayLinkCancel).
		WithTx(tx.Hash.Hex()))

	return nil
}

func (e *Executor) executeFailPayLink(tx *types.Transaction) error {
	// Admin only
	if tx.From != e.state.AdminAddress() {
		return fmt.Errorf("not admin: %s", tx.From)
	}

	var p types.FailPayLinkPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	pl := e.state.GetPayLink(p.PayLinkID)
	if pl == nil {
		return fmt.Errorf("paylink not found: %s", p.PayLinkID)
	}

	fromState := statusToFSMState(pl.Status)

	if err := e.state.SetPayLinkStatus(p.PayLinkID, types.StatusFailed); err != nil {
		return err
	}

	e.emit(events.NewEvent(events.EventPayLinkFailed, events.EntityPayLink, p.PayLinkID.Hex(), 0).
		WithTransition(fromState, fsm.PayLinkFailed, fsm.PayLinkFail).
		WithTx(tx.Hash.Hex()))

	return nil
}

// ── NFT-style PayLink Ownership ──

func (e *Executor) executeTransferPayLink(tx *types.Transaction, blockTimestamp int64) error {
	var p types.TransferPayLinkPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if p.To.IsZero() {
		return fmt.Errorf("cannot transfer to zero address")
	}

	pl := e.state.GetPayLink(p.PayLinkID)
	if pl == nil {
		return fmt.Errorf("paylink not found: %s", p.PayLinkID)
	}
	if pl.Status != types.StatusCreated {
		return fmt.Errorf("paylink not in CREATED status: %s (status: %s)", p.PayLinkID, pl.Status)
	}
	if p.To == pl.Owner {
		return fmt.Errorf("cannot transfer to current owner")
	}

	// Check authorization: owner, approved, or operator
	if !e.state.IsApprovedOrOwner(p.PayLinkID, tx.From) {
		return fmt.Errorf("not authorized to transfer: %s", tx.From)
	}

	// Evaluate rules before transfer
	if err := e.evaluateRules(pl, &rules.EvalContext{
		Action:         rules.ActionTransfer,
		BlockTimestamp: blockTimestamp,
		Sender:         tx.From,
		PayLinkOwner:   pl.Owner,
		PayLinkCreator: pl.Creator,
		Receiver:       p.To,
		Amount:         pl.Amount,
		TransferCount:  pl.TransferCount,
	}); err != nil {
		return fmt.Errorf("rules rejected transfer: %w", err)
	}

	previousOwner := pl.Owner

	if err := e.state.SetPayLinkOwner(p.PayLinkID, p.To); err != nil {
		return err
	}

	// Determine operator field for event
	operator := ""
	if tx.From != previousOwner {
		operator = tx.From.Hex()
	}

	e.emit(events.NewEvent(events.EventPayLinkTransferred, events.EntityPayLink, p.PayLinkID.Hex(), 0).
		WithTransition(fsm.PayLinkCreated, fsm.PayLinkCreated, fsm.PayLinkTransfer).
		WithTx(tx.Hash.Hex()).
		WithData(events.PayLinkTransferredData{
			From:          previousOwner.Hex(),
			To:            p.To.Hex(),
			Operator:      operator,
			TransferCount: pl.TransferCount + 1,
		}))

	return nil
}

func (e *Executor) executeApprovePayLink(tx *types.Transaction) error {
	var p types.ApprovePayLinkPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	pl := e.state.GetPayLink(p.PayLinkID)
	if pl == nil {
		return fmt.Errorf("paylink not found: %s", p.PayLinkID)
	}
	// Only the owner can set approval (not operators)
	if tx.From != pl.Owner {
		return fmt.Errorf("not owner: %s (owner: %s)", tx.From, pl.Owner)
	}
	if p.Approved == pl.Owner {
		return fmt.Errorf("cannot approve self")
	}

	if err := e.state.SetPayLinkApproval(p.PayLinkID, p.Approved); err != nil {
		return err
	}

	e.emit(events.NewEvent(events.EventPayLinkApproved, events.EntityPayLink, p.PayLinkID.Hex(), 0).
		WithTx(tx.Hash.Hex()).
		WithData(events.PayLinkApprovedData{
			Owner:    pl.Owner.Hex(),
			Approved: p.Approved.Hex(),
		}))

	return nil
}

func (e *Executor) executeSetApprovalForAll(tx *types.Transaction) error {
	var p types.SetApprovalForAllPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if p.Operator == tx.From {
		return fmt.Errorf("cannot set self as operator")
	}
	if p.Operator.IsZero() {
		return fmt.Errorf("invalid operator: zero address")
	}

	e.state.SetOperatorApproval(tx.From, p.Operator, p.Approved)

	e.emit(events.NewEvent(events.EventPayLinkApprovalForAll, events.EntityPayLink, tx.From.Hex(), 0).
		WithTx(tx.Hash.Hex()).
		WithData(events.PayLinkApprovalForAllData{
			Owner:    tx.From.Hex(),
			Operator: p.Operator.Hex(),
			Approved: p.Approved,
		}))

	return nil
}

// ── PLN Token Transfer ──

func (e *Executor) executeTransfer(tx *types.Transaction) error {
	var p types.TransferPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if p.Amount == 0 {
		return fmt.Errorf("invalid amount: zero")
	}
	if p.To.IsZero() {
		return fmt.Errorf("invalid recipient: zero address")
	}

	if err := e.state.Transfer(tx.From, p.To, p.Amount); err != nil {
		return err
	}

	e.emit(events.NewEvent(events.EventTransfer, events.EntityAccount, tx.From.Hex(), 0).
		WithTx(tx.Hash.Hex()).
		WithData(events.TransferData{
			From:   tx.From.Hex(),
			To:     p.To.Hex(),
			Amount: p.Amount,
		}))

	return nil
}

// ── Validator Staking ──

func (e *Executor) executeStake(tx *types.Transaction, blockTimestamp int64) error {
	var p types.StakePayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if p.Amount == 0 {
		return fmt.Errorf("zero amount")
	}

	// Check state before
	wasBefore := e.state.GetValidator(tx.From)
	wasActive := wasBefore != nil && wasBefore.IsActive

	// Debit balance
	if err := e.state.SubBalance(tx.From, p.Amount); err != nil {
		return fmt.Errorf("insufficient balance for stake: %w", err)
	}

	// Credit stake
	if err := e.state.Stake(tx.From, p.Amount, blockTimestamp); err != nil {
		return err
	}

	// Check state after
	vAfter := e.state.GetValidator(tx.From)

	e.emit(events.NewEvent(events.EventValidatorStaked, events.EntityValidator, tx.From.Hex(), 0).
		WithTx(tx.Hash.Hex()).
		WithData(events.ValidatorStakeData{
			Amount:      p.Amount,
			TotalStaked: vAfter.StakedAmount,
			IsActive:    vAfter.IsActive,
		}))

	// Emit activation event if transitioned to active
	if !wasActive && vAfter.IsActive {
		e.emit(events.NewEvent(events.EventValidatorActivated, events.EntityValidator, tx.From.Hex(), 0).
			WithTransition(fsm.ValidatorInactive, fsm.ValidatorActive, fsm.ValidatorActivate).
			WithTx(tx.Hash.Hex()))
	}

	return nil
}

func (e *Executor) executeInitiateUnstake(tx *types.Transaction, blockTimestamp int64) error {
	var p types.InitiateUnstakePayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if p.Amount == 0 {
		return fmt.Errorf("zero amount")
	}

	wasActive := e.state.IsActiveValidator(tx.From)

	if err := e.state.InitiateWithdrawal(tx.From, p.Amount, blockTimestamp); err != nil {
		return err
	}

	vAfter := e.state.GetValidator(tx.From)

	e.emit(events.NewEvent(events.EventValidatorUnstakeStarted, events.EntityValidator, tx.From.Hex(), 0).
		WithTransition(fsm.ValidatorActive, fsm.ValidatorPendingWithdraw, fsm.ValidatorInitUnstake).
		WithTx(tx.Hash.Hex()).
		WithData(events.ValidatorUnstakeData{
			Amount:         p.Amount,
			WithdrawableAt: vAfter.WithdrawableAt,
		}))

	// Emit deactivation if deactivated
	if wasActive && !vAfter.IsActive {
		e.emit(events.NewEvent(events.EventValidatorDeactivated, events.EntityValidator, tx.From.Hex(), 0).
			WithTransition(fsm.ValidatorActive, fsm.ValidatorInactive, fsm.ValidatorDeactivate).
			WithTx(tx.Hash.Hex()))
	}

	return nil
}

func (e *Executor) executeCompleteUnstake(tx *types.Transaction, blockTimestamp int64) error {
	amount, err := e.state.CompleteWithdrawal(tx.From, blockTimestamp)
	if err != nil {
		return err
	}

	// Credit balance with withdrawn amount
	e.state.AddBalance(tx.From, amount)

	e.emit(events.NewEvent(events.EventValidatorUnstakeComplete, events.EntityValidator, tx.From.Hex(), 0).
		WithTransition(fsm.ValidatorPendingWithdraw, fsm.ValidatorNonExistent, fsm.ValidatorCompleteUnstake).
		WithTx(tx.Hash.Hex()).
		WithData(events.ValidatorUnstakeData{
			Amount: amount,
		}))

	return nil
}

func (e *Executor) executeSlash(tx *types.Transaction) error {
	// Admin only
	if tx.From != e.state.AdminAddress() {
		return fmt.Errorf("not admin: %s", tx.From)
	}

	var p types.SlashPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	wasActive := e.state.IsActiveValidator(p.Validator)

	if err := e.state.Slash(p.Validator, p.Amount); err != nil {
		return err
	}

	vAfter := e.state.GetValidator(p.Validator)

	e.emit(events.NewEvent(events.EventValidatorSlashed, events.EntityValidator, p.Validator.Hex(), 0).
		WithTransition(fsm.ValidatorActive, fsm.ValidatorSlashed, fsm.ValidatorSlash).
		WithTx(tx.Hash.Hex()).
		WithData(events.ValidatorSlashData{
			Amount:    p.Amount,
			Reason:    p.Reason,
			Remaining: vAfter.StakedAmount,
		}))

	// Emit deactivation if deactivated
	if wasActive && !vAfter.IsActive {
		e.emit(events.NewEvent(events.EventValidatorDeactivated, events.EntityValidator, p.Validator.Hex(), 0).
			WithTransition(fsm.ValidatorActive, fsm.ValidatorInactive, fsm.ValidatorDeactivate).
			WithTx(tx.Hash.Hex()))
	}

	return nil
}

func (e *Executor) executeDistributeReward(tx *types.Transaction) error {
	// Admin only
	if tx.From != e.state.AdminAddress() {
		return fmt.Errorf("not admin: %s", tx.From)
	}

	var p types.DistributeRewardPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	// Debit from admin balance
	if err := e.state.SubBalance(tx.From, p.Amount); err != nil {
		return fmt.Errorf("insufficient balance for reward: %w", err)
	}

	// Credit to validator balance
	e.state.AddBalance(p.Validator, p.Amount)

	// Record in validator info
	if err := e.state.DistributeReward(p.Validator, p.Amount); err != nil {
		return err
	}

	vAfter := e.state.GetValidator(p.Validator)

	e.emit(events.NewEvent(events.EventValidatorRewarded, events.EntityValidator, p.Validator.Hex(), 0).
		WithTransition(fsm.ValidatorActive, fsm.ValidatorActive, fsm.ValidatorReward).
		WithTx(tx.Hash.Hex()).
		WithData(events.ValidatorRewardData{
			Amount:       p.Amount,
			TotalRewards: vAfter.TotalRewards,
		}))

	return nil
}

// ── Fee Collection (Phase 2) ──

// collectAndDistributeFees calculates fees for a settled PayLink and distributes
// them via PLN inflation: 70% minted to validators, 20% to treasury, 10% burned.
func (e *Executor) collectAndDistributeFees(plID types.Hash, amount uint64, txHash types.Hash) {
	calc := fee.NewCalculator(
		e.state.FeeRateBasisPoints(),
		e.state.ValidatorRewardShare(),
		e.state.TreasurySharePct(),
		e.state.BurnShare(),
		0, // no minimum fee floor for now
	)

	fb := calc.CalculateFee(amount)
	if fb.TotalFee == 0 {
		return
	}

	// Get voters for this PayLink
	voters := e.state.GetVoters(plID)
	if len(voters) == 0 {
		return
	}

	dist := fee.NewDistributor(e.state, e.state.TreasuryAddress())
	payouts, err := dist.DistributeFees(fb, voters)
	if err != nil {
		// Fee distribution failure is non-fatal -- the settlement stands, but the
		// missed payout must be visible to operators.
		log.Printf("fee distribution failed for paylink %s (fee %d): %v", plID, fb.TotalFee, err)
		return
	}

	// Emit fee collected event
	e.emit(events.NewEvent(events.EventFeeCollected, events.EntityPayLink, plID.Hex(), 0).
		WithTx(txHash.Hex()).
		WithData(events.FeeCollectedData{
			PayLinkID:      plID.Hex(),
			Amount:         amount,
			TotalFee:       fb.TotalFee,
			ValidatorShare: fb.ValidatorReward,
			TreasuryShare:  fb.TreasuryAmount,
			BurnAmount:     fb.BurnAmount,
		}))

	// Emit per-validator reward events
	for _, p := range payouts {
		e.emit(events.NewEvent(events.EventFeeDistributed, events.EntityValidator, p.Validator.Hex(), 0).
			WithTx(txHash.Hex()).
			WithData(events.FeeDistributedData{
				Validator: p.Validator.Hex(),
				Amount:    p.Amount,
			}))
	}

	// Emit burn event
	if fb.BurnAmount > 0 {
		e.emit(events.NewEvent(events.EventTokenBurned, events.EntityAccount, "burn", 0).
			WithTx(txHash.Hex()).
			WithData(events.TokenBurnedData{
				Amount:      fb.BurnAmount,
				TotalBurned: e.state.TotalBurned(),
			}))
	}
}

// ── VRF Key Registration (Phase 2) ──

func (e *Executor) executeRegisterVRFKey(tx *types.Transaction) error {
	var p types.RegisterVRFKeyPayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if len(p.VRFPublicKey) != 32 {
		return fmt.Errorf("invalid VRF public key length: got %d, want 32", len(p.VRFPublicKey))
	}

	if !e.state.IsActiveValidator(tx.From) {
		return fmt.Errorf("not an active validator: %s", tx.From)
	}

	if err := e.state.SetVRFPublicKey(tx.From, p.VRFPublicKey); err != nil {
		return err
	}

	e.emit(events.NewEvent(events.EventValidatorVRFKeyRegistered, events.EntityValidator, tx.From.Hex(), 0).
		WithTx(tx.Hash.Hex()))

	return nil
}

// ── Slashing Evidence (Phase 2) ──

func (e *Executor) executeSubmitEvidence(tx *types.Transaction) error {
	var p types.SubmitEvidencePayload
	if err := json.Unmarshal(tx.Payload, &p); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	detector := slashing.NewSlashingDetector(e.state)
	action, err := detector.ProcessEvidence(p.EvidenceType, p.Validator, p.Data)
	if err != nil {
		return fmt.Errorf("invalid evidence: %w", err)
	}

	// Anti-replay: one slash per offense, no matter how often (or by whom) the
	// same evidence is resubmitted.
	if e.state.IsEvidenceProcessed(action.EvidenceID) {
		return fmt.Errorf("evidence already processed: %s", action.EvidenceID)
	}
	e.state.MarkEvidenceProcessed(action.EvidenceID)

	// Execute the slash
	wasActive := e.state.IsActiveValidator(action.Validator)
	if err := e.state.Slash(action.Validator, action.Amount); err != nil {
		return fmt.Errorf("slash failed: %w", err)
	}

	vAfter := e.state.GetValidator(action.Validator)

	e.emit(events.NewEvent(events.EventValidatorSlashed, events.EntityValidator, action.Validator.Hex(), 0).
		WithTransition(fsm.ValidatorActive, fsm.ValidatorSlashed, fsm.ValidatorSlash).
		WithTx(tx.Hash.Hex()).
		WithData(events.ValidatorSlashData{
			Amount:    action.Amount,
			Reason:    action.Reason,
			Remaining: vAfter.StakedAmount,
		}))

	if wasActive && !vAfter.IsActive {
		e.emit(events.NewEvent(events.EventValidatorDeactivated, events.EntityValidator, action.Validator.Hex(), 0).
			WithTransition(fsm.ValidatorActive, fsm.ValidatorInactive, fsm.ValidatorDeactivate).
			WithTx(tx.Hash.Hex()))
	}

	return nil
}

// ExecuteBlock executes all transactions in a block, returning receipts.
// Invalid transactions are skipped (their receipts show errors).
// Every transaction must carry a valid signature (pubkey bound to From); unsigned or
// forged transactions fail without touching state. Events are buffered per block —
// callers must FlushEvents after the block commits (or DiscardEvents on failure).
func (e *Executor) ExecuteBlock(txs []types.Transaction, blockTimestamp int64, blockHeight uint64) []TxReceipt {
	receipts := make([]TxReceipt, len(txs))
	for i := range txs {
		if err := crypto.VerifyTx(&txs[i]); err != nil {
			receipts[i] = TxReceipt{TxHash: txs[i].Hash, Error: "invalid signature: " + err.Error()}
			continue
		}
		snapID := e.state.Snapshot()
		e.discardTxEvents()
		receipt := e.ExecuteTx(&txs[i], blockTimestamp)
		if !receipt.Success {
			// Revert state on failed tx
			_ = e.state.Revert(snapID)
			e.discardTxEvents()
		} else {
			e.state.DiscardSnapshot(snapID)
			e.commitTxEvents()
		}
		receipts[i] = receipt
	}
	return receipts
}

// statusToFSMState converts a types.Status to its FSM State equivalent.
func statusToFSMState(s types.Status) fsm.State {
	switch s {
	case types.StatusCreated:
		return fsm.PayLinkCreated
	case types.StatusVerified:
		return fsm.PayLinkVerified
	case types.StatusFailed:
		return fsm.PayLinkFailed
	case types.StatusCancelled:
		return fsm.PayLinkCancelled
	default:
		return fsm.PayLinkNone
	}
}
