// Package daraja is the Go core's client to the Node.js Daraja rail service (the rail SDK that
// speaks MPesa: OAuth, STK push, raw callback parsing). Everything MPesa-specific lives in Node;
// this package exchanges only rail-neutral DTOs so the A.4 boundary holds. All HTTP is behind the
// Client interface so tests use a fake and make no live network calls.
package daraja

// STKPushParams is a request to start a charge via STK Push, sent to the Node rail service.
//
// A.1 (non-custodial): ShortCode is the RECEIVER's MPesa shortcode/till — funds settle to the
// receiver directly. There is deliberately no LinkMint-owned collection account anywhere.
type STKPushParams struct {
	ShortCode  string `json:"shortcode"`   // receiver's shortcode (BusinessShortCode + PartyB)
	PayerPhone string `json:"payer_phone"` // payer MSISDN (2547XXXXXXXX)
	Amount     uint64 `json:"amount"`      // whole KES
	AccountRef string `json:"account_ref"` // short PayLink reference (AccountReference)
	PayLinkID  string `json:"paylink_id"`  // full pl_id (for the rail service's logs/AccountReference)
}

// STKPushResult is the neutral result of initiating an STK Push.
type STKPushResult struct {
	MerchantRequestID string `json:"merchant_request_id"`
	CheckoutRequestID string `json:"checkout_request_id"` // correlation id echoed in the callback
	ResponseCode      string `json:"response_code"`       // "0" on accepted
	CustomerMessage   string `json:"customer_message"`
}

// CallbackResult is the rail-neutral outcome the Node rail service forwards to the core after it
// parses a Daraja STK callback. Success is ResultCode == 0; on failure the payment fields are empty.
type CallbackResult struct {
	MerchantRequestID  string `json:"merchant_request_id"`
	CheckoutRequestID  string `json:"checkout_request_id"`
	ResultCode         int    `json:"result_code"`
	ResultDesc         string `json:"result_desc"`
	Amount             uint64 `json:"amount"`               // paid amount (whole KES)
	MpesaReceiptNumber string `json:"mpesa_receipt_number"` // becomes the proof tx_id
	PhoneNumber        string `json:"phone_number"`         // payer MSISDN — becomes the proof sender
	TransactionDate    string `json:"transaction_date"`     // Daraja yyyyMMddHHmmss (informational)
}

// Succeeded reports whether the payment completed (ResultCode == 0).
func (c CallbackResult) Succeeded() bool { return c.ResultCode == 0 }
