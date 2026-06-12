/**
 * Records a real demo video of LinkMint doing Create -> Pay -> Settle on the live stack.
 *
 * It drives the actual web wizard with Playwright, fires the same M-Pesa charge + Daraja
 * callback the passing e2e test uses (so the on-chain settlement really completes), and
 * overlays on-brand subtitle captions + title/cards for each beat.
 *
 * Each beat's on-screen hold is sized to its narration clip (vo/durations.json) and the
 * moment each beat appears is logged to vo/marks.json, so mux.mjs can drop the matching
 * voice-over clip exactly under it.
 *
 * Output: a silent .webm in pitch/video/ + vo/marks.json. Then: node mux.mjs.
 */
import { chromium } from 'playwright';
import { readFileSync, writeFileSync } from 'node:fs';

const ROOT = '/home/mbugua/Documents/augment-projects/linkMint/pitch';
const OUT_DIR = `${ROOT}/video`;
const APP = 'http://localhost:3000/';
const ADAPTER = 'http://localhost:8082';
const DARAJA = 'http://localhost:8083';
const CB_TOKEN = 'devnet-callback-token';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Narration clip durations (seconds), produced by vo_gen.py. Each beat's on-screen hold
// is sized from these so the voice-over fits, and we log when each beat appears (a timing
// manifest) so the mux step can drop each clip exactly under its beat.
const D = JSON.parse(readFileSync(`${ROOT}/vo/durations.json`, 'utf8'));
const PAD = 1.2; // seconds of silence held after each beat's narration finishes
const marks = [];

async function settle(plId, amount) {
  const receipt = 'DEMO' + Date.now();
  const ch = await fetch(`${ADAPTER}/v1/charges`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'demo-' + plId },
    body: JSON.stringify({ pl_id: plId, amount, payer_phone: '254700000000', receiver_shortcode: '174379' }),
  });
  const chBody = await ch.text();
  console.log('charge', ch.status, chBody);
  let checkoutId = 'demo-co';
  try { checkoutId = JSON.parse(chBody).checkout_request_id || checkoutId; } catch {}
  const cb = {
    Body: { stkCallback: {
      MerchantRequestID: 'demo-m', CheckoutRequestID: checkoutId, ResultCode: 0, ResultDesc: 'ok',
      CallbackMetadata: { Item: [
        { Name: 'Amount', Value: amount },
        { Name: 'MpesaReceiptNumber', Value: receipt },
        { Name: 'PhoneNumber', Value: '254700000000' },
      ] },
    } },
  };
  const r = await fetch(`${DARAJA}/daraja/callback?t=${CB_TOKEN}`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(cb),
  });
  console.log('callback', r.status);
}

const browser = await chromium.launch({ headless: true });
const ctx = await browser.newContext({
  viewport: { width: 1280, height: 720 },
  recordVideo: { dir: OUT_DIR, size: { width: 1280, height: 720 } },
  deviceScaleFactor: 1,
});
const page = await ctx.newPage();

// t0 ~ first recorded frame (page creation). mark(beat) records, in seconds from t0, the
// moment a beat's narration should start; the mux step places vo/<beat>.mp3 there.
const t0 = Date.now();
const mark = (beat) => {
  const t = (Date.now() - t0) / 1000;
  marks.push({ beat, t });
  console.log('MARK', beat, t.toFixed(2));
};
// Hold the screen for a beat's narration (its clip) plus a trailing pad.
const holdFor = (beat, pad = PAD) => sleep((D[beat] + pad) * 1000);

// --- on-brand subtitle overlay (Ivory Premium tokens) -------------------------------
async function installOverlay() {
  await page.addStyleTag({ content: `
    /* hide Next.js dev indicator so it never shows in the recording */
    nextjs-portal, #__next-build-watcher, [data-nextjs-dev-tools-button],
    [data-nextjs-toast] { display: none !important; }

    /* full-screen narrative "slide" card */
    #lm-card {
      position: fixed; inset: 0; z-index: 2147483645; pointer-events: none;
      display: flex; flex-direction: column; align-items: center; justify-content: center;
      gap: 26px; background: #FAF7F0; padding: 0 120px;
      opacity: 0; transition: opacity .55s ease;
      font-family: Inter, system-ui, sans-serif; text-align: center;
    }
    #lm-card.show { opacity: 1; }
    #lm-card .kicker { display: flex; align-items: center; gap: 12px;
      font-size: 15px; letter-spacing: .14em; text-transform: uppercase; color: #6B655C; font-weight: 600; }
    #lm-card .kicker .d { width: 14px; height: 14px; background: #0F6E4E;
      transform: rotate(45deg); border-radius: 3px; }
    #lm-card .head { font-family: 'Fraunces', Georgia, serif; font-size: 46px; font-weight: 600;
      color: #1C1A17; letter-spacing: -0.02em; line-height: 1.18; max-width: 980px; }
    #lm-card .head b { color: #0F6E4E; font-weight: 600; }
    #lm-card .lines { display: flex; flex-direction: column; gap: 12px; max-width: 900px; }
    #lm-card .lines span { font-size: 23px; color: #6B655C; line-height: 1.4; }
    #lm-card .lines span b { color: #1C1A17; font-weight: 600; }
    #lm-card .chips { display: flex; flex-wrap: wrap; gap: 12px; justify-content: center; max-width: 920px; }
    #lm-card .chips span { font-size: 18px; color: #0B5840; background: #E9F3EC;
      border: 1px solid #C8E2D0; padding: 9px 16px; border-radius: 999px; font-weight: 500; }
    #lm-card .chips span.gold { color: #7a5e16; background: #FBF6E9; border-color: #ecdca6; }

    #lm-cap {
      position: fixed; left: 0; right: 0; bottom: 0; z-index: 2147483647;
      pointer-events: none; display: flex; justify-content: center;
      padding: 0 0 40px 0; font-family: Inter, system-ui, sans-serif;
    }
    #lm-cap .bar {
      display: flex; align-items: center; gap: 14px;
      max-width: 980px; padding: 16px 26px; border-radius: 14px;
      background: rgba(28,26,23,0.92); color: #FAF7F0;
      box-shadow: 0 10px 40px rgba(28,26,23,0.28);
      border: 1px solid rgba(200,162,75,0.45);
      opacity: 0; transform: translateY(12px);
      transition: opacity .45s ease, transform .45s ease;
      backdrop-filter: blur(2px);
    }
    #lm-cap.show .bar { opacity: 1; transform: translateY(0); }
    #lm-cap .dot { width: 12px; height: 12px; flex: 0 0 12px;
      background: #C8A24B; transform: rotate(45deg); border-radius: 2px; }
    #lm-cap .txt { display: flex; flex-direction: column; gap: 2px; }
    #lm-cap .lead { font-size: 22px; font-weight: 600; letter-spacing: -0.01em; line-height: 1.25; }
    #lm-cap .sub { font-size: 14px; color: #C8C2B7; font-weight: 400; }

    /* opening / closing full-bleed title card */
    #lm-title {
      position: fixed; inset: 0; z-index: 2147483646; pointer-events: none;
      display: flex; flex-direction: column; align-items: center; justify-content: center;
      gap: 18px; background: #FAF7F0;
      opacity: 0; transition: opacity .6s ease;
      font-family: Inter, system-ui, sans-serif;
    }
    #lm-title.show { opacity: 1; }
    #lm-title .mark { display: flex; align-items: center; gap: 16px; }
    #lm-title .diamond { width: 40px; height: 40px; background: #0F6E4E;
      transform: rotate(45deg); border-radius: 6px;
      box-shadow: 0 6px 22px rgba(15,110,78,0.35); }
    #lm-title .word { font-family: 'Fraunces', Georgia, serif; font-size: 64px;
      font-weight: 600; color: #1C1A17; letter-spacing: -0.02em; }
    #lm-title .tag { font-size: 26px; color: #1C1A17; font-weight: 500; max-width: 760px;
      text-align: center; line-height: 1.3; }
    #lm-title .tag b { color: #0F6E4E; }
    #lm-title .sub2 { font-size: 16px; color: #6B655C; letter-spacing: 0.02em; }
  ` });
  await page.evaluate(() => {
    if (!document.getElementById('lm-cap')) {
      const c = document.createElement('div');
      c.id = 'lm-cap';
      c.innerHTML = '<div class="bar"><div class="dot"></div><div class="txt"><div class="lead"></div><div class="sub"></div></div></div>';
      document.documentElement.appendChild(c);
    }
    if (!document.getElementById('lm-title')) {
      const t = document.createElement('div');
      t.id = 'lm-title';
      t.innerHTML = '<div class="mark"><div class="diamond"></div><div class="word">LinkMint</div></div><div class="tag"></div><div class="sub2"></div>';
      document.documentElement.appendChild(t);
    }
    if (!document.getElementById('lm-card')) {
      const k = document.createElement('div');
      k.id = 'lm-card';
      k.innerHTML = '<div class="kicker"><div class="d"></div><span class="ktxt"></span></div><div class="head"></div><div class="lines"></div><div class="chips"></div>';
      document.documentElement.appendChild(k);
    }
  });
}

// Force the brand webfonts to load before showing any card, so the title text isn't
// invisible (FOIT) while Fraunces/Inter are still fetching. One load covers every card.
async function ensureFonts() {
  await page.evaluate(async () => {
    try {
      const specs = [
        "600 64px 'Fraunces'", "600 46px 'Fraunces'",
        "600 26px 'Inter'", "500 23px 'Inter'", "400 23px 'Inter'", "600 15px 'Inter'",
      ];
      await Promise.all(specs.map((s) => document.fonts.load(s)));
      await document.fonts.ready;
    } catch (e) {}
  });
}

async function caption(lead, sub = '') {
  await page.evaluate(({ lead, sub }) => {
    const c = document.getElementById('lm-cap');
    if (!c) return;
    c.querySelector('.lead').textContent = lead;
    c.querySelector('.sub').textContent = sub;
    c.classList.add('show');
  }, { lead, sub });
}
async function hideCaption() {
  await page.evaluate(() => document.getElementById('lm-cap')?.classList.remove('show'));
}
async function title(tag, sub2) {
  await page.evaluate(({ tag, sub2 }) => {
    const t = document.getElementById('lm-title');
    if (!t) return;
    t.querySelector('.tag').innerHTML = tag;
    t.querySelector('.sub2').textContent = sub2 || '';
    t.classList.add('show');
  }, { tag, sub2 });
}
async function hideTitle() {
  await page.evaluate(() => document.getElementById('lm-title')?.classList.remove('show'));
}
async function showCard({ kicker = '', head = '', lines = [], chips = [] }) {
  await page.evaluate(({ kicker, head, lines, chips }) => {
    const k = document.getElementById('lm-card');
    if (!k) return;
    k.querySelector('.ktxt').textContent = kicker;
    k.querySelector('.head').innerHTML = head;
    k.querySelector('.lines').innerHTML = lines.map((l) => `<span>${l}</span>`).join('');
    k.querySelector('.chips').innerHTML = chips
      .map((c) => (typeof c === 'string' ? `<span>${c}</span>` : `<span class="${c.cls || ''}">${c.t}</span>`))
      .join('');
    k.classList.add('show');
  }, { kicker, head, lines, chips });
}
async function hideCard() {
  await page.evaluate(() => document.getElementById('lm-card')?.classList.remove('show'));
}
async function removeBoot() {
  await page.evaluate(() => {
    const b = document.getElementById('lm-boot');
    if (b) { b.style.opacity = '0'; setTimeout(() => b.remove(), 550); }
  });
}

const card = () => page.locator('.chakra-card__root').first();

let plId = null;
let amount = 100000;
page.on('response', async (res) => {
  const u = res.url();
  if (u.includes('/v1/paylinks') && res.request().method() === 'POST') {
    try {
      const txt = await res.text();
      const m = txt.match(/0x[0-9a-fA-F]{64}/);
      if (m) plId = m[0];
      const am = txt.match(/"amount"\s*:\s*(\d+)/);
      if (am) amount = Number(am[1]);
      console.log('create response captured: plId=', plId, 'amount=', amount);
    } catch (e) { console.log('resp parse err', e.message); }
  }
});

// A boot cover painted from the very first frame (inline styles so React's hydration
// can't reconcile it away; static box so it keeps compositing even during load jank).
// It sits below the overlay cards but above the app, so the open is clean ivory — never
// a white flash or a half-loaded app — until we lift it for the live demo.
await page.addInitScript(() => {
  function build() {
    if (document.getElementById('lm-boot')) return;
    const b = document.createElement('div');
    b.id = 'lm-boot';
    b.setAttribute('style', [
      'position:fixed', 'inset:0', 'z-index:2147483640', 'background:#FAF7F0',
      'transition:opacity .55s ease', 'pointer-events:none',
    ].join(';'));
    (document.documentElement || document).appendChild(b);
  }
  if (document.documentElement) build();
  else document.addEventListener('DOMContentLoaded', build);
});

// ---- BEAT 1 (t1_title): open on the title card -------------------------------------
await page.goto(APP, { waitUntil: 'domcontentloaded' });
await installOverlay();
await ensureFonts(); // boot cover (ivory) holds the screen while fonts load — no FOIT
await title('Pay anyone, anywhere, through any rail — <b>with a link.</b>',
  'Non-custodial · multi-rail · blockchain-settled payments for East Africa');
await sleep(700); // title fully faded in — this is the front-trim anchor (see marks)
mark('t1_title');
await holdFor('t1_title', 0.9);
await hideTitle();
await sleep(600);

// ---- BEAT 2 (t2_problem) -----------------------------------------------------------
await showCard({
  kicker: 'The problem',
  head: 'To accept a digital payment today, a business needs a <b>licence</b>, <b>locked-up capital</b>, and one chosen rail.',
  lines: ['And fixed 1–3% fees make small payments — a tip, a song, a single article — simply not worth collecting.'],
});
await sleep(450);
mark('t2_problem');
await holdFor('t2_problem');
await hideCard();
await sleep(550);

// ---- BEAT 3 (t3_idea) --------------------------------------------------------------
await showCard({
  kicker: 'The idea',
  head: 'A <b>PayLink</b>: scan it, pay with M-PESA, settle on-chain.',
  lines: ['No account. No app. The money goes straight to the receiver — <b>it never touches LinkMint.</b>'],
});
await sleep(450);
mark('t3_idea');
await holdFor('t3_idea');
await hideCard();
await sleep(500);
await removeBoot(); // lift the cover — reveal the real, running app
await sleep(600);

// ---- BEAT 4 (t4_create): create a PayLink (live) -----------------------------------
await page.getByText('Create a PayLink').first().waitFor({ timeout: 25000 });
await sleep(500);
await caption('Create a PayLink', 'Receiver, amount, expiry — minted on the lVM, our own blockchain.');
await sleep(450);
mark('t4_create');
const tCreate = Date.now();
const amountInput = page.getByRole('spinbutton').first();
await amountInput.waitFor({ timeout: 10000 });
await amountInput.click();
await amountInput.fill('');
await amountInput.pressSequentially(String(amount), { delay: 120 });
await page.getByText('KES 1,000.00').first().waitFor({ timeout: 5000 }).catch(() => {});
// let the narration run, then submit near the end of the clip (so "mints it" ~ transition)
const usedCreate = (Date.now() - tCreate) / 1000;
await sleep(Math.max(0, (D.t4_create - usedCreate - 0.6)) * 1000);
await page.getByRole('button', { name: /Create PayLink/i }).click();
await sleep(600);

// ---- BEAT 5 (t5_pay): pay with M-Pesa ----------------------------------------------
await page.getByText('Pay with M-PESA').first().waitFor({ timeout: 20000 });
await sleep(500);
await caption('The customer pays with M-PESA', 'Pay Bill 174379, the PayLink as the account number. No app, no account.');
await sleep(450);
mark('t5_pay');
// Fire the real settlement for the PayLink the UI just created (off-camera).
for (let i = 0; i < 24 && !plId; i++) await sleep(250);
if (!plId) { console.log('NO plId captured — cannot settle'); }
else { await settle(plId, amount); }
await holdFor('t5_pay', 0.8);

// ---- BEAT 6 (t6_settle): settle on-chain (the money shot) --------------------------
await caption('Proof → validator → chain → VERIFIED',
  'A signed proof, verified and settled on-chain — real tx hash, tamper-evident trail.');
await sleep(450);
mark('t6_settle');
const tSettle = Date.now();
await page.getByRole('button', { name: /watch settlement/i }).click();
await page.getByText('Settled on-chain.').first().waitFor({ timeout: 45000 })
  .catch(() => console.log('did not reach VERIFIED in time'));
const usedSettle = (Date.now() - tSettle) / 1000; // includes the time to reach VERIFIED
await sleep(Math.max(0, (D.t6_settle + 1.4 - usedSettle)) * 1000);
await hideCaption();
await sleep(500);

// ---- BEAT 7 (t7_traction): what's already built ------------------------------------
await showCard({
  kicker: "This is not a prototype — it's running now",
  head: 'A real, end-to-end system you just watched settle.',
  chips: [
    '12 services', 'Custom blockchain (lVM)', 'M-PESA adapter', 'JavaScript SDK',
    'Next.js web app', { t: 'e2e settlement test passing in CI', cls: 'gold' }, { t: '80% coverage gate', cls: 'gold' },
  ],
});
await sleep(450);
mark('t7_traction');
await holdFor('t7_traction');
await hideCard();
await sleep(650);

// ---- BEAT 8 (t8_close): close on the brand (hold to the end) ------------------------
await title('Pay anyone, through any rail — <b>with a link.</b>',
  'LinkMint  ·  Victor Mbugua Kamande, Founder & CEO  ·  kamandembugua18@gmail.com');
await sleep(450);
mark('t8_close');
await holdFor('t8_close', 1.6); // narration + a tail to land on the brand

// persist the timing manifest for the audio-mux step
writeFileSync(`${ROOT}/vo/marks.json`, JSON.stringify(marks, null, 2));

// finalize the recording
const video = page.video();
await ctx.close();
const path = await video.path();
console.log('VIDEO', path);
await browser.close();
console.log('DONE');
