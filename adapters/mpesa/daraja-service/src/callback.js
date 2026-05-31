// Callback handling: parse Daraja's STK callback envelope into rail-neutral fields, and forward
// them to the Go adapter core. Nothing MPesa-specific crosses into the core (A.4) beyond the
// neutral fields below.

// parseCallback turns a raw Daraja STK callback object into rail-neutral fields. Failed payments
// (ResultCode != 0) carry no CallbackMetadata, so the payment fields stay empty.
export function parseCallback(raw) {
  const cb = raw?.Body?.stkCallback;
  if (!cb || !cb.CheckoutRequestID) {
    throw new Error('daraja callback missing Body.stkCallback.CheckoutRequestID');
  }
  const out = {
    merchant_request_id: cb.MerchantRequestID || '',
    checkout_request_id: cb.CheckoutRequestID,
    result_code: Number(cb.ResultCode),
    result_desc: cb.ResultDesc || '',
    amount: 0,
    mpesa_receipt_number: '',
    phone_number: '',
    transaction_date: '',
  };
  const items = cb.CallbackMetadata?.Item || [];
  for (const it of items) {
    switch (it.Name) {
      case 'Amount':
        out.amount = Math.trunc(Number(it.Value) || 0);
        break;
      case 'MpesaReceiptNumber':
        out.mpesa_receipt_number = String(it.Value ?? '');
        break;
      case 'PhoneNumber':
        out.phone_number = String(it.Value ?? '');
        break;
      case 'TransactionDate':
        out.transaction_date = String(it.Value ?? '');
        break;
    }
  }
  return out;
}

// forwardToCore POSTs the rail-neutral fields to the adapter core's /v1/callbacks/mpesa, returning
// { status, body }. Throws on transport failure so the caller can ask Daraja to redeliver.
export async function forwardToCore({ coreURL, internalToken, fields, fetchFn = fetch }) {
  const resp = await fetchFn(`${coreURL.replace(/\/$/, '')}/v1/callbacks/mpesa`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(internalToken ? { 'X-Internal-Token': internalToken } : {}),
    },
    body: JSON.stringify(fields),
  });
  const body = await resp.json().catch(() => ({}));
  return { status: resp.status, body };
}
