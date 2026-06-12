package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/paylink/paylink-chain/internal/chain"
	"github.com/paylink/paylink-chain/internal/consensus"
	pcrypto "github.com/paylink/paylink-chain/internal/crypto"
	"github.com/paylink/paylink-chain/internal/datastream"
	"github.com/paylink/paylink-chain/internal/events"
	"github.com/paylink/paylink-chain/internal/metrics"
	"github.com/paylink/paylink-chain/internal/p2p"
	"github.com/paylink/paylink-chain/internal/rpc"
	"github.com/paylink/paylink-chain/internal/state"
	"github.com/paylink/paylink-chain/internal/storage"
	"github.com/paylink/paylink-chain/internal/txpool"
	"github.com/paylink/paylink-chain/internal/types"
)

func main() {
	// Parse flags
	dataDir := flag.String("datadir", "./data", "Data directory")
	rpcAddr := flag.String("rpc", ":8545", "JSON-RPC listen address")
	genesisFile := flag.String("genesis", "", "Genesis config file (auto-generates if empty)")
	privKeyHex := flag.String("privkey", "", "Proposer private key (hex). Auto-generated if empty.")
	blockIntervalMs := flag.Int("block-interval", 1000, "Block production interval in milliseconds")
	wsEnabled := flag.Bool("ws", true, "Enable WebSocket event stream")
	wsMaxConns := flag.Int("ws-max-conns", 100, "Max WebSocket connections")
	wsOrigins := flag.String("ws-origins", "", "Comma-separated allowed WebSocket Origin patterns (empty = allow all, devnet only)")
	p2pEnabled := flag.Bool("p2p", false, "Enable P2P networking")
	p2pListen := flag.String("p2p-listen", "/ip4/0.0.0.0/tcp/9000", "P2P listen multiaddr")
	bootstrapPeers := flag.String("bootstrap-peers", "", "Comma-separated bootstrap peer multiaddrs")
	metricsEnabled := flag.Bool("metrics", false, "Enable Prometheus metrics endpoint")
	metricsAddr := flag.String("metrics-addr", ":9090", "Metrics listen address")
	rpcCORS := flag.String("rpc-cors", "*", "Access-Control-Allow-Origin for the RPC server (empty disables CORS)")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting PayLink Chain node...")

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data dir: %v", err)
	}

	// Load or generate private key
	var privKey []byte
	var proposerAddr types.Address
	if *privKeyHex != "" {
		var err error
		privKey, err = hex.DecodeString(*privKeyHex)
		if err != nil {
			log.Fatalf("Invalid private key: %v", err)
		}
		key, err := pcrypto.UnmarshalPrivateKey(privKey)
		if err != nil {
			log.Fatalf("Invalid private key: %v", err)
		}
		proposerAddr = pcrypto.PrivateKeyToAddress(key)
	} else {
		// Auto-generate key
		keyPath := filepath.Join(*dataDir, "node.key")
		privKey, proposerAddr = loadOrGenerateKey(keyPath)
	}
	log.Printf("Node address: %s", proposerAddr)

	// Load or generate genesis
	var genesis *types.GenesisConfig
	if *genesisFile != "" {
		var err error
		genesis, err = chain.LoadGenesis(*genesisFile)
		if err != nil {
			log.Fatalf("Failed to load genesis: %v", err)
		}
	} else {
		genesisPath := filepath.Join(*dataDir, "genesis.json")
		genesis = loadOrGenerateGenesis(genesisPath, proposerAddr)
	}
	log.Printf("Chain ID: %s", genesis.ChainID)

	// Initialize state
	stateDB := state.NewStateDB(genesis)
	log.Printf("Initial supply: %d", stateDB.TotalSupply())

	// Initialize storage
	dbPath := filepath.Join(*dataDir, "blocks")
	store, err := storage.NewBadgerStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to open storage: %v", err)
	}
	defer store.Close()

	// Initialize blockchain
	bc := chain.NewBlockchain(store, genesis)
	genesisBlock := chain.CreateGenesisBlock(genesis, stateDB)
	if err := bc.Init(genesisBlock); err != nil {
		log.Fatalf("Failed to init blockchain: %v", err)
	}
	log.Printf("Chain height: %d", bc.Height())

	// Initialize mempool
	mempool := txpool.NewMempool(10000)

	// Start services context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize event bus
	eventBus := events.NewBus(events.BusConfig{
		InternalBufferSize:   4096,
		SubscriberBufferSize: 256,
	})
	go eventBus.Start(ctx)

	// Initialize executor with event bus
	executor := chain.NewExecutor(stateDB, eventBus)

	// Rebuild state from persisted blocks (the store only persists blocks; state is
	// in-memory). Uses a bus-less executor so history isn't re-published as events.
	if err := chain.Replay(bc, chain.NewExecutor(stateDB, nil), stateDB); err != nil {
		log.Fatalf("Failed to rebuild state from chain data: %v\n"+
			"The data directory (%s) holds a chain this binary cannot verify "+
			"(e.g. produced before signature enforcement). For a devnet, wipe it and restart.",
			err, *dataDir)
	}

	// Initialize consensus. The canonical Phase 1 proposer is the genesis admin —
	// nodes whose key doesn't match run as followers and never produce blocks.
	validatorSet := consensus.NewValidatorSet(stateDB)
	pov := consensus.NewPoV(validatorSet, genesis.AdminAddress)

	// Follower-side block path: full validation + execution + atomic commit.
	processor := chain.NewBlockProcessor(bc, executor, stateDB, genesis)

	// Initialize block producer with event bus
	interval := time.Duration(*blockIntervalMs) * time.Millisecond
	producer := consensus.NewBlockProducer(bc, executor, stateDB, mempool, pov, interval, proposerAddr, privKey, eventBus)
	producer.SetCommitLock(&processor.CommitMu)

	// Initialize RPC server with optional WebSocket datastream
	rpcHandlers := rpc.NewHandlers(bc, stateDB, mempool)
	var rpcServer *rpc.Server
	if *wsEnabled {
		dsServer := datastream.NewServer(ctx, eventBus, datastream.ServerConfig{
			MaxConnections:   *wsMaxConns,
			SubscriberBuffer: 256,
			AllowedOrigins:   splitAndTrim(*wsOrigins, ","),
		})
		rpcServer = rpc.NewServer(rpcHandlers, *rpcAddr, dsServer.Handler())
	} else {
		rpcServer = rpc.NewServer(rpcHandlers, *rpcAddr)
	}
	rpcServer.SetCORSOrigin(*rpcCORS)

	// Initialize P2P networking (Phase 2)
	if *p2pEnabled {
		var boots []string
		if *bootstrapPeers != "" {
			for _, s := range splitAndTrim(*bootstrapPeers, ",") {
				if s != "" {
					boots = append(boots, s)
				}
			}
		}
		p2pCfg := p2p.Config{
			ListenAddrs:    []string{*p2pListen},
			BootstrapPeers: boots,
			MaxPeers:       50,
			ProtocolID:     "/linkm/1.0.0",
			ChainID:        genesis.ChainID,
			EnableDHT:      true,
		}
		p2pHost, err := p2p.NewLibP2PHost(p2pCfg, bc)
		if err != nil {
			log.Fatalf("Failed to create P2P host: %v", err)
		}

		// Register handlers: received blocks go through full validation + execution;
		// received txs must authenticate before touching the mempool.
		p2pHost.OnBlock(func(block *types.Block) {
			if err := processor.ProcessBlock(block); err != nil {
				log.Printf("P2P: rejected block %d: %v", block.Header.Height, err)
			}
		})
		p2pHost.OnTx(func(tx *types.Transaction) {
			if len(tx.Payload) > types.MaxTxPayloadBytes {
				return
			}
			if err := pcrypto.VerifyTx(tx); err != nil {
				return
			}
			if err := mempool.Add(tx); err != nil {
				log.Printf("P2P: tx %s not added: %v", tx.Hash, err)
			}
		})

		if err := p2pHost.Start(); err != nil {
			log.Fatalf("Failed to start P2P host: %v", err)
		}
		defer p2pHost.Stop()

		// Sync to head before producing blocks
		if err := p2pHost.SyncToHead(); err != nil {
			log.Printf("P2P sync warning: %v", err)
		}

		// Wire P2P broadcast into block producer
		producer.SetP2PHost(p2pHost)

		log.Printf("P2P networking enabled: %v", p2pHost.Multiaddrs())
	}

	// Start metrics server
	var metricsServer *metrics.Server
	if *metricsEnabled {
		metricsServer = metrics.NewServer(*metricsAddr)
		go func() {
			if err := metricsServer.Start(); err != nil {
				log.Printf("Metrics server error: %v", err)
			}
		}()
	}

	// Start block producer and RPC server
	go producer.Start(ctx)
	go func() {
		if err := rpcServer.Start(); err != nil {
			log.Fatalf("RPC server error: %v", err)
		}
	}()

	if *wsEnabled {
		log.Printf("PayLink Chain node running (RPC: %s, WS: %s/ws, block interval: %s)", *rpcAddr, *rpcAddr, interval)
	} else {
		log.Printf("PayLink Chain node running (RPC: %s, block interval: %s)", *rpcAddr, interval)
	}
	if *metricsEnabled {
		log.Printf("Metrics: %s/metrics", *metricsAddr)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := rpcServer.Stop(shutdownCtx); err != nil {
		log.Printf("RPC server shutdown error: %v", err)
	}
	if metricsServer != nil {
		if err := metricsServer.Stop(shutdownCtx); err != nil {
			log.Printf("Metrics server shutdown error: %v", err)
		}
	}

	log.Println("PayLink Chain node stopped")
}

func loadOrGenerateKey(path string) ([]byte, types.Address) {
	// Try to load existing key
	data, err := os.ReadFile(path)
	if err == nil {
		privKey, err := hex.DecodeString(string(data))
		if err == nil {
			key, err := pcrypto.UnmarshalPrivateKey(privKey)
			if err == nil {
				addr := pcrypto.PrivateKeyToAddress(key)
				log.Printf("Loaded existing key from %s", path)
				return privKey, addr
			}
		}
	}

	// Generate new key
	key, err := pcrypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}

	privKey := pcrypto.MarshalPrivateKey(key)
	addr := pcrypto.PrivateKeyToAddress(key)

	// Save key
	if err := os.WriteFile(path, []byte(hex.EncodeToString(privKey)), 0600); err != nil {
		log.Printf("Warning: failed to save key: %v", err)
	} else {
		log.Printf("Generated new key at %s", path)
	}

	return privKey, addr
}

func loadOrGenerateGenesis(path string, adminAddr types.Address) *types.GenesisConfig {
	// Try to load existing genesis
	genesis, err := chain.LoadGenesis(path)
	if err == nil {
		log.Printf("Loaded genesis from %s", path)
		return genesis
	}

	// Generate default genesis
	genesis = chain.DefaultGenesis(adminAddr)
	if err := chain.SaveGenesis(path, genesis); err != nil {
		log.Printf("Warning: failed to save genesis: %v", err)
	} else {
		fmt.Printf("Generated genesis at %s\n", path)
	}

	return genesis
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
