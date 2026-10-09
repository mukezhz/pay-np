# Pay-NP 🇳🇵

One Go interface for Nepali payment gateways. Integrate once, swap providers.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

```bash
go get github.com/mukezhz/pay-np
```

## Providers

| Provider | Package | Checkout | Callback signed | Status API | Spec source |
|---|---|---|---|---|---|
| eSewa ePay v2 | `esewa` | form POST | ✅ HMAC-SHA256 | ✅ by TxnID | official docs + sandbox vectors |
| Khalti ePayment | `khalti` | redirect (`pidx`) | ❌ | ✅ by pidx | official docs |
| ConnectIPS (NCHL) | `connectips` | form POST, RSA token | ❌ | ✅ by TxnID | official docs |
| Fonepay web | `fonepay` | redirect | ✅ HMAC-SHA512 | ⚠️ needs callback `UID` | community implementations — verify in dev |
| Hamro Pay Checkout | `hamropay` | form POST (session + token) | ❌ redirect · ✅ webhook (HMAC-SHA512) | ✅ Get Transaction by TxnID | official docs |
| IME Pay web | `imepay` | redirect (token) | ❌ | ✅ Confirm / Recheck by token | IME SDK config + community — verify in staging |

Prabhu Pay and NPS OnePG publish no developer docs; contributions welcome.

## Try it

```bash
go run ./examples/checkout   # http://localhost:8080 — eSewa sandbox works with no setup
```

A local test shop with a payment form and a per-attempt timeline of initiate → callback →
lookup. See [examples/checkout](examples/checkout/README.md) for provider env vars.

## The contract

```go
type Provider interface {
    Name() ProviderName
    Initiate(ctx context.Context, req InitiateRequest) (*Checkout, error)
    ParseCallback(query url.Values) (*Callback, error)
    Lookup(ctx context.Context, req LookupRequest) (*Transaction, error)
}
```

- Money is `paynp.Paisa` (`int64`, 1 rupee = 100). Adapters convert to each provider's format.
- `Initiate` never charges. It returns a `Checkout`: a redirect (`GET`) or an auto-submitting
  form (`POST`). `checkout.Write(w, r)` does either.
- `ParseCallback` reads the provider redirect and verifies its signature when the provider
  signs one. **Its status is a hint.**
- `Lookup` asks the provider server-to-server. **Only trust `Lookup`.** It returns
  `ErrAmountMismatch` if a successful payment's amount differs from what you expected.

Statuses: `PENDING`, `SUCCESS`, `FAILED`, `CANCELED`, `EXPIRED`, `NOT_FOUND`, `REFUNDED`,
`PARTIALLY_REFUNDED`. `Status.Final()` tells you when to stop polling.

## Usage

```go
var p paynp.Provider
p, _ = esewa.New(esewa.Config{ProductCode: esewa.SandboxProductCode, SecretKey: esewa.SandboxSecretKey})
// p, _ = khalti.New(khalti.Config{SecretKey: key, WebsiteURL: "https://shop.example"})
// p, _ = connectips.New(connectips.Config{MerchantID: "550", AppID: "MER-550-APP-1", AppName: "Shop", Password: pw, PrivateKey: rsaKey})

// 1. Start: persist TxnID, Amount and checkout.ProviderRef, then send the user off.
http.HandleFunc("/pay", func(w http.ResponseWriter, r *http.Request) {
    co, err := p.Initiate(r.Context(), paynp.InitiateRequest{
        TxnID:       "ord-42-a1",          // unique per attempt, ≤ 20 chars, [A-Za-z0-9-]
        Amount:      150000,               // Rs 1500
        Description: "Course fee",
        SuccessURL:  "https://shop.example/return/ord-42-a1",
        FailureURL:  "https://shop.example/return/ord-42-a1?failed=1",
    })
    if err != nil { http.Error(w, err.Error(), 502); return }
    saveAttempt("ord-42-a1", 150000, co.ProviderRef)
    _ = co.Write(w, r)
})

// 2. Return: parse, then confirm with the provider using YOUR stored amount.
http.HandleFunc("/return/", func(w http.ResponseWriter, r *http.Request) {
    cb, _ := p.ParseCallback(r.URL.Query())          // may fail on cancel; TxnID is in the path too
    a := loadAttempt(strings.TrimPrefix(r.URL.Path, "/return/"))
    tx, err := p.Lookup(r.Context(), paynp.LookupRequest{TxnID: a.ID, Amount: a.Amount, ProviderRef: a.Ref, Callback: cb})
    switch {
    case errors.Is(err, paynp.ErrAmountMismatch): // tampering or partial payment: do not fulfil
    case err != nil:                               // provider down: retry later
    case tx.Status == paynp.StatusSuccess:         // fulfil (idempotently)
    }
})
```

Run a reconciler that calls `Lookup` for attempts still `PENDING`/`NOT_FOUND`: users close the
tab after paying, and none of these providers sends webhooks.

### Provider notes

- **eSewa** — whole rupees are sent as integers (`"110"`), matching eSewa's examples. The failure
  redirect has no signed `data`; put your TxnID in `FailureURL`.
- **eSewa mobile SDK** (Android/iOS/Flutter) — the app pays in-app and sends your backend the
  refId. Verify it with `esewa.NewMobile(...).Verify(ctx, MobileVerifyRequest{ProductID, RefID,
  Amount})`, using the SDK client ID/secret (sandbox: `esewa.SandboxMobileClientID/Secret`).
  Use a unique productId per order: Verify only reports SUCCESS for a COMPLETE payment of that
  product and amount, so another order's refId cannot be replayed. Khalti's mobile SDKs reuse
  the server-side `pidx`, so plain `Lookup` covers them.
- **Khalti** — one return URL (`SuccessURL`) for every outcome. Persist `Checkout.ProviderRef`
  (`pidx`); `Lookup` needs it. Minimum amount is set by Khalti.
- **ConnectIPS** — success/failure URLs are registered with NCHL, so `SuccessURL`/`FailureURL`
  are ignored; NCHL appends `?TXNID=`. TxnID ≤ 20 chars. Load the key with
  `connectips.ParsePFX(bytes, password)` (store the .pfx encrypted). `Username` defaults to
  `AppID`. NCHL documents no hosts: defaults are `uat.connectips.com` /
  `login.connectips.com`, override with `Host`. NCHL's docs disagree on whether
  `gettxndetail.txnAmt` is paisa; the SDK assumes paisa — confirm in UAT.
- **Fonepay** — one return URL (`SuccessURL`), PRN (TxnID) 3–25 chars. The verification API is
  keyed by the `UID` Fonepay returns on redirect, so `Lookup` needs `req.Callback` and returns
  `ErrCallbackRequired` without it: abandoned payments cannot be reconciled by this flow.
- **Hamro Pay** — free self-service UAT keys at pay-sandbox.hamropatro.com. Amount Rs 10–50,000,
  TxnID ≤ 25 chars without commas. Both redirects only append `?MerchantTxnId=` (unsigned).
  `ParseWebhook(header, body)` verifies the signed webhook with `WebhookSecret` — the only
  signed status source; still match its amount. Production URLs come with live onboarding, so
  set `APIBaseURL`/`GatewayURL` for `Production`.
- **IME Pay** — `Initiate` calls GetToken (binds the amount); persist `Checkout.ProviderRef`
  (token). `Lookup` calls Confirm when the callback carried a TransactionId, otherwise Recheck.

## Errors

| Error | Meaning |
|---|---|
| `ErrInvalidConfig` / `ErrInvalidRequest` | caller bug |
| `ErrInvalidCallback` / `ErrInvalidSignature` | malformed or forged redirect |
| `ErrAmountMismatch` | provider reports a different amount; never fulfil |
| `ErrCallbackRequired` | provider's status API needs redirect values |
| `*APIError` | provider returned non-2xx (`StatusCode`, `Body`) |

## Adding a provider

Implement `paynp.Provider` in a new package, add `var _ paynp.Provider = (*Client)(nil)`, use
`internal/httpx` for calls (context, timeout, `APIError`), end `Lookup` with
`paynp.MatchAmount`, and cover signing with a published test vector plus an `httptest` server.

## Testing

```bash
go test ./...
```

## License

MIT. Unofficial SDK — check each provider's terms and test in sandbox before going live.
