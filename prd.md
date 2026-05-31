# PayLink -- Product Requirements Document

## Table of Contents

- [Product Vision](#product-vision)
- [Problem Statement](#problem-statement)
- [Target Users](#target-users)
- [User Journeys](#user-journeys)
- [Feature Requirements](#feature-requirements)
- [Acceptance Criteria](#acceptance-criteria)
- [Success Metrics](#success-metrics)
- [Constraints and Assumptions](#constraints-and-assumptions)
- [Risks and Mitigations](#risks-and-mitigations)
- [Out of Scope](#out-of-scope)
- [Dependencies](#dependencies)
- [Related Documents](#related-documents)

---

## Product Vision

PayLink turns payments into programmable, shareable links. Anyone can create a PayLink -- a URL, QR code, or NFT -- that represents a payment request. Payers click, scan, or tap to pay through whatever method they prefer (MPesa, card, bank transfer, crypto). The payment settles on-chain with cryptographic proof, giving both parties instant, tamper-proof confirmation.

No intermediary holds funds. No expensive licensing. No lock-in to a single payment provider. PayLink is the open protocol layer that makes any payment rail programmable.

**One-liner:** Pay anyone, anywhere, through any rail -- with a link.

---

## Problem Statement

### For Merchants and Receivers

1. **Fragmented payment acceptance.** A Kenyan merchant must integrate MPesa, bank APIs, and card gateways separately, each with different APIs, fee structures, and settlement timelines. Adding a new rail means a new integration.

2. **High transaction costs.** Card interchange fees (1.5-3.5%) and gateway charges make small transactions uneconomical. A KES 50 (~$0.40) purchase loses 10%+ to fees. Micropayments (tips, content unlocks, per-use services) are impossible.

3. **No programmability.** Existing rails are "fire and forget." Conditional payments (release on delivery, split among parties, auto-refund on timeout) require custom escrow logic built from scratch every time.

4. **Settlement delays.** Card settlements take 1-3 business days. Bank transfers can take longer. Merchants lack real-time confirmation that funds have arrived.

### For Payers

5. **Friction in cross-rail payments.** A payer with MPesa cannot easily pay a merchant who only accepts cards, or vice versa. Each payment method has its own flow, app, and authentication.

6. **No payment portability.** Payment requests are tied to specific rails. A payer cannot choose their preferred method when presented with a payment request.

7. **Trust gap in P2P and marketplace transactions.** No neutral, automated escrow for freelance work, marketplace purchases, or peer-to-peer trades.

### For Developers

8. **Integration burden.** Building payment acceptance requires understanding multiple APIs, handling webhooks differently per rail, managing reconciliation, and dealing with edge cases (timeouts, partial payments, retries) per provider.

9. **No open standard.** There is no universal, open protocol for creating and resolving payment requests across rails -- each provider has proprietary formats.

---

## Target Users

### Primary Personas

#### Merchant (Small-Medium Business)

| Attribute | Detail |
|-----------|--------|
| **Who** | Shop owner, online seller, service provider in Kenya/East Africa |
| **Context** | Accepts payments from multiple sources (MPesa, cash, occasionally cards). Uses a smartphone. May have basic POS. |
| **Pain** | Managing multiple payment integrations, high card fees on small transactions, delayed settlement, no automated payment rules. |
| **Goal** | Single integration that accepts any payment method. Instant confirmation. Low fees. Programmable rules (installments, escrow). |
| **Technical skill** | Low to moderate. Needs simple UI. May have a developer for online store integration. |

#### Payer (Consumer)

| Attribute | Detail |
|-----------|--------|
| **Who** | Mobile-first consumer in East Africa, or global user with card/crypto |
| **Context** | Scans QR codes, clicks payment links in messages/social media. Primary payment method is MPesa or mobile wallet. |
| **Pain** | Cannot pay merchants who don't support their preferred method. No trust mechanism for P2P/marketplace transactions. |
| **Goal** | Scan/click and pay with whatever method they have. Instant receipt. Know that escrow protects them if needed. |
| **Technical skill** | Low. Expects WhatsApp-level simplicity. |

#### Platform Developer

| Attribute | Detail |
|-----------|--------|
| **Who** | Developer building an ecommerce platform, marketplace, invoicing tool, or SaaS product in Africa or globally |
| **Context** | Needs to add payment acceptance to their product. Evaluating Stripe, Flutterwave, DPO, or building custom. |
| **Pain** | Each payment rail is a separate integration. Reconciliation is manual. No built-in escrow or programmable payments. |
| **Goal** | Single API/SDK that handles all rails. Webhooks for settlement. Built-in escrow and split payments. Low integration effort. |
| **Technical skill** | High. Evaluates based on API quality, docs, SDKs, and developer experience. |

### Secondary Personas

#### Freelancer / Gig Worker
Needs to invoice clients and get paid via any method. Wants instant settlement and the ability to set payment terms (milestone-based escrow).

#### Content Creator
Wants to monetize content with micropayments (tips, pay-per-view). Current rails make <$1 payments uneconomical.

#### Validator (Network Participant)
Technical operator who runs a validator node, stakes PLN tokens, and earns fees. Motivated by staking yield and protocol governance.

---

## User Journeys

### Journey 1: Merchant Creates a PayLink

```
Merchant opens PayLink web app
  → Enters amount (KES 1,500), description ("Invoice #1001"), expiry (30 days)
  → Selects accepted rails: MPesa, Card
  → Clicks "Create PayLink"
  → System mints PayLink NFT on-chain
  → Merchant receives:
      - Shareable URL: https://pay.linkmint.io/PLK-20260401-0001
      - QR code image
      - PayLink URI: paylink://1.0/PLK-20260401-0001
  → Merchant shares link via WhatsApp/SMS/email to customer
  → Merchant dashboard shows PayLink status: CREATED
```

### Journey 2: Payer Pays via MPesa

```
Payer receives link via WhatsApp
  → Taps link → opens PayLink resolution page in browser
  → Sees: "Acme Store requests KES 1,500 for Invoice #1001"
  → Sees available payment methods: MPesa, Card
  → Selects MPesa → enters phone number (or auto-detected)
  → System triggers STK Push via Daraja
  → Payer receives M-Pesa prompt on phone → enters PIN
  → M-Pesa confirms → adapter creates proof → validators reach consensus
  → PayLink page updates in real-time: "Payment confirmed!"
  → Payer sees receipt with transaction ID
  → Merchant receives push notification: "Payment received for Invoice #1001"
  → Merchant dashboard updates: PayLink status → VERIFIED
```

### Journey 3: Payer Pays via Card

```
Payer opens PayLink URL
  → Selects "Card" payment method
  → Enters card details (or uses saved token)
  → System calls card gateway (Stripe/Visa DPS)
  → Card authorized → adapter creates proof → validators confirm
  → PayLink settles on-chain
  → Both parties notified
```

### Journey 4: Escrow Payment (Marketplace)

```
Buyer and Seller agree on terms via marketplace platform
  → Platform creates PayLink with escrow conditions:
      - Amount: KES 5,000
      - Condition: Release when buyer confirms delivery
      - Timeout: Auto-refund after 7 days if no confirmation
  → Buyer pays via MPesa
  → Funds acknowledged on-chain (PayLink in ESCROW state)
  → Seller delivers goods
  → Buyer clicks "Confirm Delivery" in marketplace
  → Escrow Engine releases payment → Seller receives funds via their preferred rail
  → If buyer doesn't confirm within 7 days → auto-refund triggered
```

### Journey 5: Developer Integrates PayLink

```
Developer signs up on developer portal
  → Gets API keys
  → Reads OpenAPI docs, copies code sample
  → Installs SDK: npm install @linkmint/sdk
  → Creates PayLink programmatically:
      const link = await paylink.create({ amount: 500, currency: 'KES', rails: ['mpesa'] })
  → Registers webhook for PayLinkSettled events
  → Tests in sandbox with simulated MPesa
  → Deploys to production
  → Receives real-time webhook when customers pay
```

### Journey 6: Micropayment (Content Unlock)

```
Content creator embeds PayLink button on blog post
  → Button: "Unlock full article for KES 10"
  → Reader clicks → PayLink resolution page
  → Pays KES 10 via MPesa (no card fee problem at this amount)
  → Payment confirmed in <2 seconds
  → Page unlocks content automatically via webhook callback
  → Creator sees KES 10 credited (minus ~KES 1 fee)
```

### Journey 7: P2P Transfer

```
Alice wants to send KES 500 to Bob
  → Alice creates PayLink: amount KES 500, receiver = Bob's wallet/phone
  → Alice pays via her preferred rail
  → PayLink settles → Bob receives notification
  → Bob can withdraw to MPesa, bank, or hold as token balance
```

---

## Feature Requirements

### Phase 1: MVP (2026-Q2)

#### F1.1: PayLink Creation
| ID | Requirement | Priority |
|----|------------|----------|
| F1.1.1 | Merchant can create a PayLink specifying amount, currency (KES), description, and expiry | P0 |
| F1.1.2 | System generates a unique PayLink ID (format: `PLK-YYYYMMDD-NNNN`) | P0 |
| F1.1.3 | System returns shareable URL, QR code, and PayLink URI | P0 |
| F1.1.4 | PayLink is minted as an NFT on the PayLink Chain (testnet) | P0 |
| F1.1.5 | Merchant can cancel an unused PayLink | P0 |
| F1.1.6 | Expired PayLinks are automatically marked as EXPIRED | P0 |

#### F1.2: PayLink Resolution and Payment (MPesa)
| ID | Requirement | Priority |
|----|------------|----------|
| F1.2.1 | Payer can open a PayLink URL and see payment details (amount, merchant name, expiry) | P0 |
| F1.2.2 | Payer can select MPesa and enter their phone number | P0 |
| F1.2.3 | System triggers M-Pesa STK Push to payer's phone | P0 |
| F1.2.4 | System receives Daraja callback and constructs cryptographic proof | P0 |
| F1.2.5 | Validator verifies proof and settles PayLink on-chain | P0 |
| F1.2.6 | Payer sees real-time status update (pending → confirmed) | P0 |
| F1.2.7 | Single-use PayLinks reject second payment attempts | P0 |

#### F1.3: Merchant Dashboard
| ID | Requirement | Priority |
|----|------------|----------|
| F1.3.1 | Merchant can view list of created PayLinks with status | P0 |
| F1.3.2 | Merchant can view payment details for settled PayLinks | P0 |
| F1.3.3 | Merchant receives push notification / SMS on payment settlement | P1 |
| F1.3.4 | Merchant can filter PayLinks by status (CREATED, VERIFIED, EXPIRED) | P1 |

#### F1.4: Authentication and Accounts
| ID | Requirement | Priority |
|----|------------|----------|
| F1.4.1 | Merchant can register with phone number and email | P0 |
| F1.4.2 | Authentication via OAuth 2.0 / JWT | P0 |
| F1.4.3 | Payer can pay without creating an account (guest checkout) | P0 |

#### F1.5: Basic Web UI
| ID | Requirement | Priority |
|----|------------|----------|
| F1.5.1 | Web app for merchants to create and manage PayLinks | P0 |
| F1.5.2 | Mobile-responsive PayLink resolution page for payers | P0 |
| F1.5.3 | QR code display and download for created PayLinks | P0 |

### Phase 2: Beta (2026-Q3)

#### F2.1: Multi-Rail Support
| ID | Requirement | Priority |
|----|------------|----------|
| F2.1.1 | Payer can pay via credit/debit card (Visa/Mastercard) | P0 |
| F2.1.2 | Payer can pay via crypto (USDC/USDT on Ethereum or Polygon) | P1 |
| F2.1.3 | Merchant can configure which rails to accept per PayLink | P0 |
| F2.1.4 | System falls back to alternative rail if primary fails | P2 |

#### F2.2: Escrow and Conditional Payments
| ID | Requirement | Priority |
|----|------------|----------|
| F2.2.1 | Creator can attach escrow conditions to a PayLink (delivery confirmation, time-lock) | P0 |
| F2.2.2 | Buyer can confirm delivery to release escrowed funds | P0 |
| F2.2.3 | Auto-refund triggers if conditions are not met within timeout | P0 |
| F2.2.4 | Multi-party split payments (e.g., marketplace takes 10%, seller gets 90%) | P1 |

#### F2.3: Developer Platform
| ID | Requirement | Priority |
|----|------------|----------|
| F2.3.1 | REST API with full CRUD for PayLinks and payments | P0 |
| F2.3.2 | Webhook registration for PayLink events | P0 |
| F2.3.3 | JavaScript SDK published on npm | P0 |
| F2.3.4 | Flutter SDK published on pub.dev | P1 |
| F2.3.5 | Sandbox environment with simulated rails (Daraja sandbox, test cards) | P0 |
| F2.3.6 | Developer portal with API docs (OpenAPI/Swagger), guides, and code samples | P0 |
| F2.3.7 | API key management (create, rotate, revoke) | P0 |

#### F2.4: Multi-Validator Network
| ID | Requirement | Priority |
|----|------------|----------|
| F2.4.1 | 3-5 validator nodes running Proof-of-Validation consensus | P0 |
| F2.4.2 | VRF-based committee selection per proof | P0 |
| F2.4.3 | Validators can stake PLN tokens to participate | P0 |
| F2.4.4 | Slashing for invalid attestations | P1 |

#### F2.5: PLN Token
| ID | Requirement | Priority |
|----|------------|----------|
| F2.5.1 | PLN ERC-20 token deployed on PayLink Chain | P0 |
| F2.5.2 | Staking contract for validator participation | P0 |
| F2.5.3 | Transaction fee collection and distribution (70% validators, 20% treasury, 10% burn) | P1 |

#### F2.6: Compliance
| ID | Requirement | Priority |
|----|------------|----------|
| F2.6.1 | KYC verification for merchants (document upload, identity check) | P0 |
| F2.6.2 | Transaction monitoring with automatic flagging of suspicious activity | P0 |
| F2.6.3 | Threshold-based holds (e.g., KYC required for cumulative >KES 150,000) | P1 |

#### F2.7: Notifications
| ID | Requirement | Priority |
|----|------------|----------|
| F2.7.1 | SMS notification on payment settlement | P0 |
| F2.7.2 | Email notification on payment settlement | P1 |
| F2.7.3 | Webhook delivery with retry logic | P0 |

### Phase 3: Mainnet (2026-Q4+)

#### F3.1: Full Decentralization
| ID | Requirement | Priority |
|----|------------|----------|
| F3.1.1 | Open validator staking (anyone can join with minimum PLN stake) | P0 |
| F3.1.2 | 5+ active validators in production | P0 |
| F3.1.3 | DAO governance for protocol upgrades and parameter changes | P1 |

#### F3.2: Advanced Payment Features
| ID | Requirement | Priority |
|----|------------|----------|
| F3.2.1 | Recurring payments / subscriptions via multi-use PayLinks | P0 |
| F3.2.2 | Voucher/gift card PayLinks (transferable, redeemable) | P1 |
| F3.2.3 | Batch PayLinks (bulk creation for payroll, disbursements) | P1 |
| F3.2.4 | Micropayments (<KES 10) with batched on-chain settlement | P0 |

#### F3.3: Mobile App
| ID | Requirement | Priority |
|----|------------|----------|
| F3.3.1 | Flutter mobile app for merchants (create PayLinks, view dashboard) | P0 |
| F3.3.2 | QR code scanner for in-person payments | P0 |
| F3.3.3 | Push notifications for payment events | P0 |

#### F3.4: Additional Rails
| ID | Requirement | Priority |
|----|------------|----------|
| F3.4.1 | Bank transfer adapter (Kenya banks, ACH) | P0 |
| F3.4.2 | Additional mobile money (Airtel Money, T-Kash) | P1 |
| F3.4.3 | Cross-border payments via crypto stablecoins | P1 |

#### F3.5: Analytics and Reporting
| ID | Requirement | Priority |
|----|------------|----------|
| F3.5.1 | Merchant analytics dashboard (volume, revenue, payment method breakdown) | P0 |
| F3.5.2 | Exportable transaction reports (CSV/PDF) for accounting and tax | P0 |
| F3.5.3 | Real-time network dashboard (TPS, settlement latency, validator health) | P1 |

#### F3.6: Full SDK Suite
| ID | Requirement | Priority |
|----|------------|----------|
| F3.6.1 | Python SDK | P1 |
| F3.6.2 | Go SDK | P1 |
| F3.6.3 | Java SDK | P2 |
| F3.6.4 | CLI tool for PayLink management and testing | P1 |

---

## Acceptance Criteria

### AC1: Core Payment Flow (MVP)

```
GIVEN a merchant has created a PayLink for KES 1,500
WHEN a payer opens the PayLink URL
THEN the payer sees the amount (KES 1,500), merchant name, and expiry

GIVEN a payer selects MPesa on the PayLink page
WHEN the payer enters their phone number and submits
THEN an M-Pesa STK Push prompt appears on their phone within 5 seconds

GIVEN a payer completes the M-Pesa PIN entry
WHEN the Daraja callback confirms payment
THEN the PayLink status changes to VERIFIED within 3 seconds
AND the payer sees "Payment confirmed" on the resolution page
AND the merchant receives a notification

GIVEN a PayLink has been settled (VERIFIED)
WHEN another payer attempts to pay the same PayLink
THEN the system displays "This PayLink has already been used"
AND no payment is initiated
```

### AC2: PayLink Lifecycle

```
GIVEN a merchant creates a PayLink with 24-hour expiry
WHEN 24 hours pass without payment
THEN the PayLink status automatically changes to EXPIRED
AND the payer sees "This PayLink has expired" if they open the URL

GIVEN a merchant has a PayLink in CREATED status
WHEN the merchant cancels the PayLink
THEN the status changes to CANCELLED
AND the NFT is burned on-chain
AND the PayLink URL shows "This PayLink has been cancelled"
```

### AC3: Escrow (Phase 2)

```
GIVEN a PayLink with escrow condition "buyer confirms delivery"
WHEN the payer completes payment
THEN the PayLink enters ESCROW state (funds acknowledged, not released)
AND the seller is notified that payment is held in escrow

GIVEN a PayLink in ESCROW state
WHEN the buyer clicks "Confirm Delivery"
THEN the escrow releases and the seller's PayLink settles
AND both parties are notified

GIVEN a PayLink in ESCROW state with 7-day timeout
WHEN 7 days pass without buyer confirmation
THEN the escrow auto-refunds to the payer
AND both parties are notified
```

### AC4: Developer Integration (Phase 2)

```
GIVEN a developer with valid API keys
WHEN they call POST /v1/paylinks with valid parameters
THEN a PayLink is created and the response includes pl_id, URL, and QR code URL

GIVEN a developer has registered a webhook for PayLinkSettled
WHEN a PayLink they created is settled
THEN their webhook endpoint receives a POST with PayLink details within 5 seconds
AND the webhook includes a verifiable signature (HMAC)

GIVEN a developer is using the sandbox environment
WHEN they create a PayLink and simulate an MPesa payment
THEN the full flow executes (proof, validation, settlement) using test data
AND no real money is moved
```

### AC5: Performance

```
GIVEN normal operating conditions
WHEN a payment proof is submitted
THEN on-chain finality occurs within 100ms (internal)
AND end-to-end settlement (rail confirmation to PayLink VERIFIED) completes within 3 seconds

GIVEN the system is under load
WHEN 1,000 concurrent PayLinks are being settled
THEN the system maintains <2 second settlement latency at p95
AND no payments are lost or double-settled

GIVEN the API is under normal load
WHEN requests are made to any endpoint
THEN p99 response latency is <500ms
AND availability is 99.9% measured monthly
```

---

## Success Metrics

### Phase 1: MVP

| Metric | Target | Measurement |
|--------|--------|-------------|
| End-to-end demo | Complete flow: create PayLink -> MPesa payment -> on-chain settlement | Manual QA |
| Settlement latency | <3 seconds (rail callback to VERIFIED) | Prometheus |
| Payment success rate | >95% (of initiated payments that complete) | Analytics |
| System uptime | >99% during testing | Monitoring |

### Phase 2: Beta

| Metric | Target | Measurement |
|--------|--------|-------------|
| Active merchants | 10-50 pilot merchants | Database |
| Monthly PayLink volume | 1,000+ settled PayLinks | Analytics |
| Payment success rate | >98% across all rails | Analytics |
| Developer signups | 20+ on developer portal | Portal analytics |
| SDK downloads | 100+ npm/pub.dev installs | Package registry |
| Multi-validator consensus | 3-of-5 quorum succeeding >99% of attempts | Chain metrics |
| Settlement latency | <2 seconds at p95 | Prometheus |
| Merchant NPS | >40 | Survey |

### Phase 3: Mainnet

| Metric | Target | Measurement |
|--------|--------|-------------|
| Active merchants | 500+ | Database |
| Monthly volume | 50,000+ settled PayLinks | Analytics |
| Monthly transaction value | KES 50M+ | Ledger |
| Payment success rate | >99% | Analytics |
| Active validators | 5+ staking and participating | Chain state |
| Average fee per transaction | <1% of payment amount | Fee service |
| Developer integrations | 50+ apps using the API | API key count |
| System availability | 99.9% | Monitoring |
| Settlement latency | <1 second at p95 | Prometheus |

### North Star Metric

**Monthly settled PayLink volume** -- the total number of PayLinks that complete the full cycle (created -> paid -> proof validated -> settled on-chain) per month. This captures adoption by both merchants (creation) and payers (completion).

---

## Constraints and Assumptions

### Constraints

| Constraint | Impact |
|-----------|--------|
| **Non-custodial.** The system must never hold user funds. | All monetary flow is rail-to-receiver. No internal wallets holding fiat in Phase 1. Limits product features that require fund pooling. |
| **Kenya-first launch.** MVP is MPesa-only in Kenya. | All Phase 1 UX, compliance, and testing assumes Kenyan market. Currency is KES. Daraja API is the only rail. |
| **Single validator in Phase 1.** | No decentralization guarantees until Phase 2. Single point of failure acceptable for MVP/testing only. |
| **Safaricom Daraja dependency.** | STK Push availability, callback reliability, and sandbox quality are outside our control. Must handle Daraja downtime gracefully. |
| **EVM gas costs.** | On-chain operations (mint, settle) incur gas. Must choose a low-cost chain (Polygon, private chain) or batch settlements to keep per-PayLink cost negligible. |

### Assumptions

| Assumption | Risk if Wrong |
|-----------|--------------|
| Merchants will adopt link/QR-based payment collection. | Low adoption if merchants prefer existing POS/till solutions. Mitigate with compelling UX and lower fees. |
| Payers are comfortable with STK Push flow. | Already standard in Kenya (Lipa na MPesa). Low risk. |
| Non-custodial model is legally sufficient to avoid PSP licensing. | If regulators disagree, licensing cost and timeline increase significantly. Mitigate with early legal engagement. |
| Developers will integrate PayLink over existing gateways (Stripe, Flutterwave). | Must offer clear advantages: lower fees, programmability, multi-rail. Risk if DX is poor. |
| Validators will stake PLN tokens for yield. | Need attractive economics (5-10% APY). Risk if token has no market value. |
| Sub-100ms on-chain finality is achievable on chosen EVM chain. | Polygon block time is ~2 seconds. May need private chain or L2 for true sub-100ms. |

---

## Risks and Mitigations

| Risk | Severity | Likelihood | Mitigation |
|------|----------|-----------|-----------|
| **Regulatory challenge** -- Kenyan regulators classify PayLink as a PSP despite non-custodial model | High | Medium | Engage CBK counsel pre-launch. Prepare licensing application as contingency. Design system to operate under PSP license if required. |
| **Daraja unreliability** -- STK Push timeouts or callback failures degrade payment success rate | High | Medium | Implement retry logic, polling fallback (query transaction status), and graceful timeout UX. Monitor Daraja health separately. |
| **Low merchant adoption** -- Merchants don't see value over existing tools | High | Medium | Focus on concrete advantages: lower fees, instant settlement, QR simplicity. Pilot with friendly merchants. Iterate on feedback. |
| **Smart contract vulnerability** -- Bug in core contract leads to fund loss or protocol exploit | Critical | Low | Third-party audit, formal verification, bug bounty, emergency pause mechanism, UUPS proxy for patching. |
| **Validator centralization** -- Insufficient validators join, undermining decentralization claims | Medium | Medium | Attractive staking yield, low hardware requirements, progressive decentralization (start centralized, earn trust). |
| **Chain performance** -- Chosen EVM chain cannot meet latency/throughput targets | Medium | Low | Benchmark during Phase 1. Have fallback chains evaluated (Polygon, Arbitrum, private Geth). Design for chain portability. |
| **Token value collapse** -- PLN token has no market value, removing validator incentive | Medium | Medium | Don't rely on token value for MVP. Phase 1 operates without token. Ensure fee model is sustainable independent of token speculation. |
| **Competition** -- Established players (Flutterwave, DPO, Chipper Cash) add similar features | Medium | High | Differentiate on: open protocol (not proprietary), decentralization, programmability, and developer experience. Move fast on niche (micropayments, escrow). |

---

## Out of Scope

The following are explicitly **not** in scope for any current phase:

| Item | Reason |
|------|--------|
| **Fiat on/off-ramps** | Would make PayLink custodial. Users use external rails. |
| **Internal wallet balances (fiat)** | Non-custodial constraint. Users hold funds in their own MPesa/bank/crypto wallets. |
| **Lending / credit products** | Different regulatory domain. Not aligned with payment coordination mission. |
| **Card issuing** | Requires BIN sponsorship and PCI certification. Out of scope for protocol layer. |
| **White-label / self-hosted** | Focus on hosted protocol first. Self-hosted validator nodes are in scope, but not self-hosted platform. |
| **Non-payment NFT features** | PayLink NFTs are functional (payment authorization), not collectible/art. No marketplace for trading PayLink NFTs. |
| **Fiat-to-crypto exchange** | Requires money transmission license. Crypto adapter only accepts crypto payments, doesn't convert. |

---

## Dependencies

### External Dependencies

| Dependency | Owner | Risk Level | Notes |
|-----------|-------|-----------|-------|
| Safaricom Daraja API | Safaricom | High | Core payment rail for MVP. Dependent on API availability, sandbox quality, and production shortcode approval. |
| EVM-compatible blockchain | Polygon / Private chain | Medium | Smart contract deployment target. Must meet latency/cost requirements. |
| Card payment gateway | Stripe / Adyen | Low (Phase 2) | Standard integration. Multiple fallback options. |
| KYC provider | Jumio / Smile Identity | Low (Phase 2) | Identity verification for merchant onboarding. |
| Cloud infrastructure | AWS / GCP / Azure | Low | Standard Kubernetes hosting. Multi-cloud fallback possible. |
| OpenZeppelin contracts | OpenZeppelin | Low | Battle-tested Solidity libraries. Open source. |

### Internal Dependencies

| Dependency | Phase | Notes |
|-----------|-------|-------|
| Smart contracts deployed to testnet | Phase 1 | Blocks all on-chain operations |
| Daraja sandbox credentials | Phase 1 | Blocks MPesa integration testing |
| Production Daraja shortcode | Phase 1 (launch) | Requires Safaricom approval process (~4-8 weeks) |
| Domain and SSL certificates | Phase 1 | For API and PayLink resolution URLs |
| PLN token deployment | Phase 2 | Blocks staking, validator incentives, fee distribution |
| Multi-validator infrastructure | Phase 2 | Blocks decentralized consensus |

---

## Related Documents

| Document | Description |
|----------|-------------|
| [system.md](system.md) | System architecture, service definitions, deployment, monitoring |
| [spec.md](spec.md) | Technical specification: APIs, data models, Solidity contracts, consensus protocol |
| [CLAUDE.md](CLAUDE.md) | Developer instructions, repo structure, conventions, build commands |
| [deep-research-report.md](deep-research-report.md) | Original research document covering the full design rationale |
| [deep-research-report (1).md](<deep-research-report (1).md>) | Detailed specification research with contract code and microservice state machines |
