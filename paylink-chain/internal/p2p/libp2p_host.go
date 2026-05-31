package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/paylink/paylink-chain/internal/chain"
	"github.com/paylink/paylink-chain/internal/types"
)

const (
	blockTopic   = "/linkm/blocks/1.0.0"
	txTopic      = "/linkm/txs/1.0.0"
	syncProtocol = "/linkm/sync/1.0.0"
	syncBatchMax = 100
)

// LibP2PHost is a real P2P host using libp2p with GossipSub and Kademlia DHT.
type LibP2PHost struct {
	cfg        Config
	host       host.Host
	ps         *pubsub.PubSub
	blockTopic *pubsub.Topic
	txTopic    *pubsub.Topic
	blockSub   *pubsub.Subscription
	txSub      *pubsub.Subscription
	kadDHT     *dht.IpfsDHT
	blockchain *chain.Blockchain

	onBlock func(*types.Block)
	onTx    func(*types.Transaction)

	ctx    context.Context
	cancel context.CancelFunc

	// Deduplication cache for received messages
	mu       sync.Mutex
	seenMsgs map[types.Hash]struct{}
}

// NewLibP2PHost creates a new libp2p-based P2P host.
// The blockchain reference is needed for block sync serving.
func NewLibP2PHost(cfg Config, bc *chain.Blockchain) (*LibP2PHost, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Parse listen addresses
	var listenAddrs []ma.Multiaddr
	for _, addr := range cfg.ListenAddrs {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("invalid listen address %q: %w", addr, err)
		}
		listenAddrs = append(listenAddrs, maddr)
	}

	// Create libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.ConnectionManager(nil),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}

	lh := &LibP2PHost{
		cfg:        cfg,
		host:       h,
		blockchain: bc,
		ctx:        ctx,
		cancel:     cancel,
		seenMsgs:   make(map[types.Hash]struct{}),
	}

	return lh, nil
}

// Start initializes GossipSub, DHT, connects to bootstrap peers, and starts listening.
func (lh *LibP2PHost) Start() error {
	// Initialize GossipSub
	ps, err := pubsub.NewGossipSub(lh.ctx, lh.host)
	if err != nil {
		return fmt.Errorf("create gossipsub: %w", err)
	}
	lh.ps = ps

	// Join topics
	lh.blockTopic, err = ps.Join(blockTopic)
	if err != nil {
		return fmt.Errorf("join block topic: %w", err)
	}
	lh.txTopic, err = ps.Join(txTopic)
	if err != nil {
		return fmt.Errorf("join tx topic: %w", err)
	}

	// Subscribe
	lh.blockSub, err = lh.blockTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe blocks: %w", err)
	}
	lh.txSub, err = lh.txTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("subscribe txs: %w", err)
	}

	// Register sync protocol handler
	lh.host.SetStreamHandler(protocol.ID(syncProtocol), lh.handleSyncRequest)

	// Initialize DHT
	if lh.cfg.EnableDHT {
		kadDHT, err := dht.New(lh.ctx, lh.host, dht.Mode(dht.ModeAutoServer))
		if err != nil {
			return fmt.Errorf("create DHT: %w", err)
		}
		lh.kadDHT = kadDHT

		if err := kadDHT.Bootstrap(lh.ctx); err != nil {
			return fmt.Errorf("bootstrap DHT: %w", err)
		}
	}

	// Connect to bootstrap peers
	for _, addr := range lh.cfg.BootstrapPeers {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			log.Printf("P2P: invalid bootstrap addr %q: %v", addr, err)
			continue
		}
		pi, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Printf("P2P: invalid bootstrap peer info %q: %v", addr, err)
			continue
		}
		if err := lh.host.Connect(lh.ctx, *pi); err != nil {
			log.Printf("P2P: failed to connect to bootstrap %s: %v", pi.ID, err)
		} else {
			log.Printf("P2P: connected to bootstrap %s", pi.ID)
		}
	}

	// Start DHT peer discovery
	if lh.kadDHT != nil {
		go lh.discoverPeers()
	}

	// Start gossip message handlers
	go lh.handleBlocks()
	go lh.handleTxs()

	log.Printf("P2P: started, listening on %v, peer ID: %s", lh.host.Addrs(), lh.host.ID())
	return nil
}

// Stop gracefully shuts down the P2P host.
func (lh *LibP2PHost) Stop() error {
	lh.cancel()

	if lh.blockSub != nil {
		lh.blockSub.Cancel()
	}
	if lh.txSub != nil {
		lh.txSub.Cancel()
	}
	if lh.kadDHT != nil {
		lh.kadDHT.Close()
	}

	return lh.host.Close()
}

// BroadcastBlock publishes a block to the gossip network.
func (lh *LibP2PHost) BroadcastBlock(block *types.Block) error {
	if lh.blockTopic == nil {
		return nil
	}
	data, err := EncodeEnvelope(MsgTypeBlock, BlockMessage{Block: block})
	if err != nil {
		return err
	}

	// Mark as seen so we don't re-process our own message
	lh.markSeen(block.Hash)

	return lh.blockTopic.Publish(lh.ctx, data)
}

// BroadcastTx publishes a transaction to the gossip network.
func (lh *LibP2PHost) BroadcastTx(tx *types.Transaction) error {
	if lh.txTopic == nil {
		return nil
	}
	data, err := EncodeEnvelope(MsgTypeTx, TxMessage{Transaction: tx})
	if err != nil {
		return err
	}

	lh.markSeen(tx.Hash)

	return lh.txTopic.Publish(lh.ctx, data)
}

// PeerCount returns the number of connected peers.
func (lh *LibP2PHost) PeerCount() int {
	return len(lh.host.Network().Peers())
}

// OnBlock registers a callback for received blocks.
func (lh *LibP2PHost) OnBlock(handler func(*types.Block)) {
	lh.onBlock = handler
}

// OnTx registers a callback for received transactions.
func (lh *LibP2PHost) OnTx(handler func(*types.Transaction)) {
	lh.onTx = handler
}

// SyncToHead syncs the blockchain to the highest known height from connected peers.
func (lh *LibP2PHost) SyncToHead() error {
	peers := lh.host.Network().Peers()
	if len(peers) == 0 {
		log.Println("P2P sync: no peers, skipping")
		return nil
	}

	localHeight := lh.blockchain.Height()
	log.Printf("P2P sync: local height %d, checking %d peers", localHeight, len(peers))

	for _, peerID := range peers {
		if err := lh.syncFromPeer(peerID, localHeight); err != nil {
			log.Printf("P2P sync: failed from %s: %v", peerID, err)
			continue
		}
	}

	return nil
}

// Multiaddrs returns the full multiaddrs of this host (address + peer ID).
func (lh *LibP2PHost) Multiaddrs() []string {
	var addrs []string
	for _, addr := range lh.host.Addrs() {
		full := fmt.Sprintf("%s/p2p/%s", addr, lh.host.ID())
		addrs = append(addrs, full)
	}
	return addrs
}

// PeerID returns the host's peer ID string.
func (lh *LibP2PHost) PeerID() string {
	return lh.host.ID().String()
}

// ── Internal methods ──

func (lh *LibP2PHost) handleBlocks() {
	for {
		msg, err := lh.blockSub.Next(lh.ctx)
		if err != nil {
			return // context cancelled
		}
		// Skip our own messages
		if msg.ReceivedFrom == lh.host.ID() {
			continue
		}

		env, err := DecodeEnvelope(msg.Data)
		if err != nil {
			continue
		}
		if env.Type != MsgTypeBlock {
			continue
		}

		var bm BlockMessage
		if err := json.Unmarshal(env.Data, &bm); err != nil || bm.Block == nil {
			continue
		}

		// Deduplicate
		if lh.hasSeen(bm.Block.Hash) {
			continue
		}
		lh.markSeen(bm.Block.Hash)

		if lh.onBlock != nil {
			lh.onBlock(bm.Block)
		}
	}
}

func (lh *LibP2PHost) handleTxs() {
	for {
		msg, err := lh.txSub.Next(lh.ctx)
		if err != nil {
			return
		}
		if msg.ReceivedFrom == lh.host.ID() {
			continue
		}

		env, err := DecodeEnvelope(msg.Data)
		if err != nil {
			continue
		}
		if env.Type != MsgTypeTx {
			continue
		}

		var tm TxMessage
		if err := json.Unmarshal(env.Data, &tm); err != nil || tm.Transaction == nil {
			continue
		}

		if lh.hasSeen(tm.Transaction.Hash) {
			continue
		}
		lh.markSeen(tm.Transaction.Hash)

		if lh.onTx != nil {
			lh.onTx(tm.Transaction)
		}
	}
}

func (lh *LibP2PHost) handleSyncRequest(stream network.Stream) {
	defer stream.Close()

	var req SyncRequest
	if err := json.NewDecoder(stream).Decode(&req); err != nil {
		return
	}

	// Limit batch size
	toHeight := req.ToHeight
	if toHeight-req.FromHeight > syncBatchMax {
		toHeight = req.FromHeight + syncBatchMax
	}

	var blocks []*types.Block
	for h := req.FromHeight; h <= toHeight; h++ {
		block, err := lh.blockchain.GetBlockByHeight(h)
		if err != nil || block == nil {
			break
		}
		blocks = append(blocks, block)
	}

	resp := SyncResponse{Blocks: blocks}
	json.NewEncoder(stream).Encode(resp)
}

func (lh *LibP2PHost) syncFromPeer(peerID peer.ID, localHeight uint64) error {
	stream, err := lh.host.NewStream(lh.ctx, peerID, protocol.ID(syncProtocol))
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()

	// Request blocks starting from our height + 1
	req := SyncRequest{
		FromHeight: localHeight + 1,
		ToHeight:   localHeight + syncBatchMax + 1,
	}
	if err := json.NewEncoder(stream).Encode(req); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	// Signal end of write
	stream.CloseWrite()

	var resp SyncResponse
	data, err := io.ReadAll(stream)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	for _, block := range resp.Blocks {
		if block == nil {
			continue
		}
		if lh.onBlock != nil {
			lh.onBlock(block)
		}
	}

	log.Printf("P2P sync: received %d blocks from %s", len(resp.Blocks), peerID)
	return nil
}

func (lh *LibP2PHost) discoverPeers() {
	routingDiscovery := drouting.NewRoutingDiscovery(lh.kadDHT)
	rendezvous := "linkm-" + lh.cfg.ChainID

	// Advertise ourselves
	routingDiscovery.Advertise(lh.ctx, rendezvous)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lh.ctx.Done():
			return
		case <-ticker.C:
			peerCh, err := routingDiscovery.FindPeers(lh.ctx, rendezvous)
			if err != nil {
				continue
			}
			for p := range peerCh {
				if p.ID == lh.host.ID() || len(p.Addrs) == 0 {
					continue
				}
				if lh.host.Network().Connectedness(p.ID) != network.Connected {
					if len(lh.host.Network().Peers()) >= lh.cfg.MaxPeers {
						continue
					}
					lh.host.Connect(lh.ctx, p)
				}
			}
		}
	}
}

// Deduplication helpers

func (lh *LibP2PHost) markSeen(hash types.Hash) {
	lh.mu.Lock()
	defer lh.mu.Unlock()
	lh.seenMsgs[hash] = struct{}{}
	// Evict old entries when cache grows too large
	if len(lh.seenMsgs) > 10000 {
		lh.seenMsgs = make(map[types.Hash]struct{})
	}
}

func (lh *LibP2PHost) hasSeen(hash types.Hash) bool {
	lh.mu.Lock()
	defer lh.mu.Unlock()
	_, ok := lh.seenMsgs[hash]
	return ok
}
