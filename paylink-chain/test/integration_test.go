package test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/internal/chain"
	"github.com/paylink/paylink-chain/internal/consensus"
	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/rpc"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/storage"
	"github.com/paylink/paylink-chain/internal/txpool"
	"github.com/paylink/paylink-chain/internal/types"
)

// ── Test node harness ──

type testNode struct {
	state      *state.StateDB
	blockchain *chain.Blockchain
	executor   *chain.Executor
	mempool    *txpool.Mempool
	rpcServer  *rpc.Server
	producer   *consensus.BlockProducer
	cancel     context.CancelFunc
	rpcURL     string
	adminAddr  types.Address
	adminKey   []byte
	genesis    *types.GenesisConfig
	dataDir    string
	keys       map[types.Address]*ecdsa.PrivateKey // signing keys for every sending actor
}

// newActor generates a fresh P-256 key, registers it for signing, and returns its address.
// Every address that SENDS transactions must come from here (or be the admin).
func (n *testNode) newActor(t *testing.T) types.Address {
	t.Helper()
	key, err := pcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	addr := pcrypto.PrivateKeyToAddress(key)
	n.keys[addr] = key
	return addr
}

func startTestNode(t *testing.T) *testNode {
	t.Helper()

	// Generate admin key
	key, err := pcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	adminKey := pcrypto.MarshalPrivateKey(key)
	adminAddr := pcrypto.PrivateKeyToAddress(key)

	// Create genesis
	genesis := &types.GenesisConfig{
		ChainID:             "integration-test",
		AdminAddress:        adminAddr,
		InitialSupply:       100_000_000,
		MaxSupply:           1_000_000_000,
		MinimumStake:        10_000,
		WithdrawalCooldown:  5, // 5 seconds for fast testing
		RequiredValidations: 3,
		BlockIntervalMs:     100,
		InitialBalances: []types.GenesisBalance{
			{Address: adminAddr, Balance: 100_000_000},
		},
	}

	// Initialize state
	stateDB := state.NewStateDB(genesis)

	// Initialize storage (temp dir)
	dataDir, err := os.MkdirTemp("", "paylink-integration-*")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}

	store, err := storage.NewBadgerStore(dataDir)
	if err != nil {
		t.Fatalf("NewBadgerStore: %v", err)
	}

	// Initialize blockchain
	bc := chain.NewBlockchain(store, genesis)
	genesisBlock := chain.CreateGenesisBlock(genesis, stateDB)
	if err := bc.Init(genesisBlock); err != nil {
		t.Fatalf("Init blockchain: %v", err)
	}

	// Initialize mempool and executor
	mempool := txpool.NewMempool(1000)
	executor := chain.NewExecutor(stateDB, nil)

	// Initialize consensus
	validatorSet := consensus.NewValidatorSet(stateDB)
	pov := consensus.NewPoV(validatorSet, adminAddr)

	// Block producer: 100ms interval for fast testing
	producer := consensus.NewBlockProducer(
		bc, executor, stateDB, mempool, pov,
		100*time.Millisecond, adminAddr, adminKey,
	)

	// RPC server on random port
	rpcAddr := "127.0.0.1:0"
	handlers := rpc.NewHandlers(bc, stateDB, mempool)
	rpcServer := rpc.NewServer(handlers, rpcAddr)

	// Start block producer
	ctx, cancel := context.WithCancel(context.Background())
	go producer.Start(ctx)

	// Start RPC server in background and discover port
	go rpcServer.Start()

	// Wait for RPC server to be ready (try ports)
	// Since we used port 0, we need to pick a specific port instead
	// Let's use a fixed test port
	cancel() // Cancel first, we'll restart with fixed port

	// Restart with specific port
	port := findFreePort(t)
	rpcAddr = fmt.Sprintf("127.0.0.1:%d", port)
	rpcServer = rpc.NewServer(handlers, rpcAddr)

	ctx, cancel = context.WithCancel(context.Background())
	go producer.Start(ctx)
	go rpcServer.Start()

	rpcURL := fmt.Sprintf("http://%s/", rpcAddr)

	// Wait for RPC to be ready
	waitForRPC(t, rpcURL, 3*time.Second)

	node := &testNode{
		state:      stateDB,
		blockchain: bc,
		executor:   executor,
		mempool:    mempool,
		rpcServer:  rpcServer,
		producer:   producer,
		cancel:     cancel,
		rpcURL:     rpcURL,
		adminAddr:  adminAddr,
		adminKey:   adminKey,
		genesis:    genesis,
		dataDir:    dataDir,
		keys:       map[types.Address]*ecdsa.PrivateKey{adminAddr: key},
	}

	t.Cleanup(func() {
		cancel()
		shutCtx, sc := context.WithTimeout(context.Background(), 2*time.Second)
		defer sc()
		rpcServer.Stop(shutCtx)
		store.Close()
		os.RemoveAll(dataDir)
	})

	return node
}

func findFreePort(t *testing.T) int {
	t.Helper()
	// Use a listener on :0 to get a free port
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("findFreePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func waitForRPC(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Post(url, "application/json",
			bytes.NewReader([]byte(`{"jsonrpc":"2.0","method":"paylink_chainHeight","id":1}`)))
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("RPC server not ready after %s", timeout)
}

// ── JSON-RPC client helpers ──

func rpcCall(t *testing.T, url, method string, params interface{}) json.RawMessage {
	t.Helper()
	return rpcCallRaw(t, url, method, params, true)
}

func rpcCallExpectError(t *testing.T, url, method string, params interface{}) *rpc.RPCError {
	t.Helper()
	body := doRPC(t, url, method, params)

	var resp struct {
		Error *rpc.RPCError `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatalf("expected error for %s, got success", method)
	}
	return resp.Error
}

func rpcCallRaw(t *testing.T, url, method string, params interface{}, expectSuccess bool) json.RawMessage {
	t.Helper()
	body := doRPC(t, url, method, params)

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *rpc.RPCError   `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if expectSuccess && resp.Error != nil {
		t.Fatalf("%s failed: [%d] %s", method, resp.Error.Code, resp.Error.Message)
	}
	return resp.Result
}

func doRPC(t *testing.T, url, method string, params interface{}) []byte {
	t.Helper()
	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
	}

	reqBody, _ := json.Marshal(rpc.Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
		ID:      1,
	})

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST %s: %v", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return body
}

func sendTx(t *testing.T, node *testNode, txType types.TxType, from types.Address, nonce uint64, payload interface{}) string {
	t.Helper()
	key, ok := node.keys[from]
	if !ok {
		t.Fatalf("no signing key registered for sender %s (use node.newActor)", from.Hex())
	}
	data, _ := json.Marshal(payload)
	tx := types.Transaction{
		Type:    txType,
		From:    from,
		Nonce:   nonce,
		Payload: data,
	}
	tx.PubKey = pcrypto.MarshalPublicKey(&key.PublicKey)
	tx.Hash = pcrypto.SHA256Hash(tx.SignableBytes())
	sig, err := pcrypto.Sign(tx.Hash, key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	tx.Signature = sig

	result := rpcCall(t, node.rpcURL, "paylink_sendTransaction", tx)
	var sendResp rpc.SendTxResponse
	json.Unmarshal(result, &sendResp)
	return sendResp.TxHash
}

func waitForBlock(t *testing.T, node *testNode, minHeight uint64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result := rpcCall(t, node.rpcURL, "paylink_chainHeight", nil)
		var height uint64
		json.Unmarshal(result, &height)
		if height >= minHeight {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("chain did not reach height %d within %s", minHeight, timeout)
}

// waitForReceipt polls until a receipt for txHash is stored. A failed transaction
// never advances the chain on its own (the producer commits only successful txs),
// so waiting on its receipt is the way to observe its outcome.
func waitForReceipt(t *testing.T, node *testNode, txHash string, timeout time.Duration) rpc.TxReceiptResponse {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body := doRPC(t, node.rpcURL, "paylink_getTransactionReceipt", map[string]string{
			"hash": txHash,
		})
		var resp struct {
			Result json.RawMessage `json:"result"`
			Error  *rpc.RPCError   `json:"error"`
		}
		if err := json.Unmarshal(body, &resp); err == nil && resp.Error == nil && len(resp.Result) > 0 {
			var receipt rpc.TxReceiptResponse
			json.Unmarshal(resp.Result, &receipt)
			return receipt
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("receipt for tx %s not available within %s", txHash, timeout)
	return rpc.TxReceiptResponse{}
}

func getAccount(t *testing.T, node *testNode, addr types.Address) rpc.AccountResponse {
	t.Helper()
	result := rpcCall(t, node.rpcURL, "paylink_getAccount", map[string]string{
		"address": addr.Hex(),
	})
	var acc rpc.AccountResponse
	json.Unmarshal(result, &acc)
	return acc
}

func getPayLink(t *testing.T, node *testNode, id types.Hash) rpc.PayLinkResponse {
	t.Helper()
	result := rpcCall(t, node.rpcURL, "paylink_getPayLink", map[string]string{
		"id": id.Hex(),
	})
	var pl rpc.PayLinkResponse
	json.Unmarshal(result, &pl)
	return pl
}

func getValidator(t *testing.T, node *testNode, addr types.Address) rpc.ValidatorResponse {
	t.Helper()
	result := rpcCall(t, node.rpcURL, "paylink_getValidator", map[string]string{
		"address": addr.Hex(),
	})
	var v rpc.ValidatorResponse
	json.Unmarshal(result, &v)
	return v
}

// ═══════════════════════════════════════════════════════
// Integration Tests
// ═══════════════════════════════════════════════════════

// Test 1: Node startup + genesis verification
func TestIntegration_GenesisAndChainInfo(t *testing.T) {
	node := startTestNode(t)

	// Verify chain info via RPC
	result := rpcCall(t, node.rpcURL, "paylink_chainInfo", nil)
	var info rpc.ChainInfoResponse
	json.Unmarshal(result, &info)

	if info.ChainID != "integration-test" {
		t.Fatalf("ChainID: expected integration-test, got %s", info.ChainID)
	}
	if info.Height != 0 {
		t.Fatalf("Height: expected 0, got %d", info.Height)
	}

	// Verify genesis block
	height := uint64(0)
	blockResult := rpcCall(t, node.rpcURL, "paylink_getBlock", map[string]*uint64{
		"height": &height,
	})
	var block rpc.BlockResponse
	json.Unmarshal(blockResult, &block)

	if block.Height != 0 {
		t.Fatalf("Genesis block height: expected 0, got %d", block.Height)
	}
	if block.TxCount != 0 {
		t.Fatalf("Genesis block txs: expected 0, got %d", block.TxCount)
	}

	// Verify admin account funded
	acc := getAccount(t, node, node.adminAddr)
	if acc.Balance != 100_000_000 {
		t.Fatalf("Admin balance: expected 100000000, got %d", acc.Balance)
	}
	if acc.Nonce != 0 {
		t.Fatalf("Admin nonce: expected 0, got %d", acc.Nonce)
	}
}

// Test 2: PLN token transfer via RPC → block production → balance verification
func TestIntegration_TransferFlow(t *testing.T) {
	node := startTestNode(t)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	// Send transfer tx
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To:     bob,
		Amount: 5000,
	})

	// Wait for block to be produced
	waitForBlock(t, node, 1, 3*time.Second)

	// Verify balances
	bobAcc := getAccount(t, node, bob)
	if bobAcc.Balance != 5000 {
		t.Fatalf("Bob balance: expected 5000, got %d", bobAcc.Balance)
	}

	adminAcc := getAccount(t, node, node.adminAddr)
	if adminAcc.Balance != 100_000_000-5000 {
		t.Fatalf("Admin balance: expected %d, got %d", 100_000_000-5000, adminAcc.Balance)
	}
	if adminAcc.Nonce != 1 {
		t.Fatalf("Admin nonce: expected 1, got %d", adminAcc.Nonce)
	}
}

// Test 3: Multiple transfers in sequence
func TestIntegration_MultipleTransfers(t *testing.T) {
	node := startTestNode(t)

	alice := node.newActor(t)
	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	// Admin → Alice
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: alice, Amount: 10_000,
	})
	// Admin → Bob
	sendTx(t, node, types.TxTransfer, node.adminAddr, 1, types.TransferPayload{
		To: bob, Amount: 20_000,
	})

	waitForBlock(t, node, 1, 3*time.Second)

	// Alice → Bob
	sendTx(t, node, types.TxTransfer, alice, 0, types.TransferPayload{
		To: bob, Amount: 3_000,
	})

	waitForBlock(t, node, 2, 3*time.Second)

	aliceAcc := getAccount(t, node, alice)
	bobAcc := getAccount(t, node, bob)

	if aliceAcc.Balance != 7_000 {
		t.Fatalf("Alice balance: expected 7000, got %d", aliceAcc.Balance)
	}
	if bobAcc.Balance != 23_000 {
		t.Fatalf("Bob balance: expected 23000, got %d", bobAcc.Balance)
	}
}

// Test 4: Full PayLink lifecycle - create → validate → settle (VERIFIED)
func TestIntegration_PayLinkLifecycle(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-INTEG-001"))
	proofHash := pcrypto.SHA256Hash([]byte("mpesa-tx-ABC123"))

	// Setup: fund 3 validators and stake them
	v1 := node.newActor(t)
	v2 := node.newActor(t)
	v3 := node.newActor(t)

	nonce := uint64(0)
	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxTransfer, node.adminAddr, nonce, types.TransferPayload{
			To: v, Amount: 50_000,
		})
		nonce++
	}
	waitForBlock(t, node, 1, 3*time.Second)

	// Stake all 3 validators
	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxStake, v, 0, types.StakePayload{Amount: 20_000})
	}
	waitForBlock(t, node, 2, 3*time.Second)

	// Verify validators are active
	for _, v := range []types.Address{v1, v2, v3} {
		vi := getValidator(t, node, v)
		if !vi.IsActive {
			t.Fatalf("Validator %s should be active", v)
		}
		if vi.StakedAmount != 20_000 {
			t.Fatalf("Validator %s staked: expected 20000, got %d", v, vi.StakedAmount)
		}
	}

	// Create PayLink
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID:    plID,
		Receiver:     receiver,
		Amount:       1500,
		Expiry:       time.Now().Unix() + 86400,
		MetadataHash: pcrypto.SHA256Hash([]byte("orderId:INV1001")),
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// Verify PayLink created
	pl := getPayLink(t, node, plID)
	if pl.Status != "CREATED" {
		t.Fatalf("PayLink status: expected CREATED, got %s", pl.Status)
	}
	if pl.Amount != 1500 {
		t.Fatalf("PayLink amount: expected 1500, got %d", pl.Amount)
	}

	// Submit 3 validations (quorum = 3)
	sendTx(t, node, types.TxSubmitValidation, v1, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	sendTx(t, node, types.TxSubmitValidation, v2, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	sendTx(t, node, types.TxSubmitValidation, v3, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proofHash,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	// Verify PayLink is VERIFIED
	pl = getPayLink(t, node, plID)
	if pl.Status != "VERIFIED" {
		t.Fatalf("PayLink status: expected VERIFIED, got %s", pl.Status)
	}
	if pl.VoteCount != 3 {
		t.Fatalf("PayLink vote count: expected 3, got %d", pl.VoteCount)
	}

	// Verify proof is marked as used
	proofResult := rpcCall(t, node.rpcURL, "paylink_isProofUsed", map[string]string{
		"proofHash": proofHash.Hex(),
	})
	var isUsed bool
	json.Unmarshal(proofResult, &isUsed)
	if !isUsed {
		t.Fatal("Proof should be marked as used after settlement")
	}

	// Verify vote count via RPC
	voteResult := rpcCall(t, node.rpcURL, "paylink_getVoteCount", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var voteCount uint64
	json.Unmarshal(voteResult, &voteCount)
	if voteCount != 3 {
		t.Fatalf("Vote count via RPC: expected 3, got %d", voteCount)
	}
}

// Test 5: PayLink cancellation by creator
func TestIntegration_PayLinkCancellation(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-CANCEL-001"))

	// Create PayLink
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1000,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Cancel by creator
	sendTx(t, node, types.TxCancelPayLink, merchant, 1, types.CancelPayLinkPayload{
		PayLinkID: plID,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	pl := getPayLink(t, node, plID)
	if pl.Status != "CANCELLED" {
		t.Fatalf("PayLink status: expected CANCELLED, got %s", pl.Status)
	}
}

// Test 6: Admin fail PayLink (emergency)
func TestIntegration_PayLinkAdminFail(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	plID := pcrypto.SHA256Hash([]byte("PLK-FAIL-001"))

	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 2000,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Admin fails the PayLink
	sendTx(t, node, types.TxFailPayLink, node.adminAddr, 0, types.FailPayLinkPayload{
		PayLinkID: plID,
	})
	waitForBlock(t, node, 2, 3*time.Second)

	pl := getPayLink(t, node, plID)
	if pl.Status != "FAILED" {
		t.Fatalf("PayLink status: expected FAILED, got %s", pl.Status)
	}
}

// Test 7: Validator staking lifecycle - stake → unstake → complete withdrawal
func TestIntegration_StakingLifecycle(t *testing.T) {
	node := startTestNode(t)

	validator := node.newActor(t)

	// Fund validator
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: validator, Amount: 50_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Stake
	sendTx(t, node, types.TxStake, validator, 0, types.StakePayload{Amount: 20_000})
	waitForBlock(t, node, 2, 3*time.Second)

	vi := getValidator(t, node, validator)
	if !vi.IsActive {
		t.Fatal("Validator should be active after staking")
	}
	if vi.StakedAmount != 20_000 {
		t.Fatalf("Staked amount: expected 20000, got %d", vi.StakedAmount)
	}

	valAcc := getAccount(t, node, validator)
	if valAcc.Balance != 30_000 {
		t.Fatalf("Validator balance after stake: expected 30000, got %d", valAcc.Balance)
	}

	// Verify via getValidators
	validatorsResult := rpcCall(t, node.rpcURL, "paylink_getValidators", nil)
	var validators []rpc.ValidatorResponse
	json.Unmarshal(validatorsResult, &validators)
	if len(validators) != 1 {
		t.Fatalf("Validator count: expected 1, got %d", len(validators))
	}

	// Initiate unstake (full withdrawal)
	sendTx(t, node, types.TxInitiateUnstake, validator, 1, types.InitiateUnstakePayload{Amount: 20_000})
	waitForBlock(t, node, 3, 3*time.Second)

	vi = getValidator(t, node, validator)
	if vi.IsActive {
		t.Fatal("Validator should be deactivated after full unstake initiation")
	}
	if vi.PendingWithdrawal != 20_000 {
		t.Fatalf("Pending withdrawal: expected 20000, got %d", vi.PendingWithdrawal)
	}

	// Wait for cooldown (5 seconds in test genesis)
	time.Sleep(6 * time.Second)

	// Complete unstake
	sendTx(t, node, types.TxCompleteUnstake, validator, 2, types.CompleteUnstakePayload{})
	waitForBlock(t, node, 4, 3*time.Second)

	vi = getValidator(t, node, validator)
	if vi.StakedAmount != 0 {
		t.Fatalf("Staked amount after withdrawal: expected 0, got %d", vi.StakedAmount)
	}
	if vi.PendingWithdrawal != 0 {
		t.Fatalf("Pending withdrawal after complete: expected 0, got %d", vi.PendingWithdrawal)
	}

	valAcc = getAccount(t, node, validator)
	if valAcc.Balance != 50_000 {
		t.Fatalf("Validator balance after full unstake: expected 50000, got %d", valAcc.Balance)
	}
}

// Test 8: Slashing a validator
func TestIntegration_Slashing(t *testing.T) {
	node := startTestNode(t)

	validator := node.newActor(t)

	// Fund and stake
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: validator, Amount: 50_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	sendTx(t, node, types.TxStake, validator, 0, types.StakePayload{Amount: 20_000})
	waitForBlock(t, node, 2, 3*time.Second)

	// Admin slashes validator below minimum
	sendTx(t, node, types.TxSlash, node.adminAddr, 1, types.SlashPayload{
		Validator: validator, Amount: 15_000, Reason: "double-signing",
	})
	waitForBlock(t, node, 3, 3*time.Second)

	vi := getValidator(t, node, validator)
	if vi.StakedAmount != 5_000 {
		t.Fatalf("Staked after slash: expected 5000, got %d", vi.StakedAmount)
	}
	if vi.TotalSlashed != 15_000 {
		t.Fatalf("Total slashed: expected 15000, got %d", vi.TotalSlashed)
	}
	if vi.IsActive {
		t.Fatal("Validator should be deactivated after slash below minimum")
	}
}

// Test 9: Reward distribution
func TestIntegration_RewardDistribution(t *testing.T) {
	node := startTestNode(t)

	validator := node.newActor(t)

	// Fund and stake validator
	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: validator, Amount: 50_000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	sendTx(t, node, types.TxStake, validator, 0, types.StakePayload{Amount: 20_000})
	waitForBlock(t, node, 2, 3*time.Second)

	// Admin distributes reward
	sendTx(t, node, types.TxDistributeReward, node.adminAddr, 1, types.DistributeRewardPayload{
		Validator: validator, Amount: 2_500,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	vi := getValidator(t, node, validator)
	if vi.TotalRewards != 2_500 {
		t.Fatalf("Total rewards: expected 2500, got %d", vi.TotalRewards)
	}

	valAcc := getAccount(t, node, validator)
	// Balance = 50000 - 20000(staked) + 2500(reward) = 32500
	if valAcc.Balance != 32_500 {
		t.Fatalf("Validator balance: expected 32500, got %d", valAcc.Balance)
	}
}

// Test 10: Anti-replay protection
func TestIntegration_AntiReplay(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	v1 := node.newActor(t)

	proofHash := pcrypto.SHA256Hash([]byte("mpesa-tx-UNIQUE-001"))

	// Set required validations to 1 for easier testing
	// We use the genesis with required=3, but let's just fund 1 validator and set directly
	// Actually we can't change required validations via RPC. Instead, test with 3 validators.
	// For simplicity, let's use the state directly since genesis has required=3
	// OR: we set up genesis with required=1

	// Since our genesis has required=3, let's setup 3 validators
	v2 := node.newActor(t)
	v3 := node.newActor(t)

	nonce := uint64(0)
	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxTransfer, node.adminAddr, nonce, types.TransferPayload{
			To: v, Amount: 50_000,
		})
		nonce++
	}
	waitForBlock(t, node, 1, 3*time.Second)

	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxStake, v, 0, types.StakePayload{Amount: 20_000})
	}
	waitForBlock(t, node, 2, 3*time.Second)

	// Create and settle first PayLink
	plID1 := pcrypto.SHA256Hash([]byte("PLK-REPLAY-001"))
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID1, Receiver: receiver, Amount: 1000,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	for _, v := range []types.Address{v1, v2, v3} {
		sendTx(t, node, types.TxSubmitValidation, v, 1, types.SubmitValidationPayload{
			PayLinkID: plID1, ProofHash: proofHash,
		})
	}
	waitForBlock(t, node, 4, 3*time.Second)

	// Verify first PayLink settled
	pl1 := getPayLink(t, node, plID1)
	if pl1.Status != "VERIFIED" {
		t.Fatalf("PayLink 1 status: expected VERIFIED, got %s", pl1.Status)
	}

	// Create second PayLink and try to reuse same proof
	plID2 := pcrypto.SHA256Hash([]byte("PLK-REPLAY-002"))
	sendTx(t, node, types.TxCreatePayLink, merchant, 1, types.CreatePayLinkPayload{
		PayLinkID: plID2, Receiver: receiver, Amount: 2000,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 5, 3*time.Second)

	// Try to reuse proof - these should fail (rejected by executor).
	// Failed txs don't produce a block, so wait on their receipts instead.
	var replayHashes []string
	for _, v := range []types.Address{v1, v2, v3} {
		h := sendTx(t, node, types.TxSubmitValidation, v, 2, types.SubmitValidationPayload{
			PayLinkID: plID2, ProofHash: proofHash,
		})
		replayHashes = append(replayHashes, h)
	}
	for _, h := range replayHashes {
		r := waitForReceipt(t, node, h, 3*time.Second)
		if r.Success {
			t.Fatal("Reused proof validation should fail")
		}
	}

	// PayLink 2 should still be CREATED (not VERIFIED) since proof was rejected
	pl2 := getPayLink(t, node, plID2)
	if pl2.Status != "CREATED" {
		t.Fatalf("PayLink 2 status: expected CREATED (anti-replay), got %s", pl2.Status)
	}
}

// Test 11: Block production and chain continuity
func TestIntegration_BlockProduction(t *testing.T) {
	node := startTestNode(t)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	// Send several transactions to trigger multiple blocks
	for i := uint64(0); i < 5; i++ {
		sendTx(t, node, types.TxTransfer, node.adminAddr, i, types.TransferPayload{
			To: bob, Amount: 100,
		})
		// Small delay to spread across blocks
		time.Sleep(150 * time.Millisecond)
	}

	waitForBlock(t, node, 1, 5*time.Second)

	// Verify chain continuity: each block links to previous
	height := node.blockchain.Height()
	for h := uint64(1); h <= height; h++ {
		result := rpcCall(t, node.rpcURL, "paylink_getBlock", map[string]*uint64{"height": &h})
		var block rpc.BlockResponse
		json.Unmarshal(result, &block)

		prevH := h - 1
		prevResult := rpcCall(t, node.rpcURL, "paylink_getBlock", map[string]*uint64{"height": &prevH})
		var prevBlock rpc.BlockResponse
		json.Unmarshal(prevResult, &prevBlock)

		if block.PreviousHash != prevBlock.Hash {
			t.Fatalf("Block %d previous hash doesn't link to block %d", h, h-1)
		}
	}

	// Verify latest block via RPC
	latestResult := rpcCall(t, node.rpcURL, "paylink_getLatestBlock", nil)
	var latest rpc.BlockResponse
	json.Unmarshal(latestResult, &latest)

	if latest.Height != height {
		t.Fatalf("Latest block height: expected %d, got %d", height, latest.Height)
	}

	// Verify final balance
	bobAcc := getAccount(t, node, bob)
	if bobAcc.Balance != 500 {
		t.Fatalf("Bob balance: expected 500 (5*100), got %d", bobAcc.Balance)
	}
}

// Test 12: State root determinism
func TestIntegration_StateRootDeterminism(t *testing.T) {
	node := startTestNode(t)

	bob := types.HexToAddress("0x0000000000000000000000000000000000000b0b")

	sendTx(t, node, types.TxTransfer, node.adminAddr, 0, types.TransferPayload{
		To: bob, Amount: 1000,
	})
	waitForBlock(t, node, 1, 3*time.Second)

	// Get state root from block 1
	h1 := uint64(1)
	result1 := rpcCall(t, node.rpcURL, "paylink_getBlock", map[string]*uint64{"height": &h1})
	var block1 rpc.BlockResponse
	json.Unmarshal(result1, &block1)

	// State root should not be zero
	if block1.StateRoot == types.ZeroHash.Hex() {
		t.Fatal("State root should not be zero after transaction")
	}

	// State root should differ from genesis
	h0 := uint64(0)
	result0 := rpcCall(t, node.rpcURL, "paylink_getBlock", map[string]*uint64{"height": &h0})
	var block0 rpc.BlockResponse
	json.Unmarshal(result0, &block0)

	if block0.StateRoot == block1.StateRoot {
		t.Fatal("State root should change after transaction execution")
	}
}

// Test 13: Pending transactions (mempool query)
func TestIntegration_PendingTransactions(t *testing.T) {
	node := startTestNode(t)

	// Check empty mempool
	result := rpcCall(t, node.rpcURL, "paylink_pendingTransactions", nil)
	var pending []json.RawMessage
	json.Unmarshal(result, &pending)
	if len(pending) != 0 {
		t.Fatalf("Expected empty mempool, got %d txs", len(pending))
	}
}

// Test 14: RPC error handling
func TestIntegration_RPCErrors(t *testing.T) {
	node := startTestNode(t)

	// Unknown method
	rpcErr := rpcCallExpectError(t, node.rpcURL, "paylink_unknownMethod", nil)
	if rpcErr.Code != rpc.ErrCodeMethodNotFound {
		t.Fatalf("Expected method not found error, got code %d", rpcErr.Code)
	}

	// Get nonexistent PayLink
	rpcErr = rpcCallExpectError(t, node.rpcURL, "paylink_getPayLink", map[string]string{
		"id": types.ZeroHash.Hex(),
	})
	if rpcErr == nil {
		t.Fatal("Expected error for nonexistent PayLink")
	}

	// Get nonexistent validator
	rpcErr = rpcCallExpectError(t, node.rpcURL, "paylink_getValidator", map[string]string{
		"address": types.ZeroAddress.Hex(),
	})
	if rpcErr == nil {
		t.Fatal("Expected error for nonexistent validator")
	}
}

// Test 15: Proof hash consistency enforcement
func TestIntegration_ProofHashConsistency(t *testing.T) {
	node := startTestNode(t)

	merchant := node.newActor(t)
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000004")
	v1 := node.newActor(t)
	v2 := node.newActor(t)

	proof1 := pcrypto.SHA256Hash([]byte("proof-A"))
	proof2 := pcrypto.SHA256Hash([]byte("proof-B"))

	// Fund and stake 2 validators
	nonce := uint64(0)
	for _, v := range []types.Address{v1, v2} {
		sendTx(t, node, types.TxTransfer, node.adminAddr, nonce, types.TransferPayload{
			To: v, Amount: 50_000,
		})
		nonce++
	}
	waitForBlock(t, node, 1, 3*time.Second)

	for _, v := range []types.Address{v1, v2} {
		sendTx(t, node, types.TxStake, v, 0, types.StakePayload{Amount: 20_000})
	}
	waitForBlock(t, node, 2, 3*time.Second)

	// Create PayLink
	plID := pcrypto.SHA256Hash([]byte("PLK-CONSIST-001"))
	sendTx(t, node, types.TxCreatePayLink, merchant, 0, types.CreatePayLinkPayload{
		PayLinkID: plID, Receiver: receiver, Amount: 1000,
		Expiry: time.Now().Unix() + 86400,
	})
	waitForBlock(t, node, 3, 3*time.Second)

	// V1 submits proof1
	sendTx(t, node, types.TxSubmitValidation, v1, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proof1,
	})
	waitForBlock(t, node, 4, 3*time.Second)

	// V2 submits different proof2 - should be rejected.
	// A failed tx doesn't produce a block, so wait on its receipt instead.
	mismatchHash := sendTx(t, node, types.TxSubmitValidation, v2, 1, types.SubmitValidationPayload{
		PayLinkID: plID, ProofHash: proof2,
	})
	if r := waitForReceipt(t, node, mismatchHash, 3*time.Second); r.Success {
		t.Fatal("Mismatched proof validation should fail")
	}

	// Vote count should still be 1 (v2's vote rejected)
	voteResult := rpcCall(t, node.rpcURL, "paylink_getVoteCount", map[string]string{
		"paylinkId": plID.Hex(),
	})
	var voteCount uint64
	json.Unmarshal(voteResult, &voteCount)
	if voteCount != 1 {
		t.Fatalf("Vote count: expected 1 (mismatched proof rejected), got %d", voteCount)
	}
}
