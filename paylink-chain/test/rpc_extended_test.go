package test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/rpc"
	"github.com/paylink/paylink-chain/internal/types"
)

// ═══════════════════════════════════════════════════════
// Integration Tests for Extended RPC Methods
// ═══════════════════════════════════════════════════════

// Test: paylink_getTransactionReceipt
func TestRPC_GetTransactionReceipt(t *testing.T) {
	node := startTestNode(t)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	txHash := sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: bob, Amount: 1000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_getTransactionReceipt", map[string]string{
		"hash": txHash,
	})
	var receipt rpc.TxReceiptResponse
	json.Unmarshal(result, &receipt)

	if receipt.TxHash != txHash {
		t.Fatalf("Receipt txHash: expected %s, got %s", txHash, receipt.TxHash)
	}
	if !receipt.Success {
		t.Fatalf("Receipt should be success, error: %s", receipt.Error)
	}
	if receipt.BlockHeight != 1 {
		t.Fatalf("Receipt block height: expected 1, got %d", receipt.BlockHeight)
	}
}

// Test: paylink_getTransactionReceipt for failed tx
func TestRPC_GetTransactionReceiptFailed(t *testing.T) {
	node := startTestNode(t)

	// bob sends, so he needs a real key (valid signature, wrong nonce)
	bob := node.newActor(t)

	// Send tx with bad nonce (will fail and still get receipt stored).
	// A failed tx doesn't produce a block, so wait on its receipt instead.
	txHash := sendTx(t, node, types.TxTransfer, bob, 99, types.TransferPayload{
		To: node.adminAddr, Amount: 1000,
	})
	waitForReceipt(t, node, txHash, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_getTransactionReceipt", map[string]string{
		"hash": txHash,
	})
	var receipt rpc.TxReceiptResponse
	json.Unmarshal(result, &receipt)

	if receipt.Success {
		t.Fatal("Receipt should show failure")
	}
	if receipt.Error == "" {
		t.Fatal("Receipt error message should not be empty")
	}
}

// Test: paylink_getNonce
func TestRPC_GetNonce(t *testing.T) {
	node := startTestNode(t)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	// Initially nonce = 0
	result := rpcCall(t, node.rpcURL, "paylink_getNonce", map[string]string{
		"address": node.adminAddr.Hex(),
	})
	var nonce uint64
	json.Unmarshal(result, &nonce)
	if nonce != 0 {
		t.Fatalf("Initial nonce: expected 0, got %d", nonce)
	}

	// After a tx, nonce = 1
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: bob, Amount: 100,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	result = rpcCall(t, node.rpcURL, "paylink_getNonce", map[string]string{
		"address": node.adminAddr.Hex(),
	})
	json.Unmarshal(result, &nonce)
	if nonce != 1 {
		t.Fatalf("After tx nonce: expected 1, got %d", nonce)
	}
}

// Test: paylink_getPayLinksByCreator
func TestRPC_GetPayLinksByCreator(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")

	// Create 3 PayLinks from the same merchant
	for i := 0; i < 3; i++ {
		plID := pcrypto.SHA256Hash([]byte(fmt.Sprintf("PLK-CREATOR-%d", i)))
		sendTx(t, node, types.TxCreatePayLink, merchant, uint64(i), types.CreatePayLinkPayload{
			PayLinkID: plID, Receiver: receiver, Amount: uint64(1000 + i),
			Expiry: time.Now().Unix() + 86400,
		})
	}
	waitForBlock(t, node, 1, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_getPayLinksByCreator", map[string]interface{}{
		"creator": merchant.Hex(),
	})
	var paylinks []rpc.PayLinkResponse
	json.Unmarshal(result, &paylinks)

	if len(paylinks) != 3 {
		t.Fatalf("Expected 3 PayLinks by creator, got %d", len(paylinks))
	}
	for _, pl := range paylinks {
		if pl.Creator != merchant.Hex() {
			t.Fatalf("PayLink creator mismatch: expected %s, got %s", merchant.Hex(), pl.Creator)
		}
	}
}

// Test: paylink_getPayLinksByReceiver
func TestRPC_GetPayLinksByReceiver(t *testing.T) {
	node := startTestNode(t)

	merchant1 := node.newActor(t)
	merchant2 := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")

	// Two different merchants send to the same receiver
	plID1 := pcrypto.SHA256Hash([]byte("PLK-RCV-001"))
	plID2 := pcrypto.SHA256Hash([]byte("PLK-RCV-002"))

	sendTx(t, node, types.TxCreatePayLink, merchant1, 0, types.CreatePayLinkPayload{
		PayLinkID: plID1, Receiver: receiver, Amount: 1000,
		Expiry: time.Now().Unix() + 86400,
	})
	sendTx(t, node, types.TxCreatePayLink, merchant2, 0, types.CreatePayLinkPayload{
		PayLinkID: plID2, Receiver: receiver, Amount: 2000,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_getPayLinksByReceiver", map[string]interface{}{
		"receiver": receiver.Hex(),
	})
	var paylinks []rpc.PayLinkResponse
	json.Unmarshal(result, &paylinks)

	if len(paylinks) != 2 {
		t.Fatalf("Expected 2 PayLinks by receiver, got %d", len(paylinks))
	}
}

// Test: paylink_getPayLinksByStatus
func TestRPC_GetPayLinksByStatus(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")

	// Create 2 PayLinks
	plID1 := pcrypto.SHA256Hash([]byte("PLK-STATUS-001"))
	plID2 := pcrypto.SHA256Hash([]byte("PLK-STATUS-002"))

	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID1, Receiver: receiver, Amount: 1000,
		Expiry: time.Now().Unix() + 86400,
	})
	sendTx(t, node, types.TxCreatePayLink, merchant, 1, types.CreatePayLinkPayload{
		PayLinkID: plID2, Receiver: receiver, Amount: 2000,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Cancel one
	sendTx(t, node, types.TxCancelPayLink, merchant, 2, types.CancelPayLinkPayload{PayLinkID: plID1})
	waitForBlock(t, node, 2, 3*time.Second)

	// Query CREATED — should be 1
	result := rpcCall(t, node.rpcURL, "paylink_getPayLinksByStatus", map[string]interface{}{
		"status": "CREATED",
	})
	var created []rpc.PayLinkResponse
	json.Unmarshal(result, &created)
	if len(created) != 1 {
		t.Fatalf("Expected 1 CREATED PayLink, got %d", len(created))
	}

	// Query CANCELLED — should be 1
	result = rpcCall(t, node.rpcURL, "paylink_getPayLinksByStatus", map[string]interface{}{
		"status": "CANCELLED",
	})
	var cancelled []rpc.PayLinkResponse
	json.Unmarshal(result, &cancelled)
	if len(cancelled) != 1 {
		t.Fatalf("Expected 1 CANCELLED PayLink, got %d", len(cancelled))
	}
}

// Test: paylink_getPayLinksByStatus with invalid status
func TestRPC_GetPayLinksByStatusInvalid(t *testing.T) {
	node := startTestNode(t)

	rpcErr := rpcCallExpectError(t, node.rpcURL, "paylink_getPayLinksByStatus", map[string]interface{}{
		"status": "INVALID",
	})
	if rpcErr == nil {
		t.Fatal("Expected error for invalid status")
	}
}

// Test: paylink_getBlockRange
func TestRPC_GetBlockRange(t *testing.T) {
	node := startTestNode(t)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	// Generate a few blocks
	for i := uint64(0); i < 3; i++ {
		sendTx(t, node, types.TxTransfer, node.adminAddr, i, types.TransferPayload{
			To: bob, Amount: 100,
		})
		time.Sleep(150 * time.Millisecond)
	}
	waitForBlock(t, node, 3, 5*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_getBlockRange", map[string]uint64{
		"fromHeight": 0,
		"toHeight":   3,
	})
	var blocks []rpc.BlockResponse
	json.Unmarshal(result, &blocks)

	if len(blocks) < 3 {
		t.Fatalf("Expected at least 3 blocks, got %d", len(blocks))
	}

	// Verify ordering
	for i := 1; i < len(blocks); i++ {
		if blocks[i].Height != blocks[i-1].Height+1 {
			t.Fatalf("Block ordering broken at index %d", i)
		}
	}
}

// Test: paylink_getBlockTransactions
func TestRPC_GetBlockTransactions(t *testing.T) {
	node := startTestNode(t)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")
	alice := types.HexToAddress("0x000000000000000000000000000000000000a11c")

	// Send 2 txs to be in the same block
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{To: bob, Amount: 100})
	sendTx(t, node, types.TxTransfer, node.adminAddr, 1, types.TransferPayload{To: alice, Amount: 200})
	waitForBlock(t, node, 1, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_getBlockTransactions", map[string]uint64{
		"height": 1,
	})
	var txs []types.Transaction
	json.Unmarshal(result, &txs)

	if len(txs) != 2 {
		t.Fatalf("Expected 2 txs in block 1, got %d", len(txs))
	}

	// Genesis block should have 0 txs
	result = rpcCall(t, node.rpcURL, "paylink_getBlockTransactions", map[string]uint64{
		"height": 0,
	})
	json.Unmarshal(result, &txs)
	if len(txs) != 0 {
		t.Fatalf("Expected 0 txs in genesis, got %d", len(txs))
	}
}

// Test: paylink_hasVoted
func TestRPC_HasVoted(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	v1 := node.newActor(t)
	v2 := node.newActor(t)
	plID := pcrypto.SHA256Hash([]byte("PLK-HASVOTED-001"))
	proofHash := pcrypto.SHA256Hash([]byte("proof-hasvoted"))

	// Fund and stake v1
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{To: v1, Amount: 50_000})
	waitForBlock(t, node, 1, 3*time.Second)
	sendTx(t, node, types.TxStake, v1, 0, types.StakePayload{Amount: 20_000})
	waitForBlock(t, node, 2, 3*time.Second)

	// Create PayLink
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// v1 votes
	sendTx(t, node, types.TxSubmitValidation, v1, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	// v1 has voted
	result := rpcCall(t, node.rpcURL, "paylink_hasVoted", map[string]string{
		"paylinkId": plID.Hex(), "validator": v1.Hex(),
	})
	var hasVoted bool
	json.Unmarshal(result, &hasVoted)
	if !hasVoted {
		t.Fatal("v1 should have voted")
	}

	// v2 has NOT voted
	result = rpcCall(t, node.rpcURL, "paylink_hasVoted", map[string]string{
		"paylinkId": plID.Hex(), "validator": v2.Hex(),
	})
	json.Unmarshal(result, &hasVoted)
	if hasVoted {
		t.Fatal("v2 should NOT have voted")
	}
}

// Test: paylink_getVoters
func TestRPC_GetVoters(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	v1 := node.newActor(t)
	v2 := node.newActor(t)
	plID := pcrypto.SHA256Hash([]byte("PLK-VOTERS-001"))
	proofHash := pcrypto.SHA256Hash([]byte("proof-voters"))

	// Fund and stake 2 validators
	nonce := uint64(0)
	for _, v := range []types.Address{v1, v2} {
		sendTx(t, node, types.TxTransfer, node.adminAddr, nonce, types.TransferPayload{To: v, Amount: 50_000})
		nonce++
	}
	waitForBlock(t, node, 1, 3*time.Second)
	for _, v := range []types.Address{v1, v2} {
		sendTx(t, node, types.TxStake, v, 0, types.StakePayload{Amount: 20_000})
	}
	waitForBlock(t, node, 2, 3*time.Second)

	// Create PayLink and have both vote
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	sendTx(t, node, types.TxSubmitValidation, v1, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	sendTx(t, node, types.TxSubmitValidation, v2, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_getVoters", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var voters []string
	json.Unmarshal(result, &voters)

	if len(voters) != 2 {
		t.Fatalf("Expected 2 voters, got %d", len(voters))
	}
}

// Test: paylink_stakingStats
func TestRPC_StakingStats(t *testing.T) {
	node := startTestNode(t)

	v1 := node.newActor(t)
	v2 := node.newActor(t)

	// Fund and stake 2 validators with different amounts
	nonce := uint64(0)
	for _, v := range []types.Address{v1, v2} {
		sendTx(t, node, types.TxTransfer, node.adminAddr, nonce, types.TransferPayload{To: v, Amount: 50_000})
		nonce++
	}
	waitForBlock(t, node, 1, 3*time.Second)

	sendTx(t, node, types.TxStake, v1, 0, types.StakePayload{Amount: 20_000})
	sendTx(t, node, types.TxStake, v2, 0, types.StakePayload{Amount: 30_000})
	waitForBlock(t, node, 2, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_stakingStats", nil)
	var stats rpc.StakingStatsResponse
	json.Unmarshal(result, &stats)

	if stats.TotalStaked != 50_000 {
		t.Fatalf("Total staked: expected 50000, got %d", stats.TotalStaked)
	}
	if stats.ActiveValidatorCount != 2 {
		t.Fatalf("Active validators: expected 2, got %d", stats.ActiveValidatorCount)
	}
	if stats.TotalValidatorCount != 2 {
		t.Fatalf("Total validators: expected 2, got %d", stats.TotalValidatorCount)
	}
	if stats.MinimumStake != 10_000 {
		t.Fatalf("Minimum stake: expected 10000, got %d", stats.MinimumStake)
	}
}

// Test: paylink_tokenStats
func TestRPC_TokenStats(t *testing.T) {
	node := startTestNode(t)

	result := rpcCall(t, node.rpcURL, "paylink_tokenStats", nil)
	var stats rpc.TokenStatsResponse
	json.Unmarshal(result, &stats)

	if stats.TotalSupply != 100_000_000 {
		t.Fatalf("Total supply: expected 100000000, got %d", stats.TotalSupply)
	}
	if stats.MaxSupply != 1_000_000_000 {
		t.Fatalf("Max supply: expected 1000000000, got %d", stats.MaxSupply)
	}
}

// Test: paylink_chainStats
func TestRPC_ChainStats(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	// Create some state
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{To: bob, Amount: 1000})
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: pcrypto.SHA256Hash([]byte("PLK-STATS-001")),
		Receiver:  receiver, Amount: 500,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	result := rpcCall(t, node.rpcURL, "paylink_chainStats", nil)
	var stats rpc.ChainStatsResponse
	json.Unmarshal(result, &stats)

	if stats.ChainID != "integration-test" {
		t.Fatalf("ChainID: expected integration-test, got %s", stats.ChainID)
	}
	if stats.Height < 1 {
		t.Fatalf("Height should be >= 1, got %d", stats.Height)
	}
	if stats.TotalPayLinks != 1 {
		t.Fatalf("Total PayLinks: expected 1, got %d", stats.TotalPayLinks)
	}
	if stats.TotalAccounts < 2 {
		t.Fatalf("Total accounts should be >= 2, got %d", stats.TotalAccounts)
	}
	if stats.RequiredValidations != 3 {
		t.Fatalf("Required validations: expected 3, got %d", stats.RequiredValidations)
	}
}

// Test: paylink_nodeInfo
func TestRPC_NodeInfo(t *testing.T) {
	node := startTestNode(t)

	result := rpcCall(t, node.rpcURL, "paylink_nodeInfo", nil)
	var info rpc.NodeInfoResponse
	json.Unmarshal(result, &info)

	if info.ChainID != "integration-test" {
		t.Fatalf("ChainID: expected integration-test, got %s", info.ChainID)
	}
	if info.NodeVersion == "" {
		t.Fatal("NodeVersion should not be empty")
	}
	if info.AdminAddr != node.adminAddr.Hex() {
		t.Fatalf("Admin addr: expected %s, got %s", node.adminAddr.Hex(), info.AdminAddr)
	}
}

// Test: Pagination on getPayLinksByCreator
func TestRPC_PayLinkPagination(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")

	// Create 5 PayLinks
	for i := 0; i < 5; i++ {
		plID := pcrypto.SHA256Hash([]byte(fmt.Sprintf("PLK-PAGE-%d", i)))
		sendTx(t, node, types.TxCreatePayLink, merchant, uint64(i), types.CreatePayLinkPayload{
			PayLinkID: plID, Receiver: receiver, Amount: uint64(100 * (i + 1)),
			Expiry: time.Now().Unix() + 86400,
		})
	}
	waitForBlock(t, node, 1, 3*time.Second)

	// Get first 2
	result := rpcCall(t, node.rpcURL, "paylink_getPayLinksByCreator", map[string]interface{}{
		"creator": merchant.Hex(), "limit": 2, "offset": 0,
	})
	var page1 []rpc.PayLinkResponse
	json.Unmarshal(result, &page1)
	if len(page1) != 2 {
		t.Fatalf("Page 1: expected 2 items, got %d", len(page1))
	}

	// Get next 2
	result = rpcCall(t, node.rpcURL, "paylink_getPayLinksByCreator", map[string]interface{}{
		"creator": merchant.Hex(), "limit": 2, "offset": 2,
	})
	var page2 []rpc.PayLinkResponse
	json.Unmarshal(result, &page2)
	if len(page2) != 2 {
		t.Fatalf("Page 2: expected 2 items, got %d", len(page2))
	}

	// Get remaining
	result = rpcCall(t, node.rpcURL, "paylink_getPayLinksByCreator", map[string]interface{}{
		"creator": merchant.Hex(), "limit": 2, "offset": 4,
	})
	var page3 []rpc.PayLinkResponse
	json.Unmarshal(result, &page3)
	if len(page3) != 1 {
		t.Fatalf("Page 3: expected 1 item, got %d", len(page3))
	}
}
