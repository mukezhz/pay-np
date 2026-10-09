package main

import (
	"net/http"
	"os"

	paynp "github.com/mukezhz/pay-np"
)

type envVar struct {
	Name     string
	Desc     string
	Required bool
	// Builtin names the sandbox default used when the variable is unset.
	Builtin string
}

type link struct{ Label, URL string }

type providerDoc struct {
	Summary string
	GetKeys string
	Vars    []envVar
	Notes   []string
	Links   []link
	Snippet string
	// Mobile is server-side verification code for the provider's mobile SDK, if it has its own flow.
	Mobile string
}

var docs = map[paynp.ProviderName]providerDoc{
	paynp.Esewa: {
		Summary: "eSewa ePay v2. The browser form-POSTs a signed request; eSewa returns a signed base64 payload and offers a status API.",
		GetKeys: "Sandbox works out of the box with eSewa's public EPAYTEST merchant. For live keys, apply for an eSewa merchant account.",
		Vars: []envVar{
			{Name: "ESEWA_PRODUCT_CODE", Desc: "Merchant product code", Required: true, Builtin: "EPAYTEST"},
			{Name: "ESEWA_SECRET_KEY", Desc: "HMAC-SHA256 signing secret", Required: true, Builtin: "eSewa's published sandbox secret"},
			{Name: "ESEWA_STATUS_URL", Desc: "Override the status API URL"},
			{Name: "ESEWA_MOBILE_CLIENT_ID", Desc: "SDK client ID (mobile verification)", Builtin: "eSewa's published SDK client"},
			{Name: "ESEWA_MOBILE_CLIENT_SECRET", Desc: "SDK client secret (mobile verification)", Builtin: "eSewa's published SDK secret"},
		},
		Notes: []string{
			"Whole rupees are sent as integers (\"110\"), matching eSewa's signing examples.",
			"The failure redirect carries no signed data, so the TxnID is kept in the return URL path.",
			"Status API maps COMPLETE → SUCCESS, PENDING/AMBIGUOUS → PENDING, NOT_FOUND → NOT_FOUND.",
			"Mobile SDK payments use different credentials (client ID/secret) and esewa.NewMobile; the SDK test login is 9711111111, Nepal@123, MPIN 1122, token 123456.",
		},
		Mobile: `// The app pays with eSewa's Android/iOS/Flutter SDK using a productId you
// generate per order, then sends your backend the refId the SDK returned.
app, err := esewa.NewMobile(esewa.MobileConfig{
    ClientID:     esewa.SandboxMobileClientID,
    ClientSecret: esewa.SandboxMobileClientSecret,
})

tx, err := app.Verify(ctx, esewa.MobileVerifyRequest{
    ProductID: order.ID,     // binds the payment to this order
    RefID:     refIDFromApp, // optional; productId + amount also works
    Amount:    order.Amount, // from your records
})
if err == nil && tx.Status == paynp.StatusSuccess {
    fulfilOnce(order.ID, tx.ProviderRef)
}`,
		Links: []link{{"ePay v2 docs", "https://developer.esewa.com.np/pages/Epay-V2"}},
		Snippet: `p, err := esewa.New(esewa.Config{
    ProductCode: esewa.SandboxProductCode,
    SecretKey:   esewa.SandboxSecretKey,
})`,
	},
	paynp.Khalti: {
		Summary: "Khalti ePayment (KPG-2). The server registers the payment and gets a pidx and payment_url; the user is redirected there.",
		GetKeys: "Sandbox uses the dev.khalti.com key Khalti publishes in its docs. Your own test key is at test-admin.khalti.com.",
		Vars: []envVar{
			{Name: "KHALTI_SECRET_KEY", Desc: "Secret key sent as Authorization: Key …", Required: true, Builtin: "Khalti's published dev key"},
			{Name: "KHALTI_WEBSITE_URL", Desc: "Merchant site shown on Khalti checkout (defaults to BASE_URL)"},
		},
		Notes: []string{
			"Minimum amount is Rs 10.",
			"One return URL for every outcome; the redirect is unsigned, so only Lookup can report SUCCESS.",
			"Lookup needs the pidx saved from Initiate (Checkout.ProviderRef).",
		},
		Links: []link{{"ePayment docs", "https://docs.khalti.com/khalti-epayment/"}},
		Snippet: `p, err := khalti.New(khalti.Config{
    SecretKey:  khalti.SandboxSecretKey,
    WebsiteURL: "https://shop.example",
})`,
	},
	paynp.ConnectIPS: {
		Summary: "NCHL ConnectIPS. The browser form-POSTs a request signed with your merchant RSA key; status comes from gettxndetail.",
		GetKeys: "Request UAT access and a .pfx certificate from NCHL through your bank. Register success/failure URLs with NCHL.",
		Vars: []envVar{
			{Name: "CONNECTIPS_MERCHANT_ID", Desc: "Numeric merchant ID", Required: true},
			{Name: "CONNECTIPS_APP_ID", Desc: "Application ID", Required: true},
			{Name: "CONNECTIPS_APP_NAME", Desc: "Application name", Required: true},
			{Name: "CONNECTIPS_PASSWORD", Desc: "Status API Basic-auth password", Required: true},
			{Name: "CONNECTIPS_PFX_PATH", Desc: "Path to the merchant .pfx certificate", Required: true},
			{Name: "CONNECTIPS_PFX_PASSWORD", Desc: "Password of the .pfx file"},
			{Name: "CONNECTIPS_USERNAME", Desc: "Basic-auth user (defaults to APP_ID)"},
			{Name: "CONNECTIPS_HOST", Desc: "Override uat.connectips.com / login.connectips.com"},
		},
		Notes: []string{
			"Return URLs are registered with NCHL, not sent per request. Register {BASE_URL}/return/connectips.",
			"TxnID must be ≤ 20 characters; amounts are sent in paisa.",
			"Confirm in UAT that gettxndetail reports txnAmt in paisa.",
		},
		Links: []link{{"ConnectIPS docs", "https://doc.connectips.com/docs/connectIPS-Gateway/merchant-interface"}},
		Snippet: `key, err := connectips.ParsePFX(pfxBytes, pfxPassword)
p, err := connectips.New(connectips.Config{
    MerchantID: "550", AppID: "MER-550-APP-1", AppName: "Shop",
    Password: pw, PrivateKey: key,
})`,
	},
	paynp.Fonepay: {
		Summary: "Fonepay web redirect. A signed GET redirect; Fonepay signs its return and verifies with the UID it returns.",
		GetKeys: "Ask Fonepay for a dev merchant code and secret. Fonepay publishes no public spec; verify in their dev environment.",
		Vars: []envVar{
			{Name: "FONEPAY_MERCHANT_CODE", Desc: "Merchant code (PID)", Required: true},
			{Name: "FONEPAY_SECRET_KEY", Desc: "HMAC-SHA512 secret", Required: true},
		},
		Notes: []string{
			"PRN (TxnID) must be 3–25 characters.",
			"Lookup needs the callback's UID, so abandoned payments cannot be reconciled with this flow.",
		},
		Snippet: `p, err := fonepay.New(fonepay.Config{
    MerchantCode: "NBQM",
    SecretKey:    secret,
})`,
	},
	paynp.HamroPay: {
		Summary: "Hamro Pay Checkout. The server creates a session, signs a token, and the browser form-POSTs to the gateway. Hamro Pay also sends a signed webhook.",
		GetKeys: "Free self-service UAT keys: sign up at pay-sandbox.hamropatro.com/signup (verification OTP 000000), then copy them from Client Credentials.",
		Vars: []envVar{
			{Name: "HAMROPAY_MERCHANT_ID", Desc: "Merchant ID", Required: true},
			{Name: "HAMROPAY_CLIENT_ID", Desc: "Client-Id header", Required: true},
			{Name: "HAMROPAY_CLIENT_API_KEY", Desc: "Client-API-Key header", Required: true},
			{Name: "HAMROPAY_CLIENT_SECRET", Desc: "HMAC-SHA512 signing secret", Required: true},
			{Name: "HAMROPAY_WEBHOOK_SECRET", Desc: "Webhook signing secret (Configuration → Webhook)"},
			{Name: "HAMROPAY_API_BASE_URL", Desc: "Production API URL (issued at live onboarding)"},
			{Name: "HAMROPAY_GATEWAY_URL", Desc: "Production gateway URL (issued at live onboarding)"},
		},
		Notes: []string{
			"Amount must be Rs 10–50,000; TxnID ≤ 25 characters without commas.",
			"Test wallet 9841414141, T-PIN 0000. Checkout OTP 000000 succeeds, 111111 stays pending, 222222 fails.",
			"Webhooks arrive at {BASE_URL}/webhook/hamropay and need a public URL (e.g. a tunnel).",
		},
		Links: []link{
			{"API reference", "https://hamropay.com.np/checkout/developer/reference/"},
			{"UAT merchant portal", "https://pay-sandbox.hamropatro.com/"},
		},
		Snippet: `p, err := hamropay.New(hamropay.Config{
    MerchantID: mid, ClientID: cid,
    ClientAPIKey: apiKey, ClientSecret: secret,
    WebhookSecret: hookSecret, // for ParseWebhook
})`,
	},
	paynp.IMEPay: {
		Summary: "IME Pay web checkout. The server gets a token that binds the amount, the user is redirected, then Confirm or Recheck settles it.",
		GetKeys: "Ask IME Pay for staging merchant credentials. IME publishes no public spec; verify in staging.",
		Vars: []envVar{
			{Name: "IMEPAY_MERCHANT_CODE", Desc: "Merchant code", Required: true},
			{Name: "IMEPAY_MODULE", Desc: "Module name (sent base64 in the Module header)", Required: true},
			{Name: "IMEPAY_API_USER", Desc: "Basic-auth user", Required: true},
			{Name: "IMEPAY_API_PASSWORD", Desc: "Basic-auth password", Required: true},
		},
		Notes: []string{
			"Lookup calls Confirm when the callback carried a TransactionId, otherwise Recheck by token.",
		},
		Snippet: `p, err := imepay.New(imepay.Config{
    MerchantCode: code, Module: module,
    APIUser: user, APIPassword: pass,
})`,
	},
}

type varStatus struct {
	envVar
	State string // set | builtin | missing | unset
}

func (s *shop) providerDocs(w http.ResponseWriter, r *http.Request) {
	name := paynp.ProviderName(r.PathValue("name"))
	d, ok := docs[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, enabled := s.providers[name]
	vars := make([]varStatus, len(d.Vars))
	missing := 0
	for i, v := range d.Vars {
		vs := varStatus{envVar: v, State: "unset"}
		switch {
		case os.Getenv(v.Name) != "":
			vs.State = "set"
		case v.Builtin != "" && s.env == "sandbox":
			vs.State = "builtin"
		case v.Required:
			vs.State = "missing"
			missing++
		}
		vars[i] = vs
	}
	info := infoFor(name)
	info.Enabled = enabled
	render(w, "provider.html", map[string]any{
		"Title": info.Label, "Page": "providers", "Env": s.env, "BaseURL": s.baseURL,
		"P": info, "D": d, "Vars": vars, "Missing": missing, "All": s.catalog(),
	})
}
