# LinkMint — Phase 2 pitch materials

Deliverables for the Phase 2 Innovator Showcase (Rupa Mall, Eldoret).

| File | What it is |
|------|------------|
| **`linkmint-pitch-deck.pdf`** | The pitch deck — **17 slides, 16:9**. This is the submission (PDF). |
| **`linkmint-demo.mp4`** | The demo video — **~1:43, 1280×720, H.264 + narration**. A real screen-recording of the live app doing Create → Pay → Settle, with on-brand caption/title overlays and a Kenyan-accented neural voice-over. Submission-ready. |
| **`demo-video-script.md`** | Full narration script + storyboard + exact run commands, for recording your **own** voiced version of the demo if you prefer. |
| `deck.html` | Source of the deck (self-contained except the sibling `fonts/` + `assets/`). Edit this, then re-render. |
| `fonts/fonts.css` | Brand fonts (Fraunces, Inter, JetBrains Mono) base64-embedded — renders offline. |
| `assets/` | Real screenshots from the live app (`03-settled-crop.png` → slide 5) + `assets/market/` photos used on slide 8. |
| `capture.mjs` | Playwright script that drove the live wizard + fired the settlement to capture the **screenshots**. |
| `capture-video.mjs` | Playwright script that records the **demo video** (drives the wizard + fires the real settlement + draws the caption/title overlays; sizes each beat to its narration clip and logs `vo/marks.json`). |
| `vo_gen.py` | Generates the per-beat narration clips with `edge-tts` (voice `en-KE-ChilembaNeural`) → `vo/*.mp3` + `vo/durations.json`. |
| `mux.mjs` | Transcodes the webm + drops each narration clip under its beat (from `vo/marks.json`) → `linkmint-demo.mp4`. |
| `vo/` | Narration clips (`t*_*.mp3`), `durations.json`, and the per-run `marks.json` timing manifest. |
| `video/` | The raw silent `.webm` Playwright recording (source for the mux). |

## Regenerate the PDF after editing `deck.html`
```bash
google-chrome --headless=new --disable-gpu --no-pdf-header-footer --no-margins \
  --run-all-compositor-stages-before-draw --virtual-time-budget=12000 \
  --print-to-pdf=pitch/linkmint-pitch-deck.pdf "file://$PWD/pitch/deck.html"
```
Fallback renderer: `weasyprint pitch/deck.html pitch/linkmint-pitch-deck.pdf`.

## Regenerate the demo video (narrated)
Needs the live stack + frontend up (see `demo-video-script.md` §0 — `docker compose --profile e2e up -d --wait` and `npm run dev` in `linkmint-frontend/`), plus `edge-tts` (`pip install --user --break-system-packages edge-tts`). Then, from `pitch/`:
```bash
python3 vo_gen.py        # 1. narration -> vo/*.mp3 + vo/durations.json (edge-tts, en-KE voice)
node capture-video.mjs   # 2. records video/<hash>.webm + vo/marks.json (fires the real settlement)
node mux.mjs             # 3. trims the lead-in + drops each clip under its beat -> linkmint-demo.mp4
```
- Edit the narration text in `vo_gen.py` (change voice there too — `en-KE-AsiliaNeural` is the female Kenyan voice). Re-run all three steps after editing, since beat holds are sized from the clip durations.
- `mux.mjs` reads `vo/marks.json`, so the voice-over stays in sync even though each recording's live timing varies slightly.
- Prefer your own voice? Record your screen following `demo-video-script.md` instead, or lay your audio over the silent webm.

## Slide order
1 Cover · 2 Problem · 3 Solution · 4 How it works · 5 Live demo (real screenshot) · 6 Traction ·
7 Architecture · **8 Market** (real photos) · **9 How users benefit** · 10 Business model ·
11 Differentiation · 12 Impact & ROI · 13 Where LinkMint goes next (vertical expansion) ·
14 Compliance · 15 GTM & roadmap · 16 Team · 17 The ask & contact.

## One thing to fill in
- **Slide 16 (Team):** there is a bracketed prompt — *“[Add 1–2 lines on your background…]”*. Replace it with your real bio so the founder card is complete.

## Slide 8 imagery (attribution)
Photos are from Wikimedia Commons under CC BY-SA, credited on the slide and here:
`rail.jpg` © Fiona Graham / WorldRemit (CC BY-SA 2.0) · `vendor.jpg` © Biva2017 (CC BY-SA 4.0) ·
`market.jpg` © Wawerumacha (CC BY-SA 4.0) · `boda.jpg` © Tmaokisa (CC BY-SA 4.0). Swap them in `assets/market/` and re-render.

## Notes on accuracy (so claims hold up to a technical judge)
- Fee = **0.5% (50 bps), min KES 1**, split **70% validators / 20% treasury / 10% burned** — verified in `paylink-chain/internal/fee/calculator.go`.
- Compliance figures (Tier-1 ceiling **KES 50,000**, AML threshold **KES 150,000**) — verified in `compliance-risk` tests.
- Slide 5’s settlement screenshot is **real** — captured live; the chain tx hash on it is genuine.
- The e2e settlement test (`go test -tags=e2e ./adapters/mpesa/test/...`) passes against the live stack (~2.4s).
- "$800B+/yr mobile money in Sub-Saharan Africa" is attributed to GSMA (context, not our own projection).
