// Package hamropay implements paynp.Provider for Hamro Pay Checkout.
//
// Spec: https://hamropay.com.np/checkout/developer/reference/
package hamropay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/internal/httpx"
)

// UAT endpoints. Production URLs are issued with live onboarding, so a
// Production config must set APIBaseURL and GatewayURL.
const (
	SandboxAPIBaseURL = "https://uat-payclient.hamropatro.com"
	SandboxGatewayURL = "https://uat-checkout-pay.hamropatro.com"
)

const (
	MinAmount paynp.Paisa = 1000
	MaxAmount paynp.Paisa = 5_000_000
	// sessionTTL is Hamro Pay's documented session lifetime.
	sessionTTL = 10 * time.Minute
)

type Config struct {
	MerchantID   string
	ClientID     string
	ClientAPIKey string
	ClientSecret string
	// WebhookSecret (merchantWebHookSigningSecret) is only needed for ParseWebhook.
	WebhookSecret string
	Environment   paynp.Environment
	APIBaseURL    string
	GatewayURL    string
	HTTPClient    *http.Client
	Now           func() time.Time
}

type Client struct {
	cfg  Config
	http *http.Client
}

var _ paynp.Provider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	if cfg.MerchantID == "" || cfg.ClientID == "" || cfg.ClientAPIKey == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("%w: hamropay needs MerchantID, ClientID, ClientAPIKey and ClientSecret", paynp.ErrInvalidConfig)
	}
	if cfg.Environment == paynp.Sandbox {
		cfg.APIBaseURL = cmpOr(cfg.APIBaseURL, SandboxAPIBaseURL)
		cfg.GatewayURL = cmpOr(cfg.GatewayURL, SandboxGatewayURL)
	}
	if cfg.APIBaseURL == "" || cfg.GatewayURL == "" {
		return nil, fmt.Errorf("%w: hamropay production needs APIBaseURL and GatewayURL", paynp.ErrInvalidConfig)
	}
	cfg.APIBaseURL = strings.TrimRight(cfg.APIBaseURL, "/")
	cfg.GatewayURL = strings.TrimRight(cfg.GatewayURL, "/")
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Client{cfg: cfg, http: httpx.DefaultClient(cfg.HTTPClient)}, nil
}

func (c *Client) Name() paynp.ProviderName { return paynp.HamroPay }

type product struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

type sessionRequest struct {
	MerchantTxnID     string    `json:"merchantTxnId"`
	MerchantID        string    `json:"merchantId"`
	TransactionAmount string    `json:"transactionAmount"`
	FailedURL         string    `json:"failedRedirectionUrl"`
	SuccessURL        string    `json:"successRedirectionUrl"`
	ProductList       []product `json:"productList"`
	Remarks           string    `json:"remarks,omitempty"`
	PhoneNumber       string    `json:"phone_number,omitempty"`
}

// Initiate creates a checkout session and returns the signed gateway form.
func (c *Client) Initiate(ctx context.Context, req paynp.InitiateRequest) (*paynp.Checkout, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	switch {
	case req.SuccessURL == "":
		return nil, fmt.Errorf("%w: hamropay needs SuccessURL", paynp.ErrInvalidRequest)
	case len(req.TxnID) > 25 || strings.Contains(req.TxnID, ","):
		return nil, fmt.Errorf("%w: hamropay TxnID must be ≤ 25 chars without commas", paynp.ErrInvalidRequest)
	case req.Amount < MinAmount || req.Amount > MaxAmount:
		return nil, fmt.Errorf("%w: hamropay amount must be Rs %s–%s", paynp.ErrInvalidRequest, MinAmount.Rupees(), MaxAmount.Rupees())
	}
	amount := strconv.FormatInt(int64(req.Amount), 10)
	desc := cmpOr(req.Description, req.TxnID)
	body := sessionRequest{
		MerchantTxnID:     req.TxnID,
		MerchantID:        c.cfg.MerchantID,
		TransactionAmount: amount,
		FailedURL:         cmpOr(req.FailureURL, req.SuccessURL),
		SuccessURL:        req.SuccessURL,
		ProductList:       []product{{Name: desc, Price: float64(req.Amount) / 100, Quantity: 1}},
		Remarks:           desc,
		PhoneNumber:       req.Customer.Phone,
	}
	raw, err := c.call(ctx, "/v1/checkout/sessionId", body, req.TxnID, amount, c.cfg.MerchantID, c.cfg.ClientID, c.cfg.ClientAPIKey)
	if err != nil {
		return nil, err
	}
	var r struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || r.SessionID == "" {
		return nil, &paynp.APIError{Provider: paynp.HamroPay, StatusCode: http.StatusOK, Body: raw}
	}
	fields := map[string]string{
		"merchant_id":             c.cfg.MerchantID,
		"session_id":              r.SessionID,
		"merchant_transaction_id": req.TxnID,
		"remarks":                 cut(desc, 250),
		"token":                   sign(c.cfg.ClientSecret, c.cfg.MerchantID, req.TxnID, r.SessionID, amount, c.cfg.ClientID, c.cfg.ClientAPIKey),
	}
	if req.Customer.Phone != "" {
		fields["phone_number"] = req.Customer.Phone
	}
	return &paynp.Checkout{
		Provider:    paynp.HamroPay,
		Method:      http.MethodPost,
		URL:         c.cfg.GatewayURL + "/api/checkout",
		Fields:      fields,
		ProviderRef: r.SessionID,
		ExpiresAt:   c.cfg.Now().Add(sessionTTL),
	}, nil
}

// ParseCallback reads the unsigned ?MerchantTxnId= on both redirect URLs; always Lookup.
func (c *Client) ParseCallback(query url.Values) (*paynp.Callback, error) {
	id := query.Get("MerchantTxnId")
	if id == "" {
		return nil, fmt.Errorf("%w: hamropay MerchantTxnId missing", paynp.ErrInvalidCallback)
	}
	return &paynp.Callback{TxnID: id, Status: paynp.StatusPending, Values: query}, nil
}

type transaction struct {
	MerchantTransactionID string      `json:"merchantTransactionId"`
	MerchantTxnID         string      `json:"merchantTxnId"` // UAT has been seen using this name
	Status                string      `json:"status"`
	Amount                json.Number `json:"amount"`
	Message               string      `json:"message"`
}

func (c *Client) Lookup(ctx context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	if req.TxnID == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("%w: hamropay lookup needs TxnID and Amount", paynp.ErrInvalidRequest)
	}
	body := map[string]string{"merchantId": c.cfg.MerchantID, "merchantTxnId": req.TxnID}
	raw, err := c.call(ctx, "/v1/checkout/transaction", body, req.TxnID, c.cfg.MerchantID, c.cfg.ClientID, c.cfg.ClientAPIKey)
	if err != nil {
		return nil, err
	}
	var r transaction
	if err := decode(raw, &r); err != nil || r.Status == "" {
		return nil, &paynp.APIError{Provider: paynp.HamroPay, StatusCode: http.StatusOK, Body: raw}
	}
	if id := cmpOr(r.MerchantTransactionID, r.MerchantTxnID); id != "" && id != req.TxnID {
		return nil, &paynp.APIError{Provider: paynp.HamroPay, StatusCode: http.StatusOK, Body: raw}
	}
	tx := &paynp.Transaction{Provider: paynp.HamroPay, TxnID: req.TxnID, ProviderRef: req.ProviderRef, Status: mapStatus(r.Status), Raw: raw}
	if tx.Status == paynp.StatusSuccess {
		if tx.Amount, err = paynp.ParseRupees(r.Amount.String()); err != nil {
			return nil, fmt.Errorf("hamropay: amount %q: %w", r.Amount, err)
		}
	}
	return paynp.MatchAmount(tx, req.Amount)
}

type webhook struct {
	MerchantTxnID string            `json:"merchantTxnId"`
	MerchantID    string            `json:"merchantId"`
	Amount        json.Number       `json:"amount"`
	Status        string            `json:"status"`
	Metadata      map[string]string `json:"metadata"`
}

// ParseWebhook verifies a Hamro Pay webhook (Signature header) with
// WebhookSecret. A verified webhook is signed, so its status is authoritative,
// but check Amount against your records before fulfilling.
func (c *Client) ParseWebhook(header http.Header, body []byte) (*paynp.Callback, error) {
	if c.cfg.WebhookSecret == "" {
		return nil, fmt.Errorf("%w: hamropay WebhookSecret not set", paynp.ErrInvalidConfig)
	}
	var w webhook
	if err := decode(body, &w); err != nil || w.MerchantTxnID == "" {
		return nil, fmt.Errorf("%w: hamropay webhook body", paynp.ErrInvalidCallback)
	}
	if w.MerchantID != c.cfg.MerchantID {
		return nil, fmt.Errorf("%w: hamropay webhook merchantId", paynp.ErrInvalidCallback)
	}
	amount, err := paynp.ParseRupees(w.Amount.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paynp.ErrInvalidCallback, err)
	}
	got := []byte(header.Get("Signature"))
	ok := false
	// Hamro Pay may sign the double as 1200, 1200.0 or 1200.00.
	for _, a := range amountForms(w.Amount.String(), amount) {
		if hmac.Equal(got, []byte(sign(c.cfg.WebhookSecret, w.MerchantTxnID, w.MerchantID, w.Status, a))) {
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("%w: hamropay webhook", paynp.ErrInvalidSignature)
	}
	values := url.Values{}
	for k, v := range w.Metadata {
		values.Set(k, v)
	}
	return &paynp.Callback{TxnID: w.MerchantTxnID, Status: mapStatus(w.Status), Amount: amount, Values: values}, nil
}

func (c *Client) call(ctx context.Context, path string, body any, sigFields ...string) ([]byte, error) {
	_, raw, err := httpx.Do(ctx, c.http, paynp.HamroPay, httpx.Request{
		Method: http.MethodPost,
		URL:    c.cfg.APIBaseURL + path,
		Header: http.Header{
			"Client-Id":      {c.cfg.ClientID},
			"Client-API-Key": {c.cfg.ClientAPIKey},
			"Signature":      {sign(c.cfg.ClientSecret, sigFields...)},
		},
		JSON:  body,
		Close: true, // UAT drops keep-alive sockets
	})
	return raw, err
}

func mapStatus(s string) paynp.Status {
	switch strings.ToUpper(s) {
	case "COMPLETED":
		return paynp.StatusSuccess
	case "FAILED":
		return paynp.StatusFailed
	case "NOT_INITIATED":
		return paynp.StatusNotFound
	default: // PENDING, PROCESSING
		return paynp.StatusPending
	}
}

// sign is base64 HMAC-SHA512 over the comma-joined values.
func sign(secret string, values ...string) string {
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(strings.Join(values, ",")))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func amountForms(raw string, p paynp.Paisa) []string {
	whole := strconv.FormatInt(int64(p/100), 10)
	forms := []string{raw, p.Rupees()}
	if p%100 == 0 {
		forms = append(forms, whole, whole+".0")
	} else if p%10 == 0 {
		forms = append(forms, fmt.Sprintf("%s.%d", whole, p%100/10))
	}
	return forms
}

func decode(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
