# LinkMint - CLAUDE.md

## Project Overview

LinkMint implements the **PayLink protocol** -- a decentralized payment coordination system using PayLinks as immutable payment authorizations. It links any payment rail (MPesa, card, bank, crypto) to on-chain logic. The protocol is **non-custodial**: senders pay directly via their chosen rail, and LinkMint only verifies proofs and finalizes settlement on-chain.

- **Current phase:** Phase 2 (2026-Q2) -- multi-validator VRF, fee model, P2P networking
- **Architecture reference:** [system.md](system.md)
- **Research context:** [deep-research-report.md](deep-research-report.md)

## Repository Structure

```
linkMint/
  CLAUDE.md                     # This file -- project instructions
  system.md                     # Comprehensive system design reference
  deep-research-report.md       # Original research document
  paylink-chain/                # lVM node (Go) -- full blockchain implementation
    cmd/paylinkd/               # Node binary entry point
    internal/
      chain/                    # Blockchain core, block storage, tx executor
      consensus/                # PoV consensus, VRF committee selection, block producer
      crypto/                   # ECDSA signing, hashing, ECVRF implementation
      config/                   # Node configuration
      datastream/               # WebSocket event streaming
      events/                   # Event bus (pub/sub)
      fee/                      # Fee calculator and distributor (PLN inflation)
      fsm/                      # PayLink and Validator finite state machines
      metrics/                  # Prometheus metrics and HTTP server
      p2p/                      # libp2p networking (GossipSub, DHT, block sync)
      rpc/                      # JSON-RPC server and handlers
      slashing/                 # Slashing evidence detection and processing
      state/                    # In-memory state (balances, validators, paylinks)
      storage/                  # BadgerDB persistent block storage
      txpool/                   # Transaction mempool
      types/                    # Core types (Block, Transaction, Address, Hash)
    test/                       # Integration tests
  linkmint-backend/
    paylink-service/            # PayLink CRUD
    payment-orchestrator/       # Payment lifecycle coordination
    proof-validator/            # Off-chain proof verification, validator broadcast
    escrow-manager/             # Conditional release/refund logic
    compliance-risk/            # KYC/AML, sanctions screening
    notification-service/       # SMS, email, push, webhook delivery
    api-gateway/                # Kong/custom gateway configuration
  adapters/
    mpesa/                      # Safaricom Daraja integration
    card/                       # Visa DPS / Stripe adapter
    bank/                       # Bank transfer adapter
    crypto/                     # On-chain payment adapter
  sdks/
    javascript/
    python/
    go/
    java/
    flutter/
  apps/
    web/                        # React frontend
    mobile/                     # Flutter mobile app
    cli/                        # Command-line tool
  infra/
    terraform/                  # Infrastructure as Code
    helm/                       # Helm charts per service
    docker/                     # Dockerfiles, docker-compose
  monitoring/
    grafana/                    # Dashboard configs
    prometheus/                 # Metrics and rules
    alerts/                     # Alert definitions
  docs/
    api/                        # OpenAPI/Swagger specs
    architecture/               # ADRs (Architecture Decision Records)
    runbooks/                   # Operational runbooks
  .github/
    workflows/                  # CI/CD (GitHub Actions)
```

## Tech Stack

- **Blockchain:** Custom chain (lVM -- Link Virtual Machine) written in Go. Native tx executor, PoV consensus, ECVRF committee selection, libp2p P2P networking, BadgerDB storage
- **Token:** PLN (native utility token for staking, fees, governance -- managed in-state, not an ERC-20)
- **Backend Services:** TypeScript/Node.js (Go for performance-critical paths), PostgreSQL (primary), Redis (cache), Kafka or AWS SQS (message queue)
- **Frontend:** React (web), Flutter (mobile)
- **SDKs:** JavaScript/TypeScript, Python, Go, Java, Flutter/Dart
- **API:** REST (JSON) with OpenAPI spec; gRPC for internal high-perf communication
- **Auth:** OAuth 2.0, JWT tokens, API keys for partners
- **Infrastructure:** Docker, Kubernetes (EKS/GKE/AKS), Terraform, Helm
- **CI/CD:** GitHub Actions
- **Monitoring:** Prometheus + Grafana (metrics), ELK/Loki (logs)

## Development Conventions

### Branching
`main` (stable) | `develop` (integration) | `feature/*` | `fix/*` | `release/*`

### Commits
Conventional Commits with scope: `feat(paylink-chain): add VRF committee selection`
Prefixes: `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`

### Code Style
- **Go:** `gofmt`, `golint`, standard project layout
- **TypeScript:** ESLint + Prettier, strict mode enabled, no `any` types
- **All services:** 12-factor app principles, environment variables for configuration

### Testing
- Chain: `cd paylink-chain && go test ./... -count=1`
- Backend services: Unit tests with mocks, integration tests with test containers (Jest or Vitest)
- Target: 80%+ code coverage across all services

### API Design
- RESTful, versioned: `/v1/paylinks`, `/v1/payments`
- Standard error format: `{"error": {"code": "PAYLINK_EXPIRED", "message": "...", "details": {}}}`

### Database
- All schema changes via numbered migrations
- Never modify production data directly

### Secrets
- Never commit secrets (`.env` in `.gitignore`)
- Use environment variables or vault references (AWS KMS, Azure Key Vault)

## Build, Test, Run Commands

```bash
# lVM Node (paylink-chain)
cd paylink-chain && go build ./...                      # Build
cd paylink-chain && go test ./... -count=1              # Test
cd paylink-chain && go run ./cmd/paylinkd               # Run node (single validator)
cd paylink-chain && go run ./cmd/paylinkd --p2p \
  --bootstrap-peers <multiaddr>                         # Run with P2P
cd paylink-chain && go run ./cmd/paylinkd --metrics     # Run with Prometheus metrics

# Backend Services (example: paylink-service)
cd linkmint-backend/paylink-service && npm install && npm run build
cd linkmint-backend/paylink-service && npm test
cd linkmint-backend/paylink-service && npm run dev

# Full Stack (local)
docker-compose up -d                    # Start all services
docker-compose down                     # Stop all services

# Infrastructure
cd infra/terraform && terraform plan
cd infra/terraform && terraform apply

# Linting
npm run lint                            # Lint all TS/JS code
```

## Key Architecture Decisions

1. **Non-custodial invariant:** The protocol NEVER holds user funds. Money flows sender-to-receiver via external rails. This is a legal and architectural requirement -- do not introduce custodial patterns.
2. **Custom chain, no EVM:** LinkMint runs its own chain (lVM) with a native Go transaction executor. There are no smart contracts, no Solidity, no EVM bytecode. All protocol logic (PayLink lifecycle, staking, fees, slashing) is implemented directly in Go.
3. **Proof-of-Validation consensus:** VRF-based committee selection with stake-weighted Algorand-style sortition. Quorum (3-of-5) on discrete payment proofs. Immediate finality.
4. **Rail-agnostic proof format:** All adapters normalize to `{pl_id, rail, tx_id, amount, timestamp, sender, receiver, proof_signature}`. Core services are rail-unaware.
5. **PLN inflation fee model:** 0.5% fee on settlement, split 70% minted to validators / 20% treasury / 10% burned. No upfront deposits.
6. **Double-entry ledger:** All monetary flows recorded with debit/credit entries for full audit trail and reconciliation.
7. **Anti-replay:** Proof hashes stored on-chain in the state. One transaction settles exactly one PayLink.
8. **P2P mesh (lVM network):** libp2p with GossipSub for block/tx propagation, Kademlia DHT for peer discovery, block sync protocol for new nodes.

## Common Workflows

- **Adding a payment rail adapter:** Create directory under `adapters/`, implement the adapter interface (receive callback, normalize to proof format, sign proof, broadcast). Register in Payment Orchestrator config.
- **Adding an API endpoint:** Define in OpenAPI spec (`docs/api/`), implement in the relevant service, add unit + integration tests, update SDK clients.
- **Adding a new tx type:** Add constant in `types/transaction.go`, add payload struct, add case in `chain/executor.go` switch, add event kinds in `events/event.go`, write tests.
- **Adding a new microservice:** Create directory under `linkmint-backend/`, include Dockerfile, add to `docker-compose.yml`, create Helm chart, add CI workflow in `.github/workflows/`.

## Node Flags (paylinkd)

```
--datadir          Data directory (default: ./data)
--rpc              JSON-RPC listen address (default: :8545)
--genesis          Genesis config file (auto-generates if empty)
--privkey          Proposer private key (hex, auto-generated if empty)
--block-interval   Block production interval in ms (default: 1000)
--ws               Enable WebSocket event stream (default: true)
--ws-max-conns     Max WebSocket connections (default: 100)
--p2p              Enable P2P networking (default: false)
--p2p-listen       P2P listen multiaddr (default: /ip4/0.0.0.0/tcp/9000)
--bootstrap-peers  Comma-separated bootstrap peer multiaddrs
--metrics          Enable Prometheus metrics endpoint (default: false)
--metrics-addr     Metrics listen address (default: :9090)
```
