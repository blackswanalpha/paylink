// StubDaraja stands in for the real Daraja client when DARAJA_STUB=true (devnet / e2e). It returns a
// synthetic CheckoutRequestID without any live Safaricom call, so the full settlement path can be
// exercised end-to-end without sandbox credentials. NEVER enable in production.

export class StubDaraja {
  constructor() {
    this.n = 0;
  }

  async stkPush({ shortcode, accountRef }) {
    this.n += 1;
    return {
      merchant_request_id: `stub-m-${this.n}`,
      checkout_request_id: `ws_CO_stub_${accountRef || 'x'}_${this.n}`,
      response_code: '0',
      customer_message: `stubbed STK push to ${shortcode} (DARAJA_STUB=true, no live Daraja)`,
    };
  }
}
