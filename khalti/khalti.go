// Package khalti implements paynp.Provider for Khalti ePayment (KPG-2).
package khalti

import (
	"context"
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

var baseURLs = map[paynp.Environment]string{
	paynp.Sandbox:    "https://dev.khalti.com/api/v2",
	paynp.Production: "https://khalti.com/api/v2",
}

const MinAmount paynp.Paisa = 1000

const SandboxSecretKey = "live_secret_key_68791341fdd94846a146f0457ff7b455"

type Config struct {
	SecretKey   string
	WebsiteURL  string
	Environment paynp.Environment
	BaseURL     string
	HTTPClient  *http.Client
}

type Client struct {
	cfg  Config
	http *http.Client
}

var _ paynp.Provider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	if cfg.SecretKey == "" || cfg.WebsiteURL == "" {
		return nil, fmt.Errorf("%w: khalti needs SecretKey and WebsiteURL", paynp.ErrInvalidConfig)
	}
	if cfg.BaseURL == "" {
		var ok bool
		if cfg.BaseURL, ok = baseURLs[cfg.Environment]; !ok {
			return nil, fmt.Errorf("%w: khalti unknown environment", paynp.ErrInvalidConfig)
		}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg, http: httpx.DefaultClient(cfg.HTTPClient)}, nil
}

func (c *Client) Name() paynp.ProviderName { return paynp.Khalti }

type customerInfo struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type initiateRequest struct {
	ReturnURL         string        `json:"return_url"`
	WebsiteURL        string        `json:"website_url"`
	Amount            int64         `json:"amount"`
	PurchaseOrderID   string        `json:"purchase_order_id"`
	PurchaseOrderName string        `json:"purchase_order_name"`
	CustomerInfo      *customerInfo `json:"customer_info,omitempty"`
}

type initiateResponse struct {
	Pidx       string    `json:"pidx"`
	PaymentURL string    `json:"payment_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func (c *Client) Initiate(ctx context.Context, req paynp.InitiateRequest) (*paynp.Checkout, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.SuccessURL == "" {
		return nil, fmt.Errorf("%w: khalti needs SuccessURL", paynp.ErrInvalidRequest)
	}
	if req.Amount < MinAmount {
		return nil, fmt.Errorf("%w: khalti minimum is %s", paynp.ErrInvalidRequest, MinAmount.Rupees())
	}
	name := req.Description
	if name == "" {
		name = req.TxnID
	}
	body := initiateRequest{
		ReturnURL:         req.SuccessURL,
		WebsiteURL:        c.cfg.WebsiteURL,
		Amount:            int64(req.Amount),
		PurchaseOrderID:   req.TxnID,
		PurchaseOrderName: name,
	}
	if req.Customer != (paynp.Customer{}) {
		body.CustomerInfo = &customerInfo{Name: req.Customer.Name, Email: req.Customer.Email, Phone: req.Customer.Phone}
	}
	_, raw, err := c.post(ctx, "/epayment/initiate/", body, false)
	if err != nil {
		return nil, err
	}
	var r initiateResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("khalti: decode initiate: %w", err)
	}
	if r.Pidx == "" || r.PaymentURL == "" {
		return nil, &paynp.APIError{Provider: paynp.Khalti, StatusCode: http.StatusOK, Body: raw}
	}
	return &paynp.Checkout{
		Provider:    paynp.Khalti,
		Method:      http.MethodGet,
		URL:         r.PaymentURL,
		ProviderRef: r.Pidx,
		ExpiresAt:   r.ExpiresAt,
	}, nil
}

func (c *Client) ParseCallback(query url.Values) (*paynp.Callback, error) {
	pidx := query.Get("pidx")
	if pidx == "" {
		return nil, fmt.Errorf("%w: khalti pidx missing", paynp.ErrInvalidCallback)
	}
	var amount int64
	if s := query.Get("total_amount"); s != "" {
		var err error
		if amount, err = strconv.ParseInt(s, 10, 64); err != nil {
			return nil, fmt.Errorf("%w: khalti total_amount %q", paynp.ErrInvalidCallback, s)
		}
	}
	status := mapStatus(query.Get("status"))
	if status == paynp.StatusSuccess {
		status = paynp.StatusPending // unsigned: only Lookup may say SUCCESS
	}
	return &paynp.Callback{
		TxnID:       query.Get("purchase_order_id"),
		ProviderRef: pidx,
		Status:      status,
		Amount:      paynp.Paisa(amount),
		Values:      query,
	}, nil
}

type lookupResponse struct {
	Pidx          string  `json:"pidx"`
	TotalAmount   int64   `json:"total_amount"`
	Status        string  `json:"status"`
	TransactionID *string `json:"transaction_id"`
}

func (c *Client) Lookup(ctx context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	pidx := req.ProviderRef
	if pidx == "" && req.Callback != nil {
		pidx = req.Callback.ProviderRef
	}
	if pidx == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("%w: khalti lookup needs pidx (ProviderRef) and Amount", paynp.ErrInvalidRequest)
	}
	// Khalti answers 4xx for expired/canceled/unknown pidx; the body still says why.
	code, raw, err := c.post(ctx, "/epayment/lookup/", map[string]string{"pidx": pidx}, true)
	if err != nil {
		return nil, err
	}
	var r lookupResponse
	if err := json.Unmarshal(raw, &r); err != nil || r.Status == "" || (r.Pidx != "" && r.Pidx != pidx) {
		if code == http.StatusNotFound {
			return &paynp.Transaction{Provider: paynp.Khalti, TxnID: req.TxnID, ProviderRef: pidx, Status: paynp.StatusNotFound, Raw: raw}, nil
		}
		return nil, &paynp.APIError{Provider: paynp.Khalti, StatusCode: code, Body: raw}
	}
	tx := &paynp.Transaction{
		Provider:    paynp.Khalti,
		TxnID:       req.TxnID,
		ProviderRef: pidx,
		Status:      mapStatus(r.Status),
		Amount:      paynp.Paisa(r.TotalAmount),
		Raw:         raw,
	}
	return paynp.MatchAmount(tx, req.Amount)
}

func (c *Client) post(ctx context.Context, path string, body any, allow4x bool) (int, []byte, error) {
	return httpx.Do(ctx, c.http, paynp.Khalti, httpx.Request{
		Method:  http.MethodPost,
		URL:     c.cfg.BaseURL + path,
		Header:  http.Header{"Authorization": {"Key " + c.cfg.SecretKey}},
		JSON:    body,
		Allow4x: allow4x,
	})
}

func mapStatus(s string) paynp.Status {
	switch strings.ToLower(s) {
	case "completed":
		return paynp.StatusSuccess
	case "user canceled", "user cancelled", "canceled":
		return paynp.StatusCanceled
	case "expired":
		return paynp.StatusExpired
	case "refunded":
		return paynp.StatusRefunded
	case "partially refunded":
		return paynp.StatusPartiallyRefunded
	case "failed":
		return paynp.StatusFailed
	default: // Initiated, Pending
		return paynp.StatusPending
	}
}
