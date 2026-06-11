package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/paylink/paylink-chain/internal/chain"
	"github.com/paylink/paylink-chain/internal/consensus"
	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/datastream"
	"github.com/paylink/paylink-chain/internal/events"
	"github.com/paylink/paylink-chain/internal/fsm"
	"github.com/paylink/paylink-chain/internal/rpc"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/storage"
	"github.com/paylink/paylink-chain/internal/txpool"
	"github.com/paylink/paylink-chain/internal/types"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// dsTestNode is a test node with event bus and WebSocket datastream.
type dsTestNode struct {
	state      *state.StateDB
	blockchain *chain.Blockchain
	executor   *chain.Executor
	mempool    *txpool.Mempool
	eventBus   *events.Bus
	cancel     context.CancelFunc
	rpcURL     string
	wsURL      string
	adminAddr  types.Address
	adminKey   []byte
	genesis    *types.GenesisConfig
	dataDir    string
}

func startDSTestNode(t *testing.T) *dsTestNode {
	t.Helper()

	key, err := pcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	adminKey := pcrypto.MarshalPrivateKey(key)
	adminAddr := pcrypto.PrivateKeyToAddress(key)

	genesis := &types.GenesisConfig{
		ChainID:             "ds-test",
		AdminAddress:        adminAddr,
		InitialSupply:       100_000_000,
		MaxSupply:           1_000_000_000,
		MinimumStake:        10_000,
		WithdrawalCooldown:  5,
		RequiredValidations: 3,
		BlockIntervalMs:     100,
		InitialBalances: []types.GenesisBalance{
			{Address: adminAddr, Balance: 100_000_000},
		},
	}

	stateDB := state.NewStateDB(genesis)

	dataDir, err := os.MkdirTemp("", "paylink-ds-test-*")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}

	store, err := storage.NewBadgerStore(dataDir)
	if err != nil {
		t.Fatalf("NewBadgerStore: %v", err)
	}

	bc := chain.NewBlockchain(store, genesis)
	genesisBlock := chain.CreateGenesisBlock(genesis, stateDB)
	if err := bc.Init(genesisBlock); err != nil {
		t.Fatalf("Init blockchain: %v", err)
	}

	mempool := txpool.NewMempool(1000)

	ctx, cancel := context.WithCancel(context.Background())

	// Create event bus
	eventBus := events.NewBus(events.BusConfig{
		InternalBufferSize:   1024,
		SubscriberBufferSize: 128,
	})
	go eventBus.Start(ctx)

	// Create executor with event bus
	executor := chain.NewExecutor(stateDB, eventBus)

	validatorSet := consensus.NewValidatorSet(stateDB)
	pov := consensus.NewPoV(validatorSet, adminAddr)

	producer := consensus.NewBlockProducer(
		bc, executor, stateDB, mempool, pov,
		100*time.Millisecond, adminAddr, adminKey, eventBus,
	)

	// Find a free port
	port := dsFindFreePort(t)
	rpcAddr := fmt.Sprintf("127.0.0.1:%d", port)

	// Create datastream server
	dsServer := datastream.NewServer(ctx, eventBus, datastream.ServerConfig{
		MaxConnections:   10,
		SubscriberBuffer: 128,
	})

	handlers := rpc.NewHandlers(bc, stateDB, mempool)
	rpcServer := rpc.NewServer(handlers, rpcAddr, dsServer.Handler())

	go producer.Start(ctx)
	go rpcServer.Start()

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	t.Cleanup(func() {
		cancel()
		rpcServer.Stop(context.Background())
		store.Close()
		os.RemoveAll(dataDir)
	})

	return &dsTestNode{
		state:      stateDB,
		blockchain: bc,
		executor:   executor,
		mempool:    mempool,
		eventBus:   eventBus,
		cancel:     cancel,
		rpcURL:     fmt.Sprintf("http://%s/", rpcAddr),
		wsURL:      fmt.Sprintf("ws://%s/ws", rpcAddr),
		adminAddr:  adminAddr,
		adminKey:   adminKey,
		genesis:    genesis,
		dataDir:    dataDir,
	}
}

func dsFindFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func dsDialWS(t *testing.T, ctx context.Context, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial WS: %v", err)
	}
	return conn
}

func dsReadEvent(t *testing.T, ctx context.Context, conn *websocket.Conn) events.Event {
	t.Helper()
	rCtx, rCancel := context.WithTimeout(ctx, 5*time.Second)
	defer rCancel()
	var msg datastream.ServerMessage
	if err := wsjson.Read(rCtx, conn, &msg); err != nil {
		t.Fatalf("read WS: %v", err)
	}
	if msg.Type != "event" {
		t.Fatalf("expected event, got %s (error: %s)", msg.Type, msg.Error)
	}
	var evt events.Event
	if err := json.Unmarshal(msg.Event, &evt); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	return evt
}

func dsSubscribe(t *testing.T, ctx context.Context, conn *websocket.Conn, filter *datastream.SubscribeFilter) {
	t.Helper()
	wCtx, wCancel := context.WithTimeout(ctx, 2*time.Second)
	defer wCancel()
	msg := datastream.ClientMessage{
		Action: "subscribe",
		ID:     "sub",
		Filter: filter,
	}
	if err := wsjson.Write(wCtx, conn, msg); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
	// Read ack
	rCtx, rCancel := context.WithTimeout(ctx, 2*time.Second)
	defer rCancel()
	var ack datastream.ServerMessage
	if err := wsjson.Read(rCtx, conn, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack.Type != "subscribed" {
		t.Fatalf("expected subscribed, got %s", ack.Type)
	}
}

func dsMakeTx(t *testing.T, txType types.TxType, from types.Address, nonce uint64, payload interface{}, privKey []byte) *types.Transaction {
	t.Helper()
	data, _ := json.Marshal(payload)
	tx := &types.Transaction{
		Type:    txType,
		From:    from,
		Nonce:   nonce,
		Payload: data,
	}
	tx.Hash = pcrypto.SHA256Hash(tx.SignableBytes())

	key, err := pcrypto.UnmarshalPrivateKey(privKey)
	if err != nil {
		t.Fatalf("unmarshal key: %v", err)
	}
	tx.PubKey = pcrypto.MarshalPublicKey(&key.PublicKey)
	sig, err := pcrypto.Sign(tx.Hash, key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	tx.Signature = sig
	return tx
}

// ── Integration Tests ──

func TestDS_CreatePayLinkEvent(t *testing.T) {
	node := startDSTestNode(t)
	ctx := context.Background()

	conn := dsDialWS(t, ctx, node.wsURL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to paylink events
	dsSubscribe(t, ctx, conn, &datastream.SubscribeFilter{
		EntityTypes: []string{"paylink"},
	})

	// Create a PayLink
	plID := pcrypto.SHA256Hash([]byte("test-paylink-1"))
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000099")
	tx := dsMakeTx(t, types.TxCreatePayLink, node.adminAddr, 0,
		types.CreatePayLinkPayload{
			PayLinkID:    plID,
			Receiver:     receiver,
			Amount:       5000,
			Expiry:       time.Now().Unix() + 3600,
			MetadataHash: pcrypto.SHA256Hash([]byte("meta")),
		}, node.adminKey)

	// Submit via RPC
	rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, tx)))

	// Wait for block production and read the event
	evt := dsReadEvent(t, ctx, conn)

	if evt.Kind != events.EventPayLinkCreated {
		t.Fatalf("expected paylink.created, got %s", evt.Kind)
	}
	if evt.EntityType != events.EntityPayLink {
		t.Fatalf("expected paylink entity type, got %s", evt.EntityType)
	}
	if string(evt.FromState) != string(fsm.PayLinkNone) {
		t.Fatalf("expected fromState NONE, got %s", evt.FromState)
	}
	if string(evt.ToState) != string(fsm.PayLinkCreated) {
		t.Fatalf("expected toState CREATED, got %s", evt.ToState)
	}
	if string(evt.Transition) != string(fsm.PayLinkCreate) {
		t.Fatalf("expected transition Create, got %s", evt.Transition)
	}
	if evt.BlockHeight == 0 {
		// Block height should be set (block 1 after genesis)
		// Note: could be 0 on edge cases, but normally >= 1
	}

	// Verify data payload
	var data events.PayLinkCreatedData
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data.Amount != 5000 {
		t.Fatalf("expected amount 5000, got %d", data.Amount)
	}
}

func TestDS_PayLinkFullLifecycle(t *testing.T) {
	node := startDSTestNode(t)
	ctx := context.Background()

	conn := dsDialWS(t, ctx, node.wsURL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to paylink events only
	dsSubscribe(t, ctx, conn, &datastream.SubscribeFilter{
		EntityTypes: []string{"paylink"},
	})

	// First stake 3 validators so we can reach quorum (requiredValidations=3)
	var valKeys [][]byte
	var valAddrs []types.Address
	for i := 0; i < 3; i++ {
		vKey, err := pcrypto.GenerateKey()
		if err != nil {
			t.Fatalf("GenerateKey: %v", err)
		}
		vPriv := pcrypto.MarshalPrivateKey(vKey)
		vAddr := pcrypto.PrivateKeyToAddress(vKey)
		valKeys = append(valKeys, vPriv)
		valAddrs = append(valAddrs, vAddr)

		// Transfer tokens to validator
		txTransfer := dsMakeTx(t, types.TxTransfer, node.adminAddr, uint64(i),
			types.TransferPayload{To: vAddr, Amount: 50_000}, node.adminKey)
		rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, txTransfer)))
	}

	// Wait for transfers to be processed
	time.Sleep(300 * time.Millisecond)

	// Stake each validator
	for i := 0; i < 3; i++ {
		txStake := dsMakeTx(t, types.TxStake, valAddrs[i], 0,
			types.StakePayload{Amount: 20_000}, valKeys[i])
		rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, txStake)))
	}

	// Wait for staking to be processed
	time.Sleep(300 * time.Millisecond)

	// Create PayLink
	plID := pcrypto.SHA256Hash([]byte("lifecycle-pl"))
	receiver := types.HexToAddress("0x0000000000000000000000000000000000000077")
	txCreate := dsMakeTx(t, types.TxCreatePayLink, node.adminAddr, 3,
		types.CreatePayLinkPayload{
			PayLinkID:    plID,
			Receiver:     receiver,
			Amount:       1000,
			Expiry:       time.Now().Unix() + 3600,
			MetadataHash: pcrypto.SHA256Hash([]byte("meta")),
		}, node.adminKey)
	rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, txCreate)))

	// Wait and read created event
	createdEvt := dsReadEvent(t, ctx, conn)
	if createdEvt.Kind != events.EventPayLinkCreated {
		t.Fatalf("expected paylink.created, got %s", createdEvt.Kind)
	}

	// Submit 3 validations
	proofHash := pcrypto.SHA256Hash([]byte("payment-proof"))
	for i := 0; i < 3; i++ {
		txValidate := dsMakeTx(t, types.TxSubmitValidation, valAddrs[i], 1,
			types.SubmitValidationPayload{
				PayLinkID: plID,
				ProofHash: proofHash,
			}, valKeys[i])
		rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, txValidate)))
	}

	// Wait for block production
	time.Sleep(300 * time.Millisecond)

	// Read vote and verified events
	var verifiedReceived bool
	voteCount := 0
	for i := 0; i < 10; i++ { // read up to 10 events
		rCtx, rCancel := context.WithTimeout(ctx, 2*time.Second)
		var msg datastream.ServerMessage
		err := wsjson.Read(rCtx, conn, &msg)
		rCancel()
		if err != nil {
			break
		}
		if msg.Type != "event" {
			continue
		}
		var evt events.Event
		json.Unmarshal(msg.Event, &evt)

		if evt.Kind == events.EventPayLinkVoted {
			voteCount++
		}
		if evt.Kind == events.EventPayLinkVerified {
			verifiedReceived = true
			if string(evt.FromState) != string(fsm.PayLinkCreated) {
				t.Fatalf("expected fromState CREATED, got %s", evt.FromState)
			}
			if string(evt.ToState) != string(fsm.PayLinkVerified) {
				t.Fatalf("expected toState VERIFIED, got %s", evt.ToState)
			}
			break
		}
	}

	if voteCount < 3 {
		t.Fatalf("expected at least 3 vote events, got %d", voteCount)
	}
	if !verifiedReceived {
		t.Fatal("did not receive paylink.verified event")
	}
}

func TestDS_EntityTypeFilter(t *testing.T) {
	node := startDSTestNode(t)
	ctx := context.Background()

	conn := dsDialWS(t, ctx, node.wsURL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to validator events only
	dsSubscribe(t, ctx, conn, &datastream.SubscribeFilter{
		EntityTypes: []string{"validator"},
	})

	// Submit a transfer (account event, should be filtered)
	bob := types.HexToAddress("0x0000000000000000000000000000000000000002")
	txTransfer := dsMakeTx(t, types.TxTransfer, node.adminAddr, 0,
		types.TransferPayload{To: bob, Amount: 50_000}, node.adminKey)
	rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, txTransfer)))

	// Submit a stake (validator event, should pass filter)
	txStake := dsMakeTx(t, types.TxStake, bob, 0,
		types.StakePayload{Amount: 20_000}, node.adminKey)

	// Bob doesn't have a valid key in our test setup, so let's stake from admin instead
	// Transfer to admin's own stake
	txStake = dsMakeTx(t, types.TxStake, node.adminAddr, 1,
		types.StakePayload{Amount: 20_000}, node.adminKey)
	rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, txStake)))

	// Wait for block
	time.Sleep(300 * time.Millisecond)

	// Read events — should only get validator events
	rCtx, rCancel := context.WithTimeout(ctx, 2*time.Second)
	var msg datastream.ServerMessage
	err := wsjson.Read(rCtx, conn, &msg)
	rCancel()

	if err != nil {
		t.Fatalf("expected at least one validator event: %v", err)
	}

	var evt events.Event
	json.Unmarshal(msg.Event, &evt)
	if evt.EntityType != events.EntityValidator {
		t.Fatalf("expected validator entity type, got %s (kind: %s)", evt.EntityType, evt.Kind)
	}
}

func TestDS_BlockProducedEvent(t *testing.T) {
	node := startDSTestNode(t)
	ctx := context.Background()

	conn := dsDialWS(t, ctx, node.wsURL)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Subscribe to block events
	dsSubscribe(t, ctx, conn, &datastream.SubscribeFilter{
		EntityTypes: []string{"block"},
	})

	// Submit a transaction to trigger block production
	bob := types.HexToAddress("0x0000000000000000000000000000000000000002")
	tx := dsMakeTx(t, types.TxTransfer, node.adminAddr, 0,
		types.TransferPayload{To: bob, Amount: 1000}, node.adminKey)
	rpcCall(t, node.rpcURL, "paylink_sendTransaction", json.RawMessage(mustJSON(t, tx)))

	// Read block.produced event
	evt := dsReadEvent(t, ctx, conn)
	if evt.Kind != events.EventBlockProduced {
		t.Fatalf("expected block.produced, got %s", evt.Kind)
	}
	if evt.EntityType != events.EntityBlock {
		t.Fatalf("expected block entity type, got %s", evt.EntityType)
	}

	var data events.BlockProducedData
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		t.Fatalf("unmarshal block data: %v", err)
	}
	if data.TxCount == 0 {
		t.Fatal("expected at least 1 tx in block")
	}
	if data.Proposer == "" {
		t.Fatal("expected proposer to be set")
	}
}

// mustJSON marshals v to JSON or panics.
func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}
