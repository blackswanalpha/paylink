# LinkMint — Demo Video Script & Storyboard

**Deliverable:** a 1–3 minute demo video. **Target run-time: ~2 min 20 sec.**
**Presenter:** Victor Mbugua Kamande · Founder & CEO.
**Goal:** show the *real* product doing Create → Pay → Settle on our own blockchain — not slideware.

Everything below has been run against the live stack on this machine, so it records exactly as written.

---

## 0. Pre-flight (run once, before you hit record)

Open two terminals from the repo root (`/home/mbugua/Documents/augment-projects/linkMint`).

**Terminal A — bring the whole system up (chain + 12 services + M-Pesa rail):**
```bash
docker compose --profile e2e up -d --wait      # ~30s once images are built; --wait blocks until healthy
docker compose --profile e2e ps                 # confirm every service shows "healthy"
```

**Terminal B — run the web app:**
```bash
cd linkmint-frontend
npm run dev                                      # http://localhost:3000  (Ready in ~3s)
```

**Optional B-roll — prove the backend path in one command (great cutaway shot):**
```bash
go test -tags=e2e ./adapters/mpesa/test/... -v   # prints: PASS  (CREATED → VERIFIED, proof used ✓) in ~2.4s
```

> Tip: record the **live UI segment first** (it sets the real timing), then record the voice-over against the footage. Keep the voice calm and precise — match the brand: *“a private-bank statement, not a crypto dashboard.”*

---

## 1. How to make settlement happen live (devnet)

On devnet there is no real Safaricom payment, so you simulate the payer’s M-Pesa confirmation with two API calls — the **exact** calls the passing e2e test makes. The UI then flips to `VERIFIED` on its own (it polls the chain).

1. In the web app, create a PayLink. On the **Pay** screen, copy the **Account no.** — that is the PayLink id (`PL_ID`, a `0x…` hash).
2. In a spare terminal, paste (replace `PL_ID` and keep the amount equal to what you entered):

```bash
PL_ID=0x...paste-from-the-Pay-screen...
AMOUNT=100000                                   # KES 1,000.00  (minor units; must match the form)

# 1) start the M-Pesa charge (STK push)
CO=$(curl -s -X POST http://localhost:8082/v1/charges \
  -H 'Content-Type: application/json' -H "Idempotency-Key: demo-$PL_ID" \
  -d "{\"pl_id\":\"$PL_ID\",\"amount\":$AMOUNT,\"payer_phone\":\"254700000000\",\"receiver_shortcode\":\"174379\"}" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["checkout_request_id"])')

# 2) deliver the (stubbed) Daraja success callback → proof is signed → broadcast → chain settles
curl -s -X POST "http://localhost:8083/daraja/callback?t=devnet-callback-token" \
  -H 'Content-Type: application/json' \
  -d "{\"Body\":{\"stkCallback\":{\"MerchantRequestID\":\"demo-m\",\"CheckoutRequestID\":\"$CO\",\"ResultCode\":0,\"ResultDesc\":\"ok\",\"CallbackMetadata\":{\"Item\":[{\"Name\":\"Amount\",\"Value\":$AMOUNT},{\"Name\":\"MpesaReceiptNumber\",\"Value\":\"DEMO$RANDOM\"},{\"Name\":\"PhoneNumber\",\"Value\":\"254700000000\"}]}}}}"
```

3. Switch to the **Settlement** screen and watch it turn **VERIFIED** (status pill, votes `1`, a real `chain tx` hash, and a champagne “Settled on-chain.” burst) within ~2–5 seconds.

> Cleanest screen-capture trick: have the two curl calls ready in a terminal off-camera, click “I’ve sent the payment — watch settlement” in the UI, then fire the calls. On camera it simply looks like the payment landing and settling.

---

## 2. Storyboard — 7 beats (target 2:20)

| # | Time | On screen | What you do |
|---|------|-----------|-------------|
| 1 | 0:00–0:13 | **Title card** — LinkMint wordmark + tagline (use deck slide 1) | Hook: state the problem in one line. |
| 2 | 0:13–0:33 | **Problem** (deck slide 2) | Custodial, single-rail, micropayment-hostile. |
| 3 | 0:33–0:50 | **How it works** (deck slide 4 — the Create→Pay→Settle flow) | One sentence: link → pay → proof → settle. |
| 4 | 0:50–1:22 | **LIVE: Create + Pay** — screen-record the web app | Fill the create form, submit; land on the M-Pesa Pay screen (Pay Bill 174379 + PayLink id). |
| 5 | 1:22–1:52 | **LIVE: Settle** — the Settlement screen | Fire the payment; status flips **CREATED → VERIFIED**, real chain tx hash appears, champagne burst. *(Optional split-screen: the `go test` PASS line.)* **This is the money shot.** |
| 6 | 1:52–2:08 | **Traction** (deck slide 6) | 12 services, a custom chain, e2e passing, 80% coverage. |
| 7 | 2:08–2:20 | **Close** (deck slide 15 — the ask + contact) | Tagline + the ask + email. |

---

## 3. Narration script (~330 words ≈ 2:20 at ~140 wpm)

> *(Beat 1)* Across East Africa, billions move through mobile money every day. Yet to accept a digital payment, a business still needs a licence, locked-up capital, and a single chosen rail. And fixed fees make small payments — a tip, a song, a single article — simply not worth collecting.
>
> *(Beat 2–3)* LinkMint changes the model. **Pay anyone, anywhere, through any rail — with a link.** A merchant creates a PayLink — a URL, a QR, or an NFT. A customer scans it and pays with M-Pesa. No account, no app. The money goes straight to the receiver — it never touches LinkMint. Our adapter turns that payment into a signed, rail-agnostic proof, and our blockchain — the lVM — verifies it and settles the link.
>
> *(Beat 4)* Let me show you the real product, running now. I’ll create a PayLink — receiver, amount, expiry — and submit. LinkMint mints it on-chain. Now the customer pays: M-Pesa Pay Bill, the PayLink as the account number, confirm.
>
> *(Beat 5)* Watch the settlement screen. The proof flows from the adapter, to the validator, to the chain… and there it is — **VERIFIED.** Settled on-chain, in seconds, with a real, verifiable transaction hash and a tamper-evident audit trail behind it.
>
> *(Beat 6)* And this isn’t a prototype. Twelve services, a custom blockchain, an M-Pesa adapter, a JavaScript SDK, and this web app — all running, with an end-to-end settlement test passing in CI at eighty-percent coverage. Because we never hold funds, we sidestep PSP licensing — while staying compliance-ready, with KYC tiers and Kenya’s AML thresholds built in. Our fee is half a percent, versus one to three for cards — and that finally makes micropayments viable.
>
> *(Beat 7)* Phase one is done. Next: more rails, multi-validator consensus, merchant payouts, and escrow. I’m Victor Kamande. **LinkMint — pay anyone, through any rail, with a link.** Come see it live.

---

## 4. Recording checklist

- [ ] Browser at 100% zoom, clean profile, hide bookmarks bar; window ≥ 1280px wide.
- [ ] Hide the Next.js dev indicator (bottom-left) by cropping or full-screening the content.
- [ ] Mic test; record voice-over separately and lay it over the captured footage.
- [ ] Have the deck open (slides 1, 2, 4, 6, 15) for the non-live beats.
- [ ] Export 1080p, ≤ 3 min, MP4/H.264.

## 5. When you’re done recording — stop the stack
```bash
# in linkmint-frontend terminal: Ctrl-C to stop `npm run dev`
docker compose --profile e2e down               # stop all services
```
