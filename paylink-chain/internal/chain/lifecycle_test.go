package chain

import (
	"crypto/ecdsa"
	"encoding/json"
	"testing"

	"github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/storage"
	"github.com/paylink/paylink-chain/internal/types"
)

// ── helpers (lc-prefixed to avoid clashes with executor_test.go) ──

type lcActor struct {
	key  *ecdsa.PrivateKey
	addr types.Address
}

func lcNewActor(t *testing.T) lcActor {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return lcActor{key: key, addr: crypto.PrivateKeyToAddress(key)}
}

func lcGenesis(admin lcActor) *types.GenesisConfig {
	return &types.GenesisConfig{
		ChainID:             "lifecycle-test",
		AdminAddress:        admin.addr,
		InitialSupply:       100_000_000,
		MaxSupply:           1_000_000_000,
		MinimumStake:        10_000,
		WithdrawalCooldown:  5,
		RequiredValidations: 3,
		BlockIntervalMs:     100,
		GenesisTimestamp:    1735689600,
		InitialBalances: []types.GenesisBalance{
			{Address: admin.addr, Balance: 100_000_000},
		},
	}
}

func lcSignedTx(t *testing.T, a lcActor, txType types.TxType, nonce uint64, payload interface{}) types.Transaction {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	tx := types.Transaction{Type: txType, From: a.addr, Nonce: nonce, Payload: data}
	tx.PubKey = crypto.MarshalPublicKey(&a.key.PublicKey)
	tx.Hash = crypto.SHA256Hash(tx.SignableBytes())
	sig, err := crypto.Sign(tx.Hash, a.key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}
	tx.Signature = sig
	return tx
}

// lcProduceBlock executes txs and commits a fully signed block, mirroring the
// producer's flow (execute → roots → hash → commit signature → CommitBlock).
func lcProduceBlock(t *testing.T, bc *Blockchain, exec *Executor, s *state.StateDB, proposer lcActor, txs []types.Transaction, timestamp int64) *types.Block {
	t.Helper()

	tip := bc.Tip()
	height := uint64(0)
	prevHash := types.ZeroHash
	if tip != nil {
		height = tip.Header.Height + 1
		prevHash = tip.Hash
	}

	receipts := exec.ExecuteBlock(txs, timestamp, height)
	for i := range receipts {
		if !receipts[i].Success {
			t.Fatalf("tx %d failed: %s", i, receipts[i].Error)
		}
		receipts[i].BlockHeight = height
		receipts[i].TxIndex = i
	}

	block := &types.Block{
		Header: types.BlockHeader{
			Height:       height,
			Timestamp:    timestamp,
			PreviousHash: prevHash,
			StateRoot:    s.ComputeStateRoot(),
			TxRoot:       ComputeTxRoot(txs),
			ProposerAddr: proposer.addr,
		},
		Transactions: txs,
	}
	block.Hash = crypto.SHA256Hash(block.HeaderBytes())
	sig, err := crypto.Sign(block.Hash, proposer.key)
	if err != nil {
		t.Fatalf("sign block: %v", err)
	}
	block.Commit = types.BlockCommit{
		ProposerAddr: proposer.addr,
		PublicKey:    crypto.MarshalPublicKey(&proposer.key.PublicKey),
		Signature:    sig,
	}

	if err := bc.CommitBlock(block, receipts); err != nil {
		t.Fatalf("CommitBlock %d: %v", height, err)
	}
	exec.FlushEvents(height)
	return block
}

// ── C2: restart replay ──

func TestReplay_RebuildsStateAfterRestart(t *testing.T) {
	admin := lcNewActor(t)
	bob := lcNewActor(t)
	genesis := lcGenesis(admin)

	dir := t.TempDir()
	store, err := storage.NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("badger: %v", err)
	}

	// "First run": produce two blocks of transfers.
	s1 := state.NewStateDB(genesis)
	bc1 := NewBlockchain(store, genesis)
	if err := bc1.Init(CreateGenesisBlock(genesis, s1)); err != nil {
		t.Fatalf("init: %v", err)
	}
	exec1 := NewExecutor(s1, nil)

	lcProduceBlock(t, bc1, exec1, s1, admin, []types.Transaction{
		lcSignedTx(t, admin, types.TxTransfer, 0, types.TransferPayload{To: bob.addr, Amount: 5000}),
	}, genesis.GenesisTimestamp+1)
	lcProduceBlock(t, bc1, exec1, s1, admin, []types.Transaction{
		lcSignedTx(t, admin, types.TxTransfer, 1, types.TransferPayload{To: bob.addr, Amount: 2500}),
		lcSignedTx(t, bob, types.TxTransfer, 0, types.TransferPayload{To: admin.addr, Amount: 500}),
	}, genesis.GenesisTimestamp+2)

	wantRoot := s1.ComputeStateRoot()
	wantBob := s1.GetBalance(bob.addr)

	// "Restart": fresh in-memory state over the same store; replay must rebuild it.
	s2 := state.NewStateDB(genesis)
	bc2 := NewBlockchain(store, genesis)
	if err := bc2.Init(CreateGenesisBlock(genesis, s2)); err != nil {
		t.Fatalf("re-init: %v", err)
	}
	if bc2.Height() != 2 {
		t.Fatalf("persisted height = %d, want 2", bc2.Height())
	}
	if err := Replay(bc2, NewExecutor(s2, nil), s2); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if got := s2.ComputeStateRoot(); got != wantRoot {
		t.Errorf("replayed state root = %s, want %s", got, wantRoot)
	}
	if got := s2.GetBalance(bob.addr); got != wantBob {
		t.Errorf("replayed bob balance = %d, want %d", got, wantBob)
	}
	if got := s2.GetNonce(admin.addr); got != 2 {
		t.Errorf("replayed admin nonce = %d, want 2", got)
	}

	store.Close()
}

func TestReplay_FailsOnUnverifiableHistory(t *testing.T) {
	admin := lcNewActor(t)
	genesis := lcGenesis(admin)

	dir := t.TempDir()
	store, err := storage.NewBadgerStore(dir)
	if err != nil {
		t.Fatalf("badger: %v", err)
	}
	defer store.Close()

	s := state.NewStateDB(genesis)
	bc := NewBlockchain(store, genesis)
	if err := bc.Init(CreateGenesisBlock(genesis, s)); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Forge a "legacy" block whose tx is unsigned (pre-enforcement data).
	bad := types.Transaction{Type: types.TxTransfer, From: admin.addr, Nonce: 0}
	bad.Payload, _ = json.Marshal(types.TransferPayload{To: types.Address{0x01}, Amount: 1})
	bad.Hash = crypto.SHA256Hash(bad.SignableBytes())
	tip := bc.Tip()
	block := &types.Block{
		Header: types.BlockHeader{
			Height:       1,
			Timestamp:    genesis.GenesisTimestamp + 1,
			PreviousHash: tip.Hash,
			StateRoot:    s.ComputeStateRoot(),
			TxRoot:       ComputeTxRoot([]types.Transaction{bad}),
			ProposerAddr: admin.addr,
		},
		Transactions: []types.Transaction{bad},
	}
	block.Hash = crypto.SHA256Hash(block.HeaderBytes())
	if err := bc.AddBlock(block); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}

	s2 := state.NewStateDB(genesis)
	if err := Replay(bc, NewExecutor(s2, nil), s2); err == nil {
		t.Fatal("replay of a chain with unsigned txs must fail, not silently corrupt state")
	}
}

// ── C3/C4: follower block processing ──

func TestProcessor_AppliesValidBlock(t *testing.T) {
	admin := lcNewActor(t)
	bob := lcNewActor(t)
	genesis := lcGenesis(admin)

	// Producer node (in-memory store).
	storeA, _ := storage.NewBadgerStore(t.TempDir())
	defer storeA.Close()
	sA := state.NewStateDB(genesis)
	bcA := NewBlockchain(storeA, genesis)
	if err := bcA.Init(CreateGenesisBlock(genesis, sA)); err != nil {
		t.Fatalf("init A: %v", err)
	}
	execA := NewExecutor(sA, nil)

	block := lcProduceBlock(t, bcA, execA, sA, admin, []types.Transaction{
		lcSignedTx(t, admin, types.TxTransfer, 0, types.TransferPayload{To: bob.addr, Amount: 1234}),
	}, genesis.GenesisTimestamp+1)

	// Follower node.
	storeB, _ := storage.NewBadgerStore(t.TempDir())
	defer storeB.Close()
	sB := state.NewStateDB(genesis)
	bcB := NewBlockchain(storeB, genesis)
	if err := bcB.Init(CreateGenesisBlock(genesis, sB)); err != nil {
		t.Fatalf("init B: %v", err)
	}
	proc := NewBlockProcessor(bcB, NewExecutor(sB, nil), sB, genesis)

	if err := proc.ProcessBlock(block); err != nil {
		t.Fatalf("ProcessBlock: %v", err)
	}
	if got := sB.GetBalance(bob.addr); got != 1234 {
		t.Errorf("follower bob balance = %d, want 1234", got)
	}
	if got := sB.ComputeStateRoot(); got != block.Header.StateRoot {
		t.Errorf("follower state root diverged from block header")
	}
	if bcB.Height() != 1 {
		t.Errorf("follower height = %d, want 1", bcB.Height())
	}

	// Replaying the same block is a no-op, not an error (gossip echo).
	if err := proc.ProcessBlock(block); err != nil {
		t.Errorf("re-processing same height should be ignored, got: %v", err)
	}
}

func TestProcessor_RejectsForgedBlocks(t *testing.T) {
	admin := lcNewActor(t)
	bob := lcNewActor(t)
	attacker := lcNewActor(t)
	genesis := lcGenesis(admin)

	storeA, _ := storage.NewBadgerStore(t.TempDir())
	defer storeA.Close()
	sA := state.NewStateDB(genesis)
	bcA := NewBlockchain(storeA, genesis)
	if err := bcA.Init(CreateGenesisBlock(genesis, sA)); err != nil {
		t.Fatalf("init A: %v", err)
	}
	block := lcProduceBlock(t, bcA, NewExecutor(sA, nil), sA, admin, []types.Transaction{
		lcSignedTx(t, admin, types.TxTransfer, 0, types.TransferPayload{To: bob.addr, Amount: 1}),
	}, genesis.GenesisTimestamp+1)

	freshFollower := func() (*BlockProcessor, *state.StateDB) {
		store, _ := storage.NewBadgerStore(t.TempDir())
		t.Cleanup(func() { store.Close() })
		s := state.NewStateDB(genesis)
		bc := NewBlockchain(store, genesis)
		if err := bc.Init(CreateGenesisBlock(genesis, s)); err != nil {
			t.Fatalf("init follower: %v", err)
		}
		return NewBlockProcessor(bc, NewExecutor(s, nil), s, genesis), s
	}

	clone := func() *types.Block {
		data, _ := json.Marshal(block)
		var b types.Block
		_ = json.Unmarshal(data, &b)
		return &b
	}

	cases := []struct {
		name   string
		mutate func(*types.Block)
	}{
		{"unknown proposer", func(b *types.Block) {
			b.Header.ProposerAddr = attacker.addr
			b.Hash = crypto.SHA256Hash(b.HeaderBytes())
			sig, _ := crypto.Sign(b.Hash, attacker.key)
			b.Commit = types.BlockCommit{
				ProposerAddr: attacker.addr,
				PublicKey:    crypto.MarshalPublicKey(&attacker.key.PublicKey),
				Signature:    sig,
			}
		}},
		{"commit key not deriving proposer", func(b *types.Block) {
			sig, _ := crypto.Sign(b.Hash, attacker.key)
			b.Commit.PublicKey = crypto.MarshalPublicKey(&attacker.key.PublicKey)
			b.Commit.Signature = sig
		}},
		{"missing commit signature", func(b *types.Block) {
			b.Commit = types.BlockCommit{ProposerAddr: b.Header.ProposerAddr}
		}},
		{"declared hash mismatch", func(b *types.Block) {
			b.Hash = crypto.SHA256Hash([]byte("not the header"))
		}},
		{"state root lie", func(b *types.Block) {
			b.Header.StateRoot = crypto.SHA256Hash([]byte("wrong root"))
			b.Hash = crypto.SHA256Hash(b.HeaderBytes())
			sig, _ := crypto.Sign(b.Hash, admin.key)
			b.Commit.Signature = sig
		}},
		{"injected tx", func(b *types.Block) {
			extra := lcSignedTx(t, attacker, types.TxTransfer, 0, types.TransferPayload{To: attacker.addr, Amount: 1})
			b.Transactions = append(b.Transactions, extra)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proc, s := freshFollower()
			forged := clone()
			tc.mutate(forged)
			if err := proc.ProcessBlock(forged); err == nil {
				t.Fatal("forged block must be rejected")
			}
			if got := s.GetBalance(bob.addr); got != 0 {
				t.Errorf("rejected block leaked state: bob balance = %d", got)
			}
		})
	}

	// The "state root lie" case must not even leave residue: verified above by balance.
}

// ── C5: fee distribution determinism across independent nodes ──

func TestSettlement_FeeDistributionDeterministic(t *testing.T) {
	admin := lcNewActor(t)
	v1, v2, v3 := lcNewActor(t), lcNewActor(t), lcNewActor(t)
	merchant := lcNewActor(t)
	genesis := lcGenesis(admin)

	plID := crypto.SHA256Hash([]byte("paylink-determinism"))
	proof := crypto.SHA256Hash([]byte("proof-determinism"))

	buildTxs := func() []types.Transaction {
		var txs []types.Transaction
		// Fund the validators and merchant from admin.
		txs = append(txs,
			lcSignedTx(t, admin, types.TxTransfer, 0, types.TransferPayload{To: v1.addr, Amount: 50_000}),
			lcSignedTx(t, admin, types.TxTransfer, 1, types.TransferPayload{To: v2.addr, Amount: 50_000}),
			lcSignedTx(t, admin, types.TxTransfer, 2, types.TransferPayload{To: v3.addr, Amount: 50_000}),
			lcSignedTx(t, admin, types.TxTransfer, 3, types.TransferPayload{To: merchant.addr, Amount: 1_000}),
			// Stake to become active validators (different stakes → different shares).
			lcSignedTx(t, v1, types.TxStake, 0, types.StakePayload{Amount: 30_000}),
			lcSignedTx(t, v2, types.TxStake, 0, types.StakePayload{Amount: 20_000}),
			lcSignedTx(t, v3, types.TxStake, 0, types.StakePayload{Amount: 10_000}),
			// Create a PayLink and settle it with 3 votes (RequiredValidations=3).
			lcSignedTx(t, merchant, types.TxCreatePayLink, 0, types.CreatePayLinkPayload{
				PayLinkID: plID,
				Receiver:  merchant.addr,
				Amount:    2_000_000, // 0.5% fee = 10_000 → 7000/2000/1000 split
				Expiry:    genesis.GenesisTimestamp + 1_000_000,
			}),
			lcSignedTx(t, v1, types.TxSubmitValidation, 1, types.SubmitValidationPayload{PayLinkID: plID, ProofHash: proof}),
			lcSignedTx(t, v2, types.TxSubmitValidation, 1, types.SubmitValidationPayload{PayLinkID: plID, ProofHash: proof}),
			lcSignedTx(t, v3, types.TxSubmitValidation, 1, types.SubmitValidationPayload{PayLinkID: plID, ProofHash: proof}),
		)
		return txs
	}

	run := func() (types.Hash, [3]uint64) {
		s := state.NewStateDB(genesis)
		exec := NewExecutor(s, nil)
		receipts := exec.ExecuteBlock(buildTxs(), genesis.GenesisTimestamp+10, 1)
		for i := range receipts {
			if !receipts[i].Success {
				t.Fatalf("tx %d failed: %s", i, receipts[i].Error)
			}
		}
		pl := s.GetPayLink(plID)
		if pl == nil || pl.Status != types.StatusVerified {
			t.Fatalf("paylink not settled: %+v", pl)
		}
		return s.ComputeStateRoot(), [3]uint64{
			s.GetBalance(v1.addr), s.GetBalance(v2.addr), s.GetBalance(v3.addr),
		}
	}

	root1, bals1 := run()
	// Many runs: map-iteration nondeterminism shows up probabilistically.
	for i := 0; i < 20; i++ {
		root, bals := run()
		if root != root1 {
			t.Fatalf("run %d: state root diverged: %s vs %s", i, root, root1)
		}
		if bals != bals1 {
			t.Fatalf("run %d: validator payouts diverged: %v vs %v", i, bals, bals1)
		}
	}
}

// ── H2: slashing evidence replay ──

func TestSubmitEvidence_ReplayRejected(t *testing.T) {
	admin := lcNewActor(t)
	offender := lcNewActor(t)
	reporter := lcNewActor(t)
	genesis := lcGenesis(admin)

	s := state.NewStateDB(genesis)
	exec := NewExecutor(s, nil)

	// Fund + stake the offender, fund the reporter.
	setup := []types.Transaction{
		lcSignedTx(t, admin, types.TxTransfer, 0, types.TransferPayload{To: offender.addr, Amount: 100_000}),
		lcSignedTx(t, admin, types.TxTransfer, 1, types.TransferPayload{To: reporter.addr, Amount: 1_000}),
		lcSignedTx(t, offender, types.TxStake, 0, types.StakePayload{Amount: 50_000}),
	}
	for i, r := range exec.ExecuteBlock(setup, genesis.GenesisTimestamp+1, 1) {
		if !r.Success {
			t.Fatalf("setup tx %d: %s", i, r.Error)
		}
	}

	// Double-sign evidence: two different block hashes signed by the offender.
	h1 := crypto.SHA256Hash([]byte("block-A"))
	h2 := crypto.SHA256Hash([]byte("block-B"))
	sig1, _ := crypto.Sign(h1, offender.key)
	sig2, _ := crypto.Sign(h2, offender.key)
	evidence, _ := json.Marshal(map[string]interface{}{
		"height":     uint64(42),
		"blockHash1": h1.Hex(),
		"blockHash2": h2.Hex(),
		"signature1": sig1,
		"signature2": sig2,
		"publicKey":  crypto.MarshalPublicKey(&offender.key.PublicKey),
	})

	submit := func(nonce uint64) TxReceipt {
		tx := lcSignedTx(t, reporter, types.TxSubmitEvidence, nonce, map[string]interface{}{
			"evidenceType": "double_sign",
			"validator":    offender.addr.Hex(),
			"data":         json.RawMessage(evidence),
		})
		receipts := exec.ExecuteBlock([]types.Transaction{tx}, genesis.GenesisTimestamp+2, 2)
		return receipts[0]
	}

	first := submit(0)
	if !first.Success {
		t.Fatalf("first evidence submission should slash: %s", first.Error)
	}
	stakeAfterFirst := s.GetValidator(offender.addr).StakedAmount

	second := submit(1)
	if second.Success {
		t.Fatal("replayed evidence must be rejected — repeated slashing for one offense")
	}
	if got := s.GetValidator(offender.addr).StakedAmount; got != stakeAfterFirst {
		t.Errorf("stake changed on replay: %d -> %d", stakeAfterFirst, got)
	}
}
