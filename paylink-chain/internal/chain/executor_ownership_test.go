package chain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/rules"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/types"
)

// ── Helpers ──

func createPayLink(t *testing.T, exec *Executor, s *state.StateDB, from types.Address, nonce uint64, plID types.Hash, receiver types.Address, amount uint64, now int64, rulesJSON ...json.RawMessage) {
	t.Helper()
	payload := types.CreatePayLinkPayload{
		PayLinkID: plID,
		Receiver:  receiver,
		Amount:    amount,
		Expiry:    now + 86400,
	}
	if len(rulesJSON) > 0 && rulesJSON[0] != nil {
		payload.Rules = rulesJSON[0]
	}
	tx := makeTx(types.TxCreatePayLink, from, nonce, payload)
	r := exec.ExecuteTx(tx, now)
	if !r.Success {
		t.Fatalf("CreatePayLink failed: %s", r.Error)
	}
}

func mustMarshalRules(t *testing.T, ruleSet []rules.Rule) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(ruleSet)
	if err != nil {
		t.Fatalf("marshal rules: %v", err)
	}
	return data
}

// ═══════════════════════════════════════════════
// NFT Ownership Tests
// ═══════════════════════════════════════════════

func TestCreatePayLink_OwnerEqualsCreator(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-owner-001"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, merchant, 0, plID, receiver, 1000, now)

	pl := s.GetPayLink(plID)
	if pl.Owner != merchant {
		t.Fatalf("Owner should equal creator: got %s, want %s", pl.Owner.Hex(), merchant.Hex())
	}
	if pl.TransferCount != 0 {
		t.Fatalf("TransferCount should be 0: got %d", pl.TransferCount)
	}
	if !pl.Approved.IsZero() {
		t.Fatalf("Approved should be zero: got %s", pl.Approved.Hex())
	}
}

func TestTransferPayLink_ByOwner(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	newOwner := types.HexToAddress("0x0000000000000000000000000000000000000005")
	plID := crypto.SHA256Hash([]byte("PLK-transfer-001"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	// Transfer to newOwner
	tx := makeTx(types.TxTransferPayLink, owner, 1, types.TransferPayLinkPayload{
		PayLinkID: plID,
		To:        newOwner,
	})
	r := exec.ExecuteTx(tx, now)
	if !r.Success {
		t.Fatalf("Transfer failed: %s", r.Error)
	}

	pl := s.GetPayLink(plID)
	if pl.Owner != newOwner {
		t.Fatalf("Owner should be newOwner: got %s", pl.Owner.Hex())
	}
	if pl.TransferCount != 1 {
		t.Fatalf("TransferCount should be 1: got %d", pl.TransferCount)
	}
	if pl.Creator != owner {
		t.Fatalf("Creator should remain original: got %s", pl.Creator.Hex())
	}
	// Status should still be CREATED
	if pl.Status != types.StatusCreated {
		t.Fatalf("Status should remain CREATED: got %s", pl.Status)
	}
}

func TestTransferPayLink_ByApproved(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	approved := types.HexToAddress("0x0000000000000000000000000000000000000005")
	destination := types.HexToAddress("0x0000000000000000000000000000000000000006")
	plID := crypto.SHA256Hash([]byte("PLK-approved-transfer"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	// Owner approves `approved`
	approveTx := makeTx(types.TxApprovePayLink, owner, 1, types.ApprovePayLinkPayload{
		PayLinkID: plID,
		Approved:  approved,
	})
	r := exec.ExecuteTx(approveTx, now)
	if !r.Success {
		t.Fatalf("Approve failed: %s", r.Error)
	}

	// Verify approval was set
	pl := s.GetPayLink(plID)
	if pl.Approved != approved {
		t.Fatalf("Approved should be set: got %s", pl.Approved.Hex())
	}

	// Approved address transfers
	transferTx := makeTx(types.TxTransferPayLink, approved, 0, types.TransferPayLinkPayload{
		PayLinkID: plID,
		To:        destination,
	})
	r = exec.ExecuteTx(transferTx, now)
	if !r.Success {
		t.Fatalf("Transfer by approved failed: %s", r.Error)
	}

	// Verify transfer and approval cleared
	pl = s.GetPayLink(plID)
	if pl.Owner != destination {
		t.Fatalf("Owner should be destination: got %s", pl.Owner.Hex())
	}
	if !pl.Approved.IsZero() {
		t.Fatalf("Approval should be cleared after transfer: got %s", pl.Approved.Hex())
	}
}

func TestTransferPayLink_ByOperator(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	operator := types.HexToAddress("0x0000000000000000000000000000000000000005")
	destination := types.HexToAddress("0x0000000000000000000000000000000000000006")
	plID := crypto.SHA256Hash([]byte("PLK-operator-transfer"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	// Owner sets operator approval
	opTx := makeTx(types.TxSetApprovalForAll, owner, 1, types.SetApprovalForAllPayload{
		Operator: operator,
		Approved: true,
	})
	r := exec.ExecuteTx(opTx, now)
	if !r.Success {
		t.Fatalf("SetApprovalForAll failed: %s", r.Error)
	}

	// Verify operator approval
	if !s.IsOperatorApproved(owner, operator) {
		t.Fatal("Operator should be approved")
	}

	// Operator transfers
	transferTx := makeTx(types.TxTransferPayLink, operator, 0, types.TransferPayLinkPayload{
		PayLinkID: plID,
		To:        destination,
	})
	r = exec.ExecuteTx(transferTx, now)
	if !r.Success {
		t.Fatalf("Transfer by operator failed: %s", r.Error)
	}

	pl := s.GetPayLink(plID)
	if pl.Owner != destination {
		t.Fatalf("Owner should be destination: got %s", pl.Owner.Hex())
	}
}

func TestTransferPayLink_UnauthorizedFails(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	stranger := types.HexToAddress("0x0000000000000000000000000000000000000099")
	plID := crypto.SHA256Hash([]byte("PLK-unauth"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	tx := makeTx(types.TxTransferPayLink, stranger, 0, types.TransferPayLinkPayload{
		PayLinkID: plID,
		To:        stranger,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Transfer by unauthorized address should fail")
	}
}

func TestTransferPayLink_VerifiedStatusFails(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 1
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	newOwner := types.HexToAddress("0x0000000000000000000000000000000000000005")
	plID := crypto.SHA256Hash([]byte("PLK-verified-transfer"))
	now := time.Now().Unix()

	// Setup validator
	s.SetBalance(validator, 50_000)
	s.Stake(validator, 20_000, now)

	createPayLink(t, exec, s, merchant, 0, plID, receiver, 1000, now)

	// Settle the PayLink
	voteTx := makeTx(types.TxSubmitValidation, validator, 0, types.SubmitValidationPayload{
		PayLinkID: plID,
		ProofHash: crypto.SHA256Hash([]byte("proof")),
	})
	exec.ExecuteTx(voteTx, now)

	// Verify it's VERIFIED
	pl := s.GetPayLink(plID)
	if pl.Status != types.StatusVerified {
		t.Fatalf("Expected VERIFIED, got %s", pl.Status)
	}

	// Try to transfer — should fail
	tx := makeTx(types.TxTransferPayLink, merchant, 1, types.TransferPayLinkPayload{
		PayLinkID: plID,
		To:        newOwner,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Transfer of VERIFIED paylink should fail")
	}
}

func TestTransferPayLink_ToZeroFails(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-to-zero"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	tx := makeTx(types.TxTransferPayLink, owner, 1, types.TransferPayLinkPayload{
		PayLinkID: plID,
		To:        types.Address{},
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Transfer to zero address should fail")
	}
}

func TestTransferPayLink_ToSelfFails(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-to-self"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	tx := makeTx(types.TxTransferPayLink, owner, 1, types.TransferPayLinkPayload{
		PayLinkID: plID,
		To:        owner,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Transfer to self should fail")
	}
}

func TestApprovePayLink_OnlyOwner(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	stranger := types.HexToAddress("0x0000000000000000000000000000000000000099")
	plID := crypto.SHA256Hash([]byte("PLK-approve-only-owner"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	// Stranger tries to approve — should fail
	tx := makeTx(types.TxApprovePayLink, stranger, 0, types.ApprovePayLinkPayload{
		PayLinkID: plID,
		Approved:  stranger,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Non-owner approval should fail")
	}
}

func TestApprovePayLink_SelfApproveFails(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-self-approve"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	tx := makeTx(types.TxApprovePayLink, owner, 1, types.ApprovePayLinkPayload{
		PayLinkID: plID,
		Approved:  owner,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Self-approval should fail")
	}
}

func TestApprovePayLink_Revoke(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	approved := types.HexToAddress("0x0000000000000000000000000000000000000005")
	plID := crypto.SHA256Hash([]byte("PLK-revoke"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now)

	// Approve
	tx1 := makeTx(types.TxApprovePayLink, owner, 1, types.ApprovePayLinkPayload{
		PayLinkID: plID, Approved: approved,
	})
	exec.ExecuteTx(tx1, now)

	// Revoke (approve zero address)
	tx2 := makeTx(types.TxApprovePayLink, owner, 2, types.ApprovePayLinkPayload{
		PayLinkID: plID, Approved: types.Address{},
	})
	r := exec.ExecuteTx(tx2, now)
	if !r.Success {
		t.Fatalf("Revoke failed: %s", r.Error)
	}

	pl := s.GetPayLink(plID)
	if !pl.Approved.IsZero() {
		t.Fatalf("Approved should be zero after revoke: got %s", pl.Approved.Hex())
	}
}

func TestSetApprovalForAll_SelfFails(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	now := time.Now().Unix()

	tx := makeTx(types.TxSetApprovalForAll, owner, 0, types.SetApprovalForAllPayload{
		Operator: owner,
		Approved: true,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Self-operator should fail")
	}
}

func TestSetApprovalForAll_RevokeOperator(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	operator := types.HexToAddress("0x0000000000000000000000000000000000000005")
	now := time.Now().Unix()

	// Grant
	tx1 := makeTx(types.TxSetApprovalForAll, owner, 0, types.SetApprovalForAllPayload{
		Operator: operator, Approved: true,
	})
	exec.ExecuteTx(tx1, now)
	if !s.IsOperatorApproved(owner, operator) {
		t.Fatal("Operator should be approved")
	}

	// Revoke
	tx2 := makeTx(types.TxSetApprovalForAll, owner, 1, types.SetApprovalForAllPayload{
		Operator: operator, Approved: false,
	})
	r := exec.ExecuteTx(tx2, now)
	if !r.Success {
		t.Fatalf("Revoke failed: %s", r.Error)
	}
	if s.IsOperatorApproved(owner, operator) {
		t.Fatal("Operator should no longer be approved")
	}
}

func TestCancelPayLink_ByNewOwner(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	creator := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	newOwner := types.HexToAddress("0x0000000000000000000000000000000000000005")
	plID := crypto.SHA256Hash([]byte("PLK-cancel-by-new-owner"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, creator, 0, plID, receiver, 1000, now)

	// Transfer to newOwner
	tx1 := makeTx(types.TxTransferPayLink, creator, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: newOwner,
	})
	exec.ExecuteTx(tx1, now)

	// New owner cancels
	tx2 := makeTx(types.TxCancelPayLink, newOwner, 0, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	r := exec.ExecuteTx(tx2, now)
	if !r.Success {
		t.Fatalf("Cancel by new owner failed: %s", r.Error)
	}

	pl := s.GetPayLink(plID)
	if pl.Status != types.StatusCancelled {
		t.Fatalf("Expected CANCELLED, got %s", pl.Status)
	}
}

func TestCancelPayLink_ByOriginalCreatorAfterTransfer(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	creator := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	newOwner := types.HexToAddress("0x0000000000000000000000000000000000000005")
	plID := crypto.SHA256Hash([]byte("PLK-cancel-by-creator-after-transfer"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, creator, 0, plID, receiver, 1000, now)

	// Transfer
	tx1 := makeTx(types.TxTransferPayLink, creator, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: newOwner,
	})
	exec.ExecuteTx(tx1, now)

	// Original creator can still cancel
	tx2 := makeTx(types.TxCancelPayLink, creator, 2, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	r := exec.ExecuteTx(tx2, now)
	if !r.Success {
		t.Fatalf("Cancel by original creator failed: %s", r.Error)
	}
}

func TestOwnerIndex_TracksTransfers(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	alice := types.HexToAddress("0x000000000000000000000000000000000000000A")
	bob := types.HexToAddress("0x000000000000000000000000000000000000000B")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	now := time.Now().Unix()

	// Alice creates 2 PayLinks
	plID1 := crypto.SHA256Hash([]byte("PLK-idx-1"))
	plID2 := crypto.SHA256Hash([]byte("PLK-idx-2"))
	createPayLink(t, exec, s, alice, 0, plID1, receiver, 100, now)
	createPayLink(t, exec, s, alice, 1, plID2, receiver, 200, now)

	if s.OwnerPayLinkCount(alice) != 2 {
		t.Fatalf("Alice should own 2: got %d", s.OwnerPayLinkCount(alice))
	}
	if s.OwnerPayLinkCount(bob) != 0 {
		t.Fatalf("Bob should own 0: got %d", s.OwnerPayLinkCount(bob))
	}

	// Transfer plID1 to Bob
	tx := makeTx(types.TxTransferPayLink, alice, 2, types.TransferPayLinkPayload{
		PayLinkID: plID1, To: bob,
	})
	exec.ExecuteTx(tx, now)

	if s.OwnerPayLinkCount(alice) != 1 {
		t.Fatalf("Alice should own 1 after transfer: got %d", s.OwnerPayLinkCount(alice))
	}
	if s.OwnerPayLinkCount(bob) != 1 {
		t.Fatalf("Bob should own 1 after transfer: got %d", s.OwnerPayLinkCount(bob))
	}

	// Verify owner queries
	aliceLinks := s.GetPayLinksByOwner(alice)
	if len(aliceLinks) != 1 || aliceLinks[0] != plID2 {
		t.Fatal("Alice's remaining PayLink should be plID2")
	}
	bobLinks := s.GetPayLinksByOwner(bob)
	if len(bobLinks) != 1 || bobLinks[0] != plID1 {
		t.Fatal("Bob's PayLink should be plID1")
	}
}

func TestMultipleTransfers_ChainedOwnership(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	alice := types.HexToAddress("0x000000000000000000000000000000000000000A")
	bob := types.HexToAddress("0x000000000000000000000000000000000000000B")
	charlie := types.HexToAddress("0x000000000000000000000000000000000000000C")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-chain"))
	now := time.Now().Unix()

	createPayLink(t, exec, s, alice, 0, plID, receiver, 500, now)

	// Alice → Bob
	tx1 := makeTx(types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	exec.ExecuteTx(tx1, now)

	// Bob → Charlie
	tx2 := makeTx(types.TxTransferPayLink, bob, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: charlie,
	})
	exec.ExecuteTx(tx2, now)

	pl := s.GetPayLink(plID)
	if pl.Owner != charlie {
		t.Fatalf("Owner should be charlie: got %s", pl.Owner.Hex())
	}
	if pl.Creator != alice {
		t.Fatalf("Creator should remain alice: got %s", pl.Creator.Hex())
	}
	if pl.TransferCount != 2 {
		t.Fatalf("TransferCount should be 2: got %d", pl.TransferCount)
	}

	// Alice can no longer transfer (not owner)
	tx3 := makeTx(types.TxTransferPayLink, alice, 2, types.TransferPayLinkPayload{
		PayLinkID: plID, To: alice,
	})
	r := exec.ExecuteTx(tx3, now)
	if r.Success {
		t.Fatal("Previous owner should not be able to transfer")
	}
}

// ═══════════════════════════════════════════════
// Rules Engine Integration Tests
// ═══════════════════════════════════════════════

func TestCreatePayLink_WithValidRules(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-rules-valid"))
	now := time.Now().Unix()

	rulesData := mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleMaxTransfers, Params: json.RawMessage(`{"max":3}`)},
	})

	createPayLink(t, exec, s, merchant, 0, plID, receiver, 1000, now, rulesData)

	pl := s.GetPayLink(plID)
	if len(pl.Rules) == 0 {
		t.Fatal("Rules should be stored on PayLink")
	}
}

func TestCreatePayLink_WithInvalidRules(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-rules-invalid"))
	now := time.Now().Unix()

	// MaxTransfers with max=0 is invalid
	rulesData := mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleMaxTransfers, Params: json.RawMessage(`{"max":0}`)},
	})

	tx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID,
		Receiver:  receiver,
		Amount:    1000,
		Expiry:    now + 86400,
		Rules:     rulesData,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Creating PayLink with invalid rules should fail")
	}
}

func TestCreatePayLink_WithUnknownRuleType(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-rules-unknown"))
	now := time.Now().Unix()

	rulesData := json.RawMessage(`[{"type":"FooBarRule","params":{}}]`)

	tx := makeTx(types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1000, Expiry: now + 86400,
		Rules: rulesData,
	})
	r := exec.ExecuteTx(tx, now)
	if r.Success {
		t.Fatal("Creating PayLink with unknown rule type should fail")
	}
}

func TestTransferPayLink_BlockedByMaxTransfers(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	alice := types.HexToAddress("0x000000000000000000000000000000000000000A")
	bob := types.HexToAddress("0x000000000000000000000000000000000000000B")
	charlie := types.HexToAddress("0x000000000000000000000000000000000000000C")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-max-transfers"))
	now := time.Now().Unix()

	// Max 1 transfer allowed
	rulesData := mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleMaxTransfers, Params: json.RawMessage(`{"max":1}`)},
	})
	createPayLink(t, exec, s, alice, 0, plID, receiver, 1000, now, rulesData)

	// First transfer: Alice → Bob (should succeed)
	tx1 := makeTx(types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	r := exec.ExecuteTx(tx1, now)
	if !r.Success {
		t.Fatalf("First transfer should succeed: %s", r.Error)
	}

	// Second transfer: Bob → Charlie (should fail, max reached)
	tx2 := makeTx(types.TxTransferPayLink, bob, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: charlie,
	})
	r = exec.ExecuteTx(tx2, now)
	if r.Success {
		t.Fatal("Second transfer should be blocked by MaxTransfers rule")
	}
}

func TestTransferPayLink_BlockedByReceiverWhitelist(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	owner := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	allowed := types.HexToAddress("0x0000000000000000000000000000000000000005")
	blocked := types.HexToAddress("0x0000000000000000000000000000000000000006")
	plID := crypto.SHA256Hash([]byte("PLK-receiver-wl"))
	now := time.Now().Unix()

	rulesData := mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleReceiverWhitelist, Params: mustMarshalRules(t, nil)}, // will override below
	})
	// Build proper params
	params, _ := json.Marshal(rules.ReceiverWhitelistParams{
		Addresses: []types.Address{allowed},
	})
	rulesData = mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleReceiverWhitelist, Params: params},
	})
	createPayLink(t, exec, s, owner, 0, plID, receiver, 1000, now, rulesData)

	// Transfer to allowed address — should succeed
	tx1 := makeTx(types.TxTransferPayLink, owner, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: allowed,
	})
	r := exec.ExecuteTx(tx1, now)
	if !r.Success {
		t.Fatalf("Transfer to whitelisted address should succeed: %s", r.Error)
	}

	// Transfer from allowed to blocked address — should fail
	tx2 := makeTx(types.TxTransferPayLink, allowed, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: blocked,
	})
	r = exec.ExecuteTx(tx2, now)
	if r.Success {
		t.Fatal("Transfer to non-whitelisted address should fail")
	}
}

func TestSettlement_BlockedByTimeLock(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 1
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	plID := crypto.SHA256Hash([]byte("PLK-timelock-blocked"))
	now := int64(1000)

	s.SetBalance(validator, 50_000)
	s.Stake(validator, 20_000, now)

	// TimeLock: settle only allowed after timestamp 2000
	params, _ := json.Marshal(rules.TimeLockParams{
		NotBefore: 2000,
		Actions:   []rules.ActionKind{rules.ActionSettle},
	})
	rulesData := mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleTimeLock, Params: params},
	})
	createPayLink(t, exec, s, merchant, 0, plID, receiver, 1000, now, rulesData)

	// Vote at timestamp 1500 (before NotBefore=2000): reaches quorum but TimeLock blocks settlement
	voteTx := makeTx(types.TxSubmitValidation, validator, 0, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: crypto.SHA256Hash([]byte("proof-tl")),
	})
	r := exec.ExecuteTx(voteTx, 1500)
	// Tx should fail because TimeLock blocks
	if r.Success {
		pl := s.GetPayLink(plID)
		if pl.Status == types.StatusVerified {
			t.Fatal("Settlement should be blocked by TimeLock before NotBefore")
		}
	}
	pl := s.GetPayLink(plID)
	if pl.Status == types.StatusVerified {
		t.Fatal("PayLink should not be VERIFIED before NotBefore")
	}
}

func TestSettlement_AllowedAfterTimeLock(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 1
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	plID := crypto.SHA256Hash([]byte("PLK-timelock-allowed"))
	now := int64(1000)

	s.SetBalance(validator, 50_000)
	s.Stake(validator, 20_000, now)

	// TimeLock: settle only allowed after timestamp 2000
	params, _ := json.Marshal(rules.TimeLockParams{
		NotBefore: 2000,
		Actions:   []rules.ActionKind{rules.ActionSettle},
	})
	rulesData := mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleTimeLock, Params: params},
	})
	createPayLink(t, exec, s, merchant, 0, plID, receiver, 1000, now, rulesData)

	// Vote at timestamp 2500 (after NotBefore=2000): quorum reached + TimeLock passes
	voteTx := makeTx(types.TxSubmitValidation, validator, 0, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: crypto.SHA256Hash([]byte("proof-tl2")),
	})
	r := exec.ExecuteTx(voteTx, 2500)
	if !r.Success {
		t.Fatalf("Settlement after NotBefore should succeed: %s", r.Error)
	}
	pl := s.GetPayLink(plID)
	if pl.Status != types.StatusVerified {
		t.Fatalf("Expected VERIFIED after time lock expired, got %s", pl.Status)
	}
}

func TestCancel_BlockedByAddressWhitelist(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	creator := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	allowedCanceller := types.HexToAddress("0x0000000000000000000000000000000000000005")
	plID := crypto.SHA256Hash([]byte("PLK-cancel-whitelist"))
	now := time.Now().Unix()

	// Only allowedCanceller can cancel
	params, _ := json.Marshal(rules.AddressWhitelistParams{
		Addresses: []types.Address{allowedCanceller},
		Actions:   []rules.ActionKind{rules.ActionCancel},
	})
	rulesData := mustMarshalRules(t, []rules.Rule{
		{Type: rules.RuleAddressWhitelist, Params: params},
	})
	createPayLink(t, exec, s, creator, 0, plID, receiver, 1000, now, rulesData)

	// Creator tries to cancel — should fail (not in whitelist)
	cancelTx := makeTx(types.TxCancelPayLink, creator, 1, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	r := exec.ExecuteTx(cancelTx, now)
	if r.Success {
		t.Fatal("Cancel by non-whitelisted address should fail")
	}

	// Transfer ownership to allowedCanceller, then cancel
	transferTx := makeTx(types.TxTransferPayLink, creator, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: allowedCanceller,
	})
	exec.ExecuteTx(transferTx, now)

	cancelTx2 := makeTx(types.TxCancelPayLink, allowedCanceller, 0, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	r = exec.ExecuteTx(cancelTx2, now)
	if !r.Success {
		t.Fatalf("Cancel by whitelisted address should succeed: %s", r.Error)
	}
}

func TestPayLink_NoRules_UnchangedBehavior(t *testing.T) {
	genesis := testGenesis()
	genesis.RequiredValidations = 1
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	merchant := types.HexToAddress("0x0000000000000000000000000000000000000003")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	validator := types.HexToAddress("0x0000000000000000000000000000000000000010")
	plID := crypto.SHA256Hash([]byte("PLK-no-rules"))
	now := time.Now().Unix()

	s.SetBalance(validator, 50_000)
	s.Stake(validator, 20_000, now)

	// Create WITHOUT rules
	createPayLink(t, exec, s, merchant, 0, plID, receiver, 1000, now)

	// Settle immediately — should work
	voteTx := makeTx(types.TxSubmitValidation, validator, 0, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: crypto.SHA256Hash([]byte("proof")),
	})
	r := exec.ExecuteTx(voteTx, now)
	if !r.Success {
		t.Fatalf("Settlement without rules should succeed: %s", r.Error)
	}
	pl := s.GetPayLink(plID)
	if pl.Status != types.StatusVerified {
		t.Fatalf("Expected VERIFIED, got %s", pl.Status)
	}
}

// ═══════════════════════════════════════════════
// Snapshot/Revert with new ownership state
// ═══════════════════════════════════════════════

func TestExecuteBlock_OwnershipRevert(t *testing.T) {
	genesis := testGenesis()
	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	// ExecuteBlock enforces signatures, so alice and bob need real keys.
	aliceKey, alice := genTestKey(t)
	bobKey, bob := genTestKey(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := crypto.SHA256Hash([]byte("PLK-revert"))
	now := time.Now().Unix()

	// Create PayLink directly (ExecuteTx does not verify signatures)
	createPayLink(t, exec, s, alice, 0, plID, receiver, 1000, now)

	// Build a block with: valid transfer + invalid tx (valid signature, bad nonce)
	txGood := makeSignedTx(t, types.TxTransferPayLink, aliceKey, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	txBad := makeSignedTx(t, types.TxTransferPayLink, bobKey, 99, types.TransferPayLinkPayload{
		PayLinkID: plID, To: alice,
	})

	txs := []types.Transaction{*txGood, *txBad}
	receipts := exec.ExecuteBlock(txs, now, 1)

	if !receipts[0].Success {
		t.Fatalf("First tx should succeed: %s", receipts[0].Error)
	}
	if receipts[1].Success {
		t.Fatal("Second tx should fail (bad nonce)")
	}

	// Verify: ownership changed by first tx, second tx reverted
	pl := s.GetPayLink(plID)
	if pl.Owner != bob {
		t.Fatalf("Owner should be bob after first tx: got %s", pl.Owner.Hex())
	}
}
