// DarajaClient speaks the Safaricom Daraja API: OAuth token (cached) + Lipa na M-Pesa Online (STK
// Push). All MPesa wire shapes live here. fetchFn and now are injectable for tests (no live calls).

const TOKEN_SKEW_MS = 30_000; // refresh a little before expiry

// pad2 / timestamp build the Daraja yyyyMMddHHmmss timestamp (UTC).
function pad2(n) {
  return String(n).padStart(2, '0');
}

export function darajaTimestamp(d) {
  return (
    d.getUTCFullYear().toString() +
    pad2(d.getUTCMonth() + 1) +
    pad2(d.getUTCDate()) +
    pad2(d.getUTCHours()) +
    pad2(d.getUTCMinutes()) +
    pad2(d.getUTCSeconds())
  );
}

// stkPassword = base64(shortcode + passkey + timestamp), per Daraja's STK Push spec.
export function stkPassword(shortcode, passkey, timestamp) {
  return Buffer.from(shortcode + passkey + timestamp, 'utf8').toString('base64');
}

export class DarajaClient {
  constructor(config, { fetchFn = fetch, now = () => new Date() } = {}) {
    if (config.sandbox && !config.darajaBaseURL.includes('sandbox')) {
      throw new Error(
        `DARAJA_SANDBOX=true but DARAJA_BASE_URL is not a sandbox host (${config.darajaBaseURL}); set DARAJA_SANDBOX=false to allow it`,
      );
    }
    this.cfg = config;
    this.fetchFn = fetchFn;
    this.now = now;
    this._token = null;
    this._tokenExpiresAt = 0;
  }

  // getToken returns a cached OAuth token, refreshing when missing/expired.
  async getToken() {
    if (this._token && this.now().getTime() < this._tokenExpiresAt - TOKEN_SKEW_MS) {
      return this._token;
    }
    const auth = Buffer.from(`${this.cfg.consumerKey}:${this.cfg.consumerSecret}`, 'utf8').toString('base64');
    const url = `${this.cfg.darajaBaseURL}/oauth/v1/generate?grant_type=client_credentials`;
    const resp = await this.fetchFn(url, { headers: { Authorization: `Basic ${auth}` } });
    if (!resp.ok) {
      throw new Error(`daraja oauth failed: http ${resp.status}`);
    }
    const body = await resp.json();
    this._token = body.access_token;
    const expiresIn = Number(body.expires_in || 3599);
    this._tokenExpiresAt = this.now().getTime() + expiresIn * 1000;
    return this._token;
  }

  // stkPush starts a charge. shortcode is the RECEIVER's shortcode (BusinessShortCode + PartyB) —
  // A.1: funds settle to the receiver directly, never a LinkMint-owned account. Returns rail-neutral
  // fields the core uses for correlation.
  async stkPush({ shortcode, payerPhone, amount, accountRef, paylinkId }) {
    const token = await this.getToken();
    const ts = darajaTimestamp(this.now());
    const body = {
      BusinessShortCode: shortcode,
      Password: stkPassword(shortcode, this.cfg.passkey, ts),
      Timestamp: ts,
      TransactionType: 'CustomerPayBillOnline',
      Amount: String(amount),
      PartyA: payerPhone,
      PartyB: shortcode,
      PhoneNumber: payerPhone,
      CallBackURL: this.cfg.callbackURL,
      AccountReference: accountRef || (paylinkId || '').slice(0, 12),
      TransactionDesc: 'LinkMint PayLink payment',
    };
    const resp = await this.fetchFn(`${this.cfg.darajaBaseURL}/mpesa/stkpush/v1/processrequest`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const out = await resp.json().catch(() => ({}));
    if (!resp.ok) {
      throw new Error(`daraja stk push failed: http ${resp.status} ${JSON.stringify(out)}`);
    }
    return {
      merchant_request_id: out.MerchantRequestID || '',
      checkout_request_id: out.CheckoutRequestID || '',
      response_code: String(out.ResponseCode ?? ''),
      customer_message: out.CustomerMessage || '',
    };
  }
}
