# Test shop

A local checkout UI for trying every provider end to end: start a payment, pay on the
provider's sandbox, come back, and watch `ParseCallback` and `Lookup` on a timeline.

```bash
go run ./examples/checkout
# open http://localhost:8080
```

`/` is an overview of the SDK and provider readiness; `/checkout` is the test shop
(keys `1`–`5` pick a provider, `⌘↵` pays); `/attempts/{txn}` shows status, flow and raw
provider responses (`R` re-runs Lookup).

eSewa and Khalti work with no setup (their published sandbox merchants). Other providers
are enabled by env vars. Attempts are kept in memory only.

| Provider | Env vars |
|---|---|
| eSewa | `ESEWA_PRODUCT_CODE`, `ESEWA_SECRET_KEY` (sandbox defaults built in), `ESEWA_STATUS_URL` (optional) |
| Khalti | `KHALTI_SECRET_KEY` (sandbox default built in), `KHALTI_WEBSITE_URL` (defaults to `BASE_URL`) |
| ConnectIPS | `CONNECTIPS_MERCHANT_ID`, `CONNECTIPS_APP_ID`, `CONNECTIPS_APP_NAME`, `CONNECTIPS_PASSWORD`, `CONNECTIPS_USERNAME` (optional), `CONNECTIPS_PFX_PATH`, `CONNECTIPS_PFX_PASSWORD`, `CONNECTIPS_HOST` (optional) |
| Fonepay | `FONEPAY_MERCHANT_CODE`, `FONEPAY_SECRET_KEY` |
| Hamro Pay | `HAMROPAY_MERCHANT_ID`, `HAMROPAY_CLIENT_ID`, `HAMROPAY_CLIENT_API_KEY`, `HAMROPAY_CLIENT_SECRET`, `HAMROPAY_WEBHOOK_SECRET` (optional), `HAMROPAY_API_BASE_URL`/`HAMROPAY_GATEWAY_URL` (production) |
| IME Pay | `IMEPAY_MERCHANT_CODE`, `IMEPAY_MODULE`, `IMEPAY_API_USER`, `IMEPAY_API_PASSWORD` |
| Server | `ADDR` (`:8080`), `BASE_URL` (`http://localhost:8080`), `PAYNP_ENV` (`sandbox` \| `production`) |

Return URLs are `{BASE_URL}/return/{provider}/{txn}`. ConnectIPS uses the URLs registered
with NCHL instead: register `{BASE_URL}/return/connectips` as both success and failure URL.
If a provider rejects `localhost`, expose the server with a tunnel and set `BASE_URL`.

Sandbox accounts:

- eSewa: ID `9806800001`–`9806800005`, password `Nepal@123`, token `123456`
- Khalti: ID `9800000000`–`9800000005`, MPIN `1111`, OTP `987654`; minimum Rs 10
- Hamro Pay: wallet `9841414141`, T-PIN `0000`; OTP `000000` succeeds, `111111` stays pending,
  `222222` fails. Merchant keys: free signup at https://pay-sandbox.hamropatro.com/signup
  (verification OTP `000000`). For webhooks, register `{BASE_URL}/webhook/hamropay` (needs a
  public URL, e.g. a tunnel).

The **Check status** button calls `Lookup` again, the way a reconciler would after a user
closes the tab.
