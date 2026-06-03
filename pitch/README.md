# LinkMint — Phase 2 pitch materials

Deliverables for the Phase 2 Innovator Showcase (Rupa Mall, Eldoret).

| File | What it is |
|------|------------|
| **`linkmint-pitch-deck.pdf`** | The pitch deck — **16 slides, 16:9**. This is the submission (PDF). |
| **`demo-video-script.md`** | Full script + storyboard + exact run commands for the 1–3 min demo video (you record the live system). |
| `deck.html` | Source of the deck (self-contained except the sibling `fonts/` + `assets/`). Edit this, then re-render. |
| `fonts/fonts.css` | Brand fonts (Fraunces, Inter, JetBrains Mono) base64-embedded — renders offline. |
| `assets/` | Real screenshots captured from the live app (`03-settled-crop.png` is the one used on slide 5). |
| `capture.mjs` | Playwright script that drove the live wizard + fired the settlement to capture the screenshots. |

## Regenerate the PDF after editing `deck.html`
```bash
google-chrome --headless=new --disable-gpu --no-pdf-header-footer --no-margins \
  --run-all-compositor-stages-before-draw --virtual-time-budget=12000 \
  --print-to-pdf=pitch/linkmint-pitch-deck.pdf "file://$PWD/pitch/deck.html"
```
Fallback renderer: `weasyprint pitch/deck.html pitch/linkmint-pitch-deck.pdf`.

## Slide order
1 Cover · 2 Problem · 3 Solution · 4 How it works · 5 Live demo (real screenshot) · 6 Traction ·
7 Architecture · 8 Market · 9 Business model · 10 Differentiation · **11 Impact & ROI** ·
**12 Where LinkMint goes next** (vertical expansion) · 13 Compliance · 14 GTM & roadmap ·
15 Team · 16 The ask & contact.

## One thing to fill in
- **Slide 15 (Team):** there is a bracketed prompt — *“[Add 1–2 lines on your background…]”*. Replace it with your real bio so the founder card is complete.

## Notes on accuracy (so claims hold up to a technical judge)
- Fee = **0.5% (50 bps), min KES 1**, split **70% validators / 20% treasury / 10% burned** — verified in `paylink-chain/internal/fee/calculator.go`.
- Compliance figures (Tier-1 ceiling **KES 50,000**, AML threshold **KES 150,000**) — verified in `compliance-risk` tests.
- Slide 5’s settlement screenshot is **real** — captured live; the chain tx hash on it is genuine.
- The e2e settlement test (`go test -tags=e2e ./adapters/mpesa/test/...`) passes against the live stack (~2.4s).
- "$800B+/yr mobile money in Sub-Saharan Africa" is attributed to GSMA (context, not our own projection).
