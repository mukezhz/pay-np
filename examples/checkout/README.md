# Test shop

A local checkout UI for trying every provider end to end: start a payment, pay on the
provider's sandbox, come back, and watch `ParseCallback` and `Lookup` on a timeline.

```bash
go run ./examples/checkout
# open http://localhost:8080
```

eSewa works with no setup (public `EPAYTEST` merchant). Other providers are enabled
by env vars. Attempts are kept in memory only.

| Provider | Env vars |
|---|---|
| eSewa | `ESEWA_PRODUCT_CODE`, `ESEWA_SECRET_KEY` (sandbox defaults built in), `ESEWA_STATUS_URL` (optional) |
| Khalti | `KHALTI_SECRET_KEY` (test key from test-admin.khalti.com), `KHALTI_WEBSITE_URL` (defaults to `BASE_URL`) |
| ConnectIPS | `CONNECTIPS_MERCHANT_ID`, `CONNECTIPS_APP_ID`, `CONNECTIPS_APP_NAME`, `CONNECTIPS_PASSWORD`, `CONNECTIPS_USERNAME` (optional), `CONNECTIPS_PFX_PATH`, `CONNECTIPS_PFX_PASSWORD`, `CONNECTIPS_HOST` (optional) |
| Fonepay | `FONEPAY_MERCHANT_CODE`, `FONEPAY_SECRET_KEY` |
| IME Pay | `IMEPAY_MERCHANT_CODE`, `IMEPAY_MODULE`, `IMEPAY_API_USER`, `IMEPAY_API_PASSWORD` |
| Server | `ADDR` (`:8080`), `BASE_URL` (`http://localhost:8080`), `PAYNP_ENV` (`sandbox` \| `production`) |

Return URLs are `{BASE_URL}/return/{provider}/{txn}`. ConnectIPS uses the URLs registered
with NCHL instead: register `{BASE_URL}/return/connectips` as both success and failure URL.
If a provider rejects `localhost`, expose the server with a tunnel and set `BASE_URL`.

Sandbox accounts:

- eSewa: ID `9806800001`–`9806800005`, password `Nepal@123`, token `123456`
- Khalti: ID `9800000000`–`9800000005`, MPIN `1111`, OTP `987654`; minimum Rs 10

The **Check status** button calls `Lookup` again, the way a reconciler would after a user
closes the tab.
