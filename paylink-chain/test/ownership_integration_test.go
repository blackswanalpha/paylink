package test

import (
	"encoding/json"
	"testing"
	"time"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/rpc"
	"github.com/paylink/paylink-chain/internal/rules"
	"github.com/paylink/paylink-chain/internal/types"
)

// ═══════════════════════════════════════════════════════
// Ownership Integration Tests
// ═══════════════════════════════════════════════════════

// Test: PayLink creation sets owner = creator, owner queryable via RPC
func TestIntegration_OwnershipOnCreate(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-OWN-001"))

	// Fund merchant
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: merchant, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create PayLink
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// Verify owner == creator via getPayLink
	pl := getPayLink(t, node, plID)
	if pl.Owner != merchant.Hex() {
		t.Fatalf("Owner: expected %s, got %s", merchant.Hex(), pl.Owner)
	}
	if pl.TransferCount != 0 {
		t.Fatalf("TransferCount: expected 0, got %d", pl.TransferCount)
	}

	// Verify ownerOf RPC
	result := rpcCall(t, node.rpcURL, "paylink_ownerOf", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var ownerResp struct{ Owner string }
	json.Unmarshal(result, &ownerResp)
	if ownerResp.Owner != merchant.Hex() {
		t.Fatalf("ownerOf: expected %s, got %s", merchant.Hex(), ownerResp.Owner)
	}

	// Verify balanceOf RPC
	result = rpcCall(t, node.rpcURL, "paylink_balanceOf", map[string]string{
		"owner": merchant.Hex(),
	})
	var balResp struct{ Balance int }
	json.Unmarshal(result, &balResp)
	if balResp.Balance != 1 {
		t.Fatalf("balanceOf: expected 1, got %d", balResp.Balance)
	}

	// Verify getPayLinksByOwner RPC
	result = rpcCall(t, node.rpcURL, "paylink_getPayLinksByOwner", map[string]string{
		"owner": merchant.Hex(),
	})
	var plList []rpc.PayLinkResponse
	json.Unmarshal(result, &plList)
	if len(plList) != 1 {
		t.Fatalf("getPayLinksByOwner: expected 1 paylink, got %d", len(plList))
	}
	if plList[0].ID != plID.Hex() {
		t.Fatalf("getPayLinksByOwner[0].ID: expected %s, got %s", plID.Hex(), plList[0].ID)
	}
}

// Test: Full ownership transfer lifecycle via block production
func TestIntegration_TransferOwnership(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-XFER-001"))

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create PayLink as alice
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// Verify alice owns it
	pl := getPayLink(t, node, plID)
	if pl.Owner != alice.Hex() {
		t.Fatalf("Owner before transfer: expected %s, got %s", alice.Hex(), pl.Owner)
	}

	// Transfer from alice to bob
	sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// Verify bob is new owner
	pl = getPayLink(t, node, plID)
	if pl.Owner != bob.Hex() {
		t.Fatalf("Owner after transfer: expected %s, got %s", bob.Hex(), pl.Owner)
	}
	if pl.TransferCount != 1 {
		t.Fatalf("TransferCount: expected 1, got %d", pl.TransferCount)
	}
	if pl.Status != "CREATED" {
		t.Fatalf("Status after transfer: expected CREATED, got %s", pl.Status)
	}

	// Verify ownerOf RPC reflects new owner
	result := rpcCall(t, node.rpcURL, "paylink_ownerOf", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var ownerResp struct{ Owner string }
	json.Unmarshal(result, &ownerResp)
	if ownerResp.Owner != bob.Hex() {
		t.Fatalf("ownerOf after transfer: expected %s, got %s", bob.Hex(), ownerResp.Owner)
	}

	// Verify balanceOf updated for both
	result = rpcCall(t, node.rpcURL, "paylink_balanceOf", map[string]string{"owner": alice.Hex()})
	var aliceBal struct{ Balance int }
	json.Unmarshal(result, &aliceBal)
	if aliceBal.Balance != 0 {
		t.Fatalf("alice balanceOf: expected 0, got %d", aliceBal.Balance)
	}

	result = rpcCall(t, node.rpcURL, "paylink_balanceOf", map[string]string{"owner": bob.Hex()})
	var bobBal struct{ Balance int }
	json.Unmarshal(result, &bobBal)
	if bobBal.Balance != 1 {
		t.Fatalf("bob balanceOf: expected 1, got %d", bobBal.Balance)
	}
}

// Test: Approval and transfer-by-approved through block production
func TestIntegration_ApprovalAndTransfer(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	charlie := types.HexToAddress("0x00000000000000000000000000000000000c4a12")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-APPROVE-001"))

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create PayLink
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// Alice approves bob
	sendTx(t, node, types.TxApprovePayLink, alice, 1, types.ApprovePayLinkPayload{
		PayLinkID: plID, Approved: bob,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// Verify getApproved RPC
	result := rpcCall(t, node.rpcURL, "paylink_getApproved", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var appResp struct{ Approved string }
	json.Unmarshal(result, &appResp)
	if appResp.Approved != bob.Hex() {
		t.Fatalf("getApproved: expected %s, got %s", bob.Hex(), appResp.Approved)
	}

	// Bob (approved) transfers to charlie
	sendTx(t, node, types.TxTransferPayLink, bob, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: charlie,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	// Verify charlie is owner and approval is cleared
	pl := getPayLink(t, node, plID)
	if pl.Owner != charlie.Hex() {
		t.Fatalf("Owner after approved transfer: expected %s, got %s", charlie.Hex(), pl.Owner)
	}
	if pl.Approved != types.ZeroAddress.Hex() {
		t.Fatalf("Approved should be cleared after transfer, got %s", pl.Approved)
	}
}

// Test: Operator (ApprovalForAll) and transfer-by-operator through block production
func TestIntegration_OperatorTransfer(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	operator := node.newActor(t)
	bob := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-OPERATOR-001"))

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create PayLink
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// Alice sets operator
	sendTx(t, node, types.TxSetApprovalForAll, alice, 1, types.SetApprovalForAllPayload{
		Operator: operator, Approved: true,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// Verify isApprovedForAll RPC
	result := rpcCall(t, node.rpcURL, "paylink_isApprovedForAll", map[string]string{
		"owner":    alice.Hex(),
		"operator": operator.Hex(),
	})
	var approvedResp struct{ Approved bool }
	json.Unmarshal(result, &approvedResp)
	if !approvedResp.Approved {
		t.Fatal("isApprovedForAll: expected true")
	}

	// Operator transfers to bob
	sendTx(t, node, types.TxTransferPayLink, operator, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	// Verify bob is owner
	pl := getPayLink(t, node, plID)
	if pl.Owner != bob.Hex() {
		t.Fatalf("Owner after operator transfer: expected %s, got %s", bob.Hex(), pl.Owner)
	}
}

// Test: New owner can cancel, creator can also cancel after transfer
func TestIntegration_CancelAfterTransfer(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-CANCEL-OWN-001"))

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create and transfer to bob
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// Bob (new owner) cancels
	sendTx(t, node, types.TxCancelPayLink, bob, 0, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	pl := getPayLink(t, node, plID)
	if pl.Status != "CANCELLED" {
		t.Fatalf("Status after cancel by new owner: expected CANCELLED, got %s", pl.Status)
	}
}

// ═══════════════════════════════════════════════════════
// Rules Integration Tests
// ═══════════════════════════════════════════════════════

func mustMarshalJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// Test: PayLink created with rules, rules queryable via RPC
func TestIntegration_CreateWithRules(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-RULES-001"))

	ruleSet := []rules.Rule{{
		Type:   rules.RuleMaxTransfers,
		Params: mustMarshalJSON(rules.MaxTransfersParams{Max: 3}),
	}}
	rulesJSON, _ := json.Marshal(ruleSet)

	// Fund merchant
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: merchant, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create PayLink with rules
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
		Rules:  rulesJSON,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// Verify rules stored via getPayLink
	pl := getPayLink(t, node, plID)
	if pl.Rules == nil {
		t.Fatal("Rules should be stored on PayLink")
	}

	// Verify getPayLinkRules RPC
	result := rpcCall(t, node.rpcURL, "paylink_getPayLinkRules", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var rulesResp struct{ Rules json.RawMessage }
	json.Unmarshal(result, &rulesResp)

	var parsedRules []rules.Rule
	json.Unmarshal(rulesResp.Rules, &parsedRules)
	if len(parsedRules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(parsedRules))
	}
	if parsedRules[0].Type != rules.RuleMaxTransfers {
		t.Fatalf("Rule type: expected %s, got %s", rules.RuleMaxTransfers, parsedRules[0].Type)
	}
}

// Test: MaxTransfers rule blocks transfer after limit reached
func TestIntegration_MaxTransfersRuleBlocks(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	charlie := types.HexToAddress("0x00000000000000000000000000000000000c4a12")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-MAXRULE-001"))

	ruleSet := []rules.Rule{{
		Type:   rules.RuleMaxTransfers,
		Params: mustMarshalJSON(rules.MaxTransfersParams{Max: 1}),
	}}
	rulesJSON, _ := json.Marshal(ruleSet)

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create PayLink with max 1 transfer
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
		Rules:  rulesJSON,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// First transfer: alice → bob (should succeed)
	sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	pl := getPayLink(t, node, plID)
	if pl.Owner != bob.Hex() {
		t.Fatalf("First transfer should succeed, owner: %s", pl.Owner)
	}
	if pl.TransferCount != 1 {
		t.Fatalf("TransferCount: expected 1, got %d", pl.TransferCount)
	}

	// Second transfer: bob → charlie (should be blocked by MaxTransfers rule).
	// A blocked tx doesn't produce a block, so wait on its receipt instead.
	blockedHash := sendTx(t, node, types.TxTransferPayLink, bob, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: charlie,
	})
	if r := waitForReceipt(t, node, blockedHash, 3*time.Second); r.Success {
		t.Fatal("Second transfer should be blocked by MaxTransfers rule")
	}

	// Owner should still be bob (transfer rejected)
	pl = getPayLink(t, node, plID)
	if pl.Owner != bob.Hex() {
		t.Fatalf("Second transfer should be blocked, owner: expected %s, got %s", bob.Hex(), pl.Owner)
	}
	if pl.TransferCount != 1 {
		t.Fatalf("TransferCount should still be 1, got %d", pl.TransferCount)
	}
}

// Test: ReceiverWhitelist rule blocks non-whitelisted recipients
func TestIntegration_ReceiverWhitelistRule(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	charlie := types.HexToAddress("0x00000000000000000000000000000000000c4a12")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-RCVWL-001"))

	// Only allow transfer to bob
	ruleSet := []rules.Rule{{
		Type: rules.RuleReceiverWhitelist,
		Params: mustMarshalJSON(rules.ReceiverWhitelistParams{
			Addresses: []types.Address{bob},
		}),
	}}
	rulesJSON, _ := json.Marshal(ruleSet)

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create with receiver whitelist
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
		Rules:  rulesJSON,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// Try transfer to charlie (not whitelisted) -- should fail.
	// A blocked tx doesn't produce a block, so wait on its receipt instead.
	blockedHash := sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: charlie,
	})
	if r := waitForReceipt(t, node, blockedHash, 3*time.Second); r.Success {
		t.Fatal("Transfer to non-whitelisted should be blocked")
	}

	pl := getPayLink(t, node, plID)
	if pl.Owner != alice.Hex() {
		t.Fatalf("Transfer to non-whitelisted should be blocked, owner: %s", pl.Owner)
	}

	// Transfer to bob (whitelisted) -- should succeed
	// Nonce stays at 1 because the rejected tx didn't consume it
	sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	pl = getPayLink(t, node, plID)
	if pl.Owner != bob.Hex() {
		t.Fatalf("Transfer to whitelisted should succeed, owner: expected %s, got %s", bob.Hex(), pl.Owner)
	}
}

// Test: AddressWhitelist rule blocks cancel by non-whitelisted sender
func TestIntegration_AddressWhitelistCancelRule(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-ADDRWL-001"))

	// Only alice can cancel
	ruleSet := []rules.Rule{{
		Type: rules.RuleAddressWhitelist,
		Params: mustMarshalJSON(rules.AddressWhitelistParams{
			Addresses: []types.Address{alice},
			Actions:   []rules.ActionKind{rules.ActionCancel},
		}),
	}}
	rulesJSON, _ := json.Marshal(ruleSet)

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create PayLink and transfer to bob
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
		Rules:  rulesJSON,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// Bob (owner) tries to cancel -- should be blocked by AddressWhitelist.
	// A blocked tx doesn't produce a block, so wait on its receipt instead.
	blockedHash := sendTx(t, node, types.TxCancelPayLink, bob, 0, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	if r := waitForReceipt(t, node, blockedHash, 3*time.Second); r.Success {
		t.Fatal("Cancel by non-whitelisted should be blocked")
	}

	pl := getPayLink(t, node, plID)
	if pl.Status != "CREATED" {
		t.Fatalf("Cancel by non-whitelisted should be blocked, status: %s", pl.Status)
	}

	// Alice (whitelisted, original creator) cancels -- should succeed
	sendTx(t, node, types.TxCancelPayLink, alice, 2, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	pl = getPayLink(t, node, plID)
	if pl.Status != "CANCELLED" {
		t.Fatalf("Cancel by whitelisted should succeed, status: %s", pl.Status)
	}
}

// Test: Full lifecycle -- create with rules, transfer, settle with rules enforcement
func TestIntegration_FullLifecycleWithOwnershipAndRules(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-FULL-001"))
	proofHash := pcrypto.SHA256Hash([]byte("mpesa-tx-FULL-001"))

	// Rules: max 2 transfers, amount threshold 100-5000
	ruleSet := []rules.Rule{
		{Type: rules.RuleMaxTransfers, Params: mustMarshalJSON(rules.MaxTransfersParams{Max: 2})},
		{Type: rules.RuleAmountThreshold, Params: mustMarshalJSON(rules.AmountThresholdParams{MinAmount: 100, MaxAmount: 5000})},
	}
	rulesJSON, _ := json.Marshal(ruleSet)

	// Setup: fund alice and 3 validators
	v1 := node.newActor(t)
	v2 := node.newActor(t)
	v3 := node.newActor(t)

	nonce := uint64(0)
	sendTx(t, node, types.TxTransfer, node.adminAddr, nonce, types.TransferPayload{To: alice, Amount: 10_000})
	nonce++
	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxTransfer, node.adminAddr, nonce, types.TransferPayload{To: v, Amount: 50_000})
		nonce++
	}
	waitForBlock(t, node, 1, 3*time.Second)

	// Stake validators
	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxStake, v, 0, types.StakePayload{Amount: 20_000})
	}
	waitForBlock(t, node, 2, 3*time.Second)

	// Create PayLink with rules
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500,
		Expiry: time.Now().Unix() + 86400,
		Rules:  rulesJSON,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	pl := getPayLink(t, node, plID)
	if pl.Status != "CREATED" {
		t.Fatalf("Status: expected CREATED, got %s", pl.Status)
	}
	if pl.Owner != alice.Hex() {
		t.Fatalf("Owner: expected %s, got %s", alice.Hex(), pl.Owner)
	}

	// Transfer to bob
	sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	pl = getPayLink(t, node, plID)
	if pl.Owner != bob.Hex() {
		t.Fatalf("Owner after transfer: expected %s, got %s", bob.Hex(), pl.Owner)
	}

	// Settle via 3 validators (quorum)
	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxSubmitValidation, v, 1, types.SubmitValidationPayload{
			PayLinkID: plID, ProofHash: proofHash,
		})
	}
	waitForBlock(t, node, 5, 3*time.Second)

	// Verify settled
	pl = getPayLink(t, node, plID)
	if pl.Status != "VERIFIED" {
		t.Fatalf("Status after settlement: expected VERIFIED, got %s", pl.Status)
	}
	if pl.VoteCount != 3 {
		t.Fatalf("VoteCount: expected 3, got %d", pl.VoteCount)
	}
	if pl.Owner != bob.Hex() {
		t.Fatalf("Owner unchanged after settlement: expected %s, got %s", bob.Hex(), pl.Owner)
	}

	// Transfer should fail on VERIFIED paylink.
	// A failed tx doesn't produce a block, so wait on its receipt instead.
	failedHash := sendTx(t, node, types.TxTransferPayLink, bob, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: alice,
	})
	if r := waitForReceipt(t, node, failedHash, 3*time.Second); r.Success {
		t.Fatal("Transfer on VERIFIED paylink should fail")
	}

	pl = getPayLink(t, node, plID)
	if pl.Owner != bob.Hex() {
		t.Fatalf("Transfer on VERIFIED should fail, owner: %s", pl.Owner)
	}
}

// Test: Multiple PayLinks per owner, enumeration via RPC
func TestIntegration_MultiplePayLinksEnumeration(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")

	plID1 := pcrypto.SHA256Hash([]byte("PLK-ENUM-001"))
	plID2 := pcrypto.SHA256Hash([]byte("PLK-ENUM-002"))
	plID3 := pcrypto.SHA256Hash([]byte("PLK-ENUM-003"))

	// Fund merchant
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: merchant, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create 3 PayLinks
	for i, id := range []types.Hash{plID1, plID2, plID3} {
		sendTx(t, node, types.TxCreatePayLink, merchant, uint64(i), types.CreatePayLinkPayload{
			PayLinkID: id, Receiver: receiver, Amount: 500,
			Expiry: time.Now().Unix() + 86400,
		})
	}
	waitForBlock(t, node, 2, 3*time.Second)

	// Verify balanceOf
	result := rpcCall(t, node.rpcURL, "paylink_balanceOf", map[string]string{
		"owner": merchant.Hex(),
	})
	var balResp struct{ Balance int }
	json.Unmarshal(result, &balResp)
	if balResp.Balance != 3 {
		t.Fatalf("balanceOf: expected 3, got %d", balResp.Balance)
	}

	// Verify getPayLinksByOwner returns all 3
	result = rpcCall(t, node.rpcURL, "paylink_getPayLinksByOwner", map[string]string{
		"owner": merchant.Hex(),
	})
	var plList []rpc.PayLinkResponse
	json.Unmarshal(result, &plList)
	if len(plList) != 3 {
		t.Fatalf("getPayLinksByOwner: expected 3, got %d", len(plList))
	}
}

// Test: No rules means original behavior is unchanged
func TestIntegration_NoRulesUnchangedBehavior(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := node.newActor(t)
	charlie := types.HexToAddress("0x00000000000000000000000000000000000c4a12")
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-NORULE-001"))

	// Fund alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Create without rules
	sendTx(t, node, types.TxCreatePayLink, alice, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	// Transfer freely: alice → bob → charlie
	sendTx(t, node, types.TxTransferPayLink, alice, 1, types.TransferPayLinkPayload{
		PayLinkID: plID, To: bob,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	sendTx(t, node, types.TxTransferPayLink, bob, 0, types.TransferPayLinkPayload{
		PayLinkID: plID, To: charlie,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	pl := getPayLink(t, node, plID)
	if pl.Owner != charlie.Hex() {
		t.Fatalf("Owner after chain of transfers: expected %s, got %s", charlie.Hex(), pl.Owner)
	}
	if pl.TransferCount != 2 {
		t.Fatalf("TransferCount: expected 2, got %d", pl.TransferCount)
	}

	// Verify getPayLinkRules returns empty/null
	result := rpcCall(t, node.rpcURL, "paylink_getPayLinkRules", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var rulesResp struct{ Rules json.RawMessage }
	json.Unmarshal(result, &rulesResp)
	if rulesResp.Rules != nil && string(rulesResp.Rules) != "null" {
		t.Fatalf("Rules should be empty for no-rules paylink, got %s", string(rulesResp.Rules))
	}
}
