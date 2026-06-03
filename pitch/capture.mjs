/**
 * Drives the live LinkMint demo wizard and captures real screenshots of each step.
 * After creating a PayLink in the UI, it fires the same M-Pesa charge + Daraja callback the
 * passing e2e test uses, so the on-chain settlement the UI is polling actually completes.
 */
import { chromium } from 'playwright';

const OUT = '/home/mbugua/Documents/augment-projects/linkMint/pitch/assets';
const APP = 'http://localhost:3000/';
const ADAPTER = 'http://localhost:8082';
const DARAJA = 'http://localhost:8083';
const CB_TOKEN = 'devnet-callback-token';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function settle(plId, amount) {
  const receipt = 'DEMO' + Date.now();
  const ch = await fetch(`${ADAPTER}/v1/charges`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Idempotency-Key': 'demo-' + plId },
    body: JSON.stringify({ pl_id: plId, amount, payer_phone: '254700000000', receiver_shortcode: '174379' }),
  });
  const chBody = await ch.text();
  console.log('charge', ch.status, chBody);
  const checkoutId = (JSON.parse(chBody).checkout_request_id) || 'demo-co';
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
const page = await browser.newPage({ viewport: { width: 720, height: 1100 }, deviceScaleFactor: 2 });
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

await page.goto(APP, { waitUntil: 'networkidle' });
await page.getByText('Create a PayLink').first().waitFor({ timeout: 15000 });
// Set a nicer headline amount (100000 minor = KES 1,000.00) and wait for the field to settle.
const amountInput = page.getByRole('spinbutton').first();
await amountInput.waitFor({ timeout: 10000 });
await amountInput.fill(String(amount));
await page.getByText('KES 1,000.00').first().waitFor({ timeout: 5000 }).catch(() => {});
await sleep(500);
await card().screenshot({ path: `${OUT}/01-create.png` });
console.log('captured 01-create');

await page.getByRole('button', { name: /Create PayLink/i }).click();

// Step 2 — pay instructions
await page.getByText('Pay with M-PESA').first().waitFor({ timeout: 15000 });
await sleep(1400);
await card().screenshot({ path: `${OUT}/02-pay.png` });
console.log('captured 02-pay');

// Fire the real settlement for the PayLink the UI just created.
for (let i = 0; i < 20 && !plId; i++) await sleep(250);
if (!plId) { console.log('NO plId captured — cannot settle'); }
else { await settle(plId, amount); }

// Step 3 — watch settlement
await page.getByRole('button', { name: /watch settlement/i }).click();
await page.getByText('Settled on-chain.').first().waitFor({ timeout: 45000 }).catch(() => console.log('did not reach VERIFIED in time'));
// Dismiss the success toast so the card screenshot is clean.
await sleep(2000);
await page.locator('[data-sonner-toast] button, button[aria-label*="lose" i]').first().click({ timeout: 1500 }).catch(() => {});
await sleep(600);
await card().screenshot({ path: `${OUT}/03-settled.png` });
console.log('captured 03-settled');

await browser.close();
console.log('DONE');
