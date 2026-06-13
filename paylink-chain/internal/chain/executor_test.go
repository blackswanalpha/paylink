package chain

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/events"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

func testGenesis() *types.GenesisConfig {
	return testGenesisWithAdmin(types.HexToAddress("0x0000000000000000000000000000000000000001"))
}

func testGenesisWithAdmin(admin types.Address) *types.GenesisConfig {
	return &types.GenesisConfig{
		ChainID:             "test-chain",
		AdminAddress:        admin,
		InitialSupply:       100_000_000,
		MaxSupply:           1_000_000_000,
		MinimumStake:        10_000,
		WithdrawalCooldown:  7 * 24 * 3600,
		RequiredValidations: 3,
		BlockIntervalMs:     1000,
		InitialBalances: []types.GenesisBalance{
			{Address: admin, Balance: 100_000_000},
		},
	}
}

func makeTx(txType types.TxType, from types.Address, nonce uint64, payload interface{}) *types.Transaction {
	data, _ := json.Marshal(payload)
	tx := &types.Transaction{
		Type:    txType,
		From:    from,
		Nonce:   nonce,
		Payload: data,
	}
	tx.Hash = crypto.SHA256Hash(tx.SignableBytes())
	return tx
}

// genTestKey generates a P-256 key for a test actor and returns it with its derived address.
func genTestKey(t *testing.T) (*ecdsa.PrivateKey, types.Address) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key, crypto.PrivateKeyToAddress(key)
}

// makeSignedTx builds a fully signed transaction (PubKey + Hash + Signature) from key.
// Required for ExecuteBlock tests, which enforce crypto.VerifyTx.
func makeSignedTx(t *testing.T, txType types.TxType, key *ecdsa.PrivateKey, nonce uint64, payload interface{}) *types.Transaction {
	t.Helper()
	tx := makeTx(txType, crypto.PrivateKeyToAddress(key), nonce, payload)
	tx.PubKey = crypto.MarshalPublicKey(&key.PublicKey)
	sig, err := crypto.Sign(tx.Hash, key)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	tx.Signature = sig
	return tx
}

func TestExecuteTransfer(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	admin := genesis.AdminAddress
	bob := types.HexToAddress("0x0000000000000000000000000000000000000002")

	// Transfer 1000 from admin to bob
	tx := makeTx(types.TxTransfer, admin, 0, types.TransferPayload{
		To:     bob,
		Amount: 1000,
	})

	receipt := exec.ExecuteTx(tx, time.Now().Unix())
	if !receipt.Success {
		t.Fatalf("Transfer failed: %s", receipt.Error)
	}

	if s.GetBalance(bob) != 1000 {
		t.Fatalf("Bob balance: expected 1000, got %d", s.GetBalance(bob))
	}
	if s.GetBalance(admin) != 100_000_000-1000 {
		t.Fatalf("Admin balance: expected %d, got %d", 100_000_000-1000, s.GetBalance(admin))
	}
	if s.GetNonce(admin) != 1 {
		t.Fatalf("Admin nonce: expected 1, got %d", s.GetNonce(admin))
	}
}

func TestExecuteTransferInsufficientBalance(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000002")

	// Bob has no balance
	tx := makeTx(types.TxTransfer, bob, 0, types.TransferPayload{
		To:     genesis.AdminAddress,
		Amount: 1000,
	})

	receipt := exec.ExecuteTx(tx, time.Now().Unix())
	if receipt.Success {
		t.Fatal("Transfer should fail with insufficient balance")
	}
}

func TestExecuteTransferInvalidNonce(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000002")

	// Wrong nonce
	tx := makeTx(types.TxTransfer, genesis.AdminAddress, 5, types.TransferPayload{
		To:     bob,
		Amount: 1000,
	})

	receipt := exec.ExecuteTx(tx, time.Now().Unix())
	if receipt.Success {
		t.Fatal("Transfer should fail with wrong nonce")
	}
}

func TestExecuteCreatePayLink(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	tx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID:    plID,
		Receiver:     receiver,
		Amount:       1500,
		Expiry:       now + 86400,
		MetadataHash: crypto.SHA256Hash([]byte("orderId:001")),
	})

	receipt := exec.ExecuteTx(tx, now)
	if !receipt.Success {
		t.Fatalf("CreatePayLink failed: %s", receipt.Error)
	}

	pl := s.GetPayLink(plID)
	if pl == nil {
		t.Fatal("PayLink not found")
	}
	if pl.Status != types.StatusCreated {
		t.Fatalf("Expected CREATED, got %s", pl.Status)
	}
	if pl.Amount != 1500 {
		t.Fatalf("Expected amount 1500, got %d", pl.Amount)
	}
	if pl.Creator != merchant {
		t.Fatal("Creator mismatch")
	}
}

func TestExecuteCreatePayLinkDuplicate(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	payload := types.CreatePayLinkPayload{
		PayLinkID: plID,
		Receiver:  receiver,
		Amount:    1500,
		Expiry:    now + 86400,
	}

	tx1 := makeTx(types.TxCreatePayLink, merchant, 0, payload)
	exec.ExecuteTx(tx1, now)

	tx2 := makeTx(types.TxCreatePayLink, merchant, 1, payload)
	receipt := exec.ExecuteTx(tx2, now)
	if receipt.Success {
		t.Fatal("Duplicate PayLink creation should fail")
	}
}

func TestExecuteSubmitValidationAndSettlement(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 3
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	// Setup: create paylink
	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	// Fund validators and stake them
	validators := []types.Address{
		types.HexToAddress("0x0000000000000000000000000000000000000010"),
		types.HexToAddress("0x0000000000000000000000000000000000000011"),
		types.HexToAddress("0x0000000000000000000000000000000000000012"),
	}
	for _, v := range validators {
		s.SetBalance(v, 50_000)
		s.Stake(v, 20_000, now)
	}

	// Create paylink
	createTx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID,
		Receiver:  receiver,
		Amount:    1500,
		Expiry:    now + 86400,
	})
	receipt := exec.ExecuteTx(createTx, now)
	if !receipt.Success {
		t.Fatalf("CreatePayLink failed: %s", receipt.Error)
	}

	// Validators submit validations
	proofHash := crypto.SHA256Hash([]byte("proof-001"))

	for i, v := range validators {
		tx := makeTx(types.TxSubmitValidation, v, 0, types.SubmitValidationPayload{
			PayLinkID: plID,
			ProofHash: proofHash,
		})
		r := exec.ExecuteTx(tx, now)
		if !r.Success {
			t.Fatalf("Validation %d failed: %s", i, r.Error)
		}
	}

	// Check settled
	pl := s.GetPayLink(plID)
	if pl.Status != types.StatusVerified {
		t.Fatalf("Expected VERIFIED, got %s", pl.Status)
	}

	// Proof should be used
	if !s.IsProofUsed(proofHash) {
		t.Fatal("Proof should be marked as used")
	}
}

// TestSettlementEventCarriesPayeeAndAmount verifies the enrichment of the paylink.verified
// event (work23): its data must carry the PayLink's Receiver (payee) and gross Amount so the
// off-chain settlement-service can aggregate verified PayLinks into a merchant settlement without
// a separate lookup. The fields are observability metadata only — never part of consensus.
func TestSettlementEventCarriesPayeeAndAmount(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 3
	s := state.NewStateDB(genesis)

	bus := events.NewBus(events.DefaultBusConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Start(ctx)
	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	exec := NewExecutor(s, bus)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-pay"))
	now := time.Now().Unix()

	validators := []types.Address{
		types.HexToAddress("0x0000000000000000000000000000000000000010"),
		types.HexToAddress("0x0000000000000000000000000000000000000011"),
		types.HexToAddress("0x0000000000000000000000000000000000000012"),
	}
	for _, v := range validators {
		s.SetBalance(v, 50_000)
		s.Stake(v, 20_000, now)
	}

	const grossAmount uint64 = 1500
	createTx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: grossAmount, Expiry: now + 86400,
	})
	if r := exec.ExecuteTx(createTx, now); !r.Success {
		t.Fatalf("CreatePayLink failed: %s", r.Error)
	}
	exec.commitTxEvents()

	proofHash := crypto.SHA256Hash([]byte("proof-pay"))
	for i, v := range validators {
		tx := makeTx(types.TxSubmitValidation, v, 0, types.SubmitValidationPayload{
			PayLinkID: plID, ProofHash: proofHash,
		})
		if r := exec.ExecuteTx(tx, now); !r.Success {
			t.Fatalf("Validation %d failed: %s", i, r.Error)
		}
		exec.commitTxEvents()
	}
	exec.FlushEvents(7)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case evt := <-sub.Ch():
			if evt.Kind != events.EventPayLinkVerified {
				continue
			}
			var d events.PayLinkSettledData
			if err := json.Unmarshal(evt.Data, &d); err != nil {
				t.Fatalf("decode settled data: %v", err)
			}
			if d.Payee != receiver.Hex() {
				t.Fatalf("payee = %q, want %q", d.Payee, receiver.Hex())
			}
			if d.Amount != grossAmount {
				t.Fatalf("amount = %d, want %d", d.Amount, grossAmount)
			}
			return
		case <-deadline:
			t.Fatal("did not observe a paylink.verified event")
		}
	}
}

func TestExecuteValidationDoubleVote(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 3
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	s.SetBalance(validator, 50_000)
	s.Stake(validator, 20_000, now)

	createTx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500, Expiry: now + 86400,
	})
	exec.ExecuteTx(createTx, now)

	proofHash := crypto.SHA256Hash([]byte("proof-001"))

	// First vote
	tx1 := makeTx(types.TxSubmitValidation, validator, 0, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	r1 := exec.ExecuteTx(tx1, now)
	if !r1.Success {
		t.Fatalf("First vote failed: %s", r1.Error)
	}

	// Double vote should fail
	tx2 := makeTx(types.TxSubmitValidation, validator, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	r2 := exec.ExecuteTx(tx2, now)
	if r2.Success {
		t.Fatal("Double vote should fail")
	}
}

func TestExecuteValidationProofMismatch(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 3
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	v1 := types.HexToAddress("0x0000000000000000000000000000000000000010")
	v2 := types.HexToAddress("0x0000000000000000000000000000000000000011")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	s.SetBalance(v1, 50_000)
	s.SetBalance(v2, 50_000)
	s.Stake(v1, 20_000, now)
	s.Stake(v2, 20_000, now)

	createTx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500, Expiry: now + 86400,
	})
	exec.ExecuteTx(createTx, now)

	// V1 submits proof1
	proof1 := crypto.SHA256Hash([]byte("proof-001"))
	tx1 := makeTx(types.TxSubmitValidation, v1, 0, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proof1,
	})
	exec.ExecuteTx(tx1, now)

	// V2 submits different proof - should fail
	proof2 := crypto.SHA256Hash([]byte("proof-002"))
	tx2 := makeTx(types.TxSubmitValidation, v2, 0, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proof2,
	})
	r := exec.ExecuteTx(tx2, now)
	if r.Success {
		t.Fatal("Mismatched proof should fail")
	}
}

func TestExecuteCancelPayLink(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	createTx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500, Expiry: now + 86400,
	})
	exec.ExecuteTx(createTx, now)

	cancelTx := makeTx(types.TxCancelPayLink, merchant, 1, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	r := exec.ExecuteTx(cancelTx, now)
	if !r.Success {
		t.Fatalf("Cancel failed: %s", r.Error)
	}

	pl := s.GetPayLink(plID)
	if pl.Status != types.StatusCancelled {
		t.Fatalf("Expected CANCELLED, got %s", pl.Status)
	}
}

func TestExecuteCancelByNonCreator(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	createTx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500, Expiry: now + 86400,
	})
	exec.ExecuteTx(createTx, now)

	// Non-creator tries to cancel
	cancelTx := makeTx(types.TxCancelPayLink, receiver, 0, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	r := exec.ExecuteTx(cancelTx, now)
	if r.Success {
		t.Fatal("Non-creator cancel should fail")
	}
}

func TestExecuteFailPayLink(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-001"))
	now := time.Now().Unix()

	createTx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500, Expiry: now + 86400,
	})
	exec.ExecuteTx(createTx, now)

	// Admin fails the paylink
	failTx := makeTx(types.TxFailPayLink, genesis.AdminAddress, 0, types.FailPayLinkPayload{
		PayLinkID: plID,
	})
	r := exec.ExecuteTx(failTx, now)
	if !r.Success {
		t.Fatalf("FailPayLink failed: %s", r.Error)
	}

	pl := s.GetPayLink(plID)
	if pl.Status != types.StatusFailed {
		t.Fatalf("Expected FAILED, got %s", pl.Status)
	}
}

func TestExecuteStakeAndUnstake(t *testing.T) {
	genesis := testGenesis()
	genesis.MinimumStake = 10_000
	genesis.WithdrawalCooldown = 100 // 100 seconds for testing
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	admin := genesis.AdminAddress
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	now := time.Now().Unix()

	// Transfer funds to validator
	transferTx := makeTx(types.TxTransfer, admin, 0, types.TransferPayload{
		To: validator, Amount: 50_000,
	})
	exec.ExecuteTx(transferTx, now)

	// Stake
	stakeTx := makeTx(types.TxStake, validator, 0, types.StakePayload{Amount: 20_000})
	r := exec.ExecuteTx(stakeTx, now)
	if !r.Success {
		t.Fatalf("Stake failed: %s", r.Error)
	}

	if !s.IsActiveValidator(validator) {
		t.Fatal("Validator should be active")
	}
	if s.GetBalance(validator) != 30_000 {
		t.Fatalf("Validator balance: expected 30000, got %d", s.GetBalance(validator))
	}

	// Initiate unstake
	unstakeTx := makeTx(types.TxInitiateUnstake, validator, 1, types.InitiateUnstakePayload{Amount: 20_000})
	r = exec.ExecuteTx(unstakeTx, now)
	if !r.Success {
		t.Fatalf("InitiateUnstake failed: %s", r.Error)
	}

	if s.IsActiveValidator(validator) {
		t.Fatal("Validator should be deactivated after full unstake")
	}

	// Complete unstake before cooldown - should fail
	completeTx := makeTx(types.TxCompleteUnstake, validator, 2, types.CompleteUnstakePayload{})
	r = exec.ExecuteTx(completeTx, now)
	if r.Success {
		t.Fatal("CompleteUnstake before cooldown should fail")
	}

	// Complete unstake after cooldown
	completeTx2 := makeTx(types.TxCompleteUnstake, validator, 2, types.CompleteUnstakePayload{})
	r = exec.ExecuteTx(completeTx2, now+101)
	if !r.Success {
		t.Fatalf("CompleteUnstake failed: %s", r.Error)
	}

	if s.GetBalance(validator) != 50_000 {
		t.Fatalf("Validator balance after unstake: expected 50000, got %d", s.GetBalance(validator))
	}
}

func TestExecuteSlash(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	now := time.Now().Unix()

	s.SetBalance(validator, 50_000)
	s.Stake(validator, 30_000, now)

	// Admin slashes validator
	slashTx := makeTx(types.TxSlash, genesis.AdminAddress, 0, types.SlashPayload{
		Validator: validator,
		Amount:    25_000,
		Reason:    "double-signing",
	})
	r := exec.ExecuteTx(slashTx, now)
	if !r.Success {
		t.Fatalf("Slash failed: %s", r.Error)
	}

	v := s.GetValidator(validator)
	if v.StakedAmount != 5_000 {
		t.Fatalf("Staked amount: expected 5000, got %d", v.StakedAmount)
	}
	if v.IsActive {
		t.Fatal("Validator should be deactivated after slash below minimum")
	}
}

func TestExecuteDistributeReward(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	admin := genesis.AdminAddress
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	now := time.Now().Unix()

	// Transfer funds to validator, then stake via executor (which debits balance)
	transferTx := makeTx(types.TxTransfer, admin, 0, types.TransferPayload{
		To: validator, Amount: 50_000,
	})
	exec.ExecuteTx(transferTx, now)

	stakeTx := makeTx(types.TxStake, validator, 0, types.StakePayload{Amount: 20_000})
	exec.ExecuteTx(stakeTx, now)

	// Balance should be 30_000 after staking
	if s.GetBalance(validator) != 30_000 {
		t.Fatalf("Pre-reward balance: expected 30000, got %d", s.GetBalance(validator))
	}

	// Admin distributes reward
	rewardTx := makeTx(types.TxDistributeReward, admin, 1, types.DistributeRewardPayload{
		Validator: validator,
		Amount:    1000,
	})
	r := exec.ExecuteTx(rewardTx, now)
	if !r.Success {
		t.Fatalf("DistributeReward failed: %s", r.Error)
	}

	if s.GetBalance(validator) != 31_000 {
		t.Fatalf("Validator balance: expected 31000, got %d", s.GetBalance(validator))
	}

	v := s.GetValidator(validator)
	if v.TotalRewards != 1000 {
		t.Fatalf("Total rewards: expected 1000, got %d", v.TotalRewards)
	}
}

func TestExecuteBlockWithRevert(t *testing.T) {
	// ExecuteBlock enforces signatures, so build genesis around a real admin key.
	adminKey, admin := genTestKey(t)
	bobKey, bob := genTestKey(t)

	genesis := testGenesisWithAdmin(admin)
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	now := time.Now().Unix()

	txs := []types.Transaction{
		// Valid transfer
		*makeSignedTx(t, types.TxTransfer, adminKey, 0, types.TransferPayload{To: bob, Amount: 1000}),
		// Invalid transfer (bob has no balance yet for 2nd tx nonce): valid signature, bad nonce
		*makeSignedTx(t, types.TxTransfer, bobKey, 999, types.TransferPayload{To: admin, Amount: 5000}),
		// Another valid transfer
		*makeSignedTx(t, types.TxTransfer, adminKey, 1, types.TransferPayload{To: bob, Amount: 500}),
	}

	receipts := exec.ExecuteBlock(txs, now, 1)

	if !receipts[0].Success {
		t.Fatalf("Tx 0 should succeed: %s", receipts[0].Error)
	}
	if receipts[1].Success {
		t.Fatal("Tx 1 should fail (bad nonce)")
	}
	if !receipts[2].Success {
		t.Fatalf("Tx 2 should succeed: %s", receipts[2].Error)
	}

	if s.GetBalance(bob) != 1500 {
		t.Fatalf("Bob balance: expected 1500, got %d", s.GetBalance(bob))
	}
}

func TestAntiReplay(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 1
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	now := time.Now().Unix()

	s.SetBalance(validator, 50_000)
	s.Stake(validator, 20_000, now)

	proofHash := crypto.SHA256Hash([]byte("proof-001"))

	// Create and settle first paylink
	plID1 := crypto.SHA256Hash([]byte("PLK-001"))
	createTx1 := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID1, Receiver: receiver, Amount: 1500, Expiry: now + 86400,
	})
	exec.ExecuteTx(createTx1, now)

	voteTx1 := makeTx(types.TxSubmitValidation, validator, 0, types.SubmitValidationPayload{
		PayLinkID: plID1, ProofHash: proofHash,
	})
	exec.ExecuteTx(voteTx1, now)

	// Create second paylink, try to reuse proof
	plID2 := crypto.SHA256Hash([]byte("PLK-002"))
	createTx2 := makeTx(types.TxCreatePayLink, merchant, 1, types.CreatePayLinkPayload{
		PayLinkID: plID2, Receiver: receiver, Amount: 2000, Expiry: now + 86400,
	})
	exec.ExecuteTx(createTx2, now)

	voteTx2 := makeTx(types.TxSubmitValidation, validator, 1, types.SubmitValidationPayload{
		PayLinkID: plID2, ProofHash: proofHash,
	})
	r := exec.ExecuteTx(voteTx2, now)
	if r.Success {
		t.Fatal("Anti-replay: reused proof should be rejected")
	}
}
