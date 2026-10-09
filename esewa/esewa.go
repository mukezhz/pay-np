// Package esewa implements paynp.Provider for eSewa ePay v2.
package esewa

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/internal/httpx"
)

const (
	SandboxProductCode = "EPAYTEST"
	SandboxSecretKey   = "8gBm/:&EnhH.1/q"
)

var endpoints = map[paynp.Environment]struct{ form, status string }{
	paynp.Sandbox: {
		form:   "https://rc-epay.esewa.com.np/api/epay/main/v2/form",
		status: "https://rc.esewa.com.np/api/epay/transaction/status/",
	},
	paynp.Production: {
		form:   "https://epay.esewa.com.np/api/epay/main/v2/form",
		status: "https://epay.esewa.com.np/api/epay/transaction/status/",
	},
}

const signedFields = "total_amount,transaction_uuid,product_code"

type Client struct {
	cfg  Config
	http *http.Client
}

var _ paynp.Provider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	if cfg.ProductCode == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("%w: esewa needs ProductCode and SecretKey", paynp.ErrInvalidConfig)
	}
	ep, ok := endpoints[cfg.Environment]
	if !ok {
		return nil, fmt.Errorf("%w: esewa unknown environment", paynp.ErrInvalidConfig)
	}
	if cfg.FormURL == "" {
		cfg.FormURL = ep.form
	}
	if cfg.StatusURL == "" {
		cfg.StatusURL = ep.status
	}
	return &Client{cfg: cfg, http: httpx.DefaultClient(cfg.HTTPClient)}, nil
}

func (c *Client) Name() paynp.ProviderName { return paynp.Esewa }

func (c *Client) Initiate(_ context.Context, req paynp.InitiateRequest) (*paynp.Checkout, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.SuccessURL == "" || req.FailureURL == "" {
		return nil, fmt.Errorf("%w: esewa needs SuccessURL and FailureURL", paynp.ErrInvalidRequest)
	}
	total := formatAmount(req.Amount)
	fields := map[string]string{
		"amount":                  total,
		"tax_amount":              "0",
		"product_service_charge":  "0",
		"product_delivery_charge": "0",
		"total_amount":            total,
		"transaction_uuid":        req.TxnID,
		"product_code":            c.cfg.ProductCode,
		"success_url":             req.SuccessURL,
		"failure_url":             req.FailureURL,
		"signed_field_names":      signedFields,
	}
	sig, err := c.sign(fields, signedFields)
	if err != nil {
		return nil, err
	}
	fields["signature"] = sig
	return &paynp.Checkout{Provider: paynp.Esewa, Method: http.MethodPost, URL: c.cfg.FormURL, Fields: fields}, nil
}

// The failure redirect carries no signed data; keep the TxnID in FailureURL.
func (c *Client) ParseCallback(query url.Values) (*paynp.Callback, error) {
	raw, err := base64.StdEncoding.DecodeString(query.Get("data"))
	if err != nil || len(raw) == 0 {
		return nil, fmt.Errorf("%w: esewa data missing or not base64", paynp.ErrInvalidCallback)
	}
	fields, err := decodeFlat(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: esewa data: %v", paynp.ErrInvalidCallback, err)
	}
	signed := "," + fields["signed_field_names"] + ","
	for _, n := range []string{"transaction_uuid", "total_amount", "product_code", "status"} {
		if !strings.Contains(signed, ","+n+",") {
			return nil, fmt.Errorf("%w: esewa callback does not sign %s", paynp.ErrInvalidSignature, n)
		}
	}
	if fields["product_code"] != c.cfg.ProductCode {
		return nil, fmt.Errorf("%w: esewa product_code", paynp.ErrInvalidCallback)
	}
	want, err := c.sign(fields, fields["signed_field_names"])
	if err != nil {
		return nil, err
	}
	if !hmac.Equal([]byte(want), []byte(fields["signature"])) {
		return nil, fmt.Errorf("%w: esewa callback", paynp.ErrInvalidSignature)
	}
	amount, err := paynp.ParseRupees(fields["total_amount"])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paynp.ErrInvalidCallback, err)
	}
	return &paynp.Callback{
		TxnID:       fields["transaction_uuid"],
		ProviderRef: fields["transaction_code"],
		Status:      mapStatus(fields["status"]),
		Amount:      amount,
		Values:      query,
	}, nil
}

func (c *Client) Lookup(ctx context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	if req.TxnID == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("%w: esewa lookup needs TxnID and Amount", paynp.ErrInvalidRequest)
	}
	q := url.Values{
		"product_code":     {c.cfg.ProductCode},
		"total_amount":     {formatAmount(req.Amount)},
		"transaction_uuid": {req.TxnID},
	}
	_, body, err := httpx.Do(ctx, c.http, paynp.Esewa, httpx.Request{Method: http.MethodGet, URL: c.cfg.StatusURL + "?" + q.Encode()})
	if err != nil {
		return nil, err
	}
	var r statusResponse
	if err := decodeJSON(body, &r); err != nil {
		return nil, fmt.Errorf("esewa: decode status: %w", err)
	}
	if r.Status == "" { // eSewa reports outages as 200 {"code":0,"error_message":...}
		return nil, &paynp.APIError{Provider: paynp.Esewa, StatusCode: http.StatusOK, Body: body}
	}
	tx := &paynp.Transaction{Provider: paynp.Esewa, TxnID: req.TxnID, Status: mapStatus(r.Status), Raw: body}
	if ref := cmp.Or(r.RefID, r.RefIDOld); ref != nil {
		tx.ProviderRef = *ref
	}
	if amt := cmp.Or(r.TotalAmount, r.TotalAmountOld); amt != "" {
		if tx.Amount, err = paynp.ParseRupees(amt.String()); err != nil {
			return nil, err
		}
	}
	return paynp.MatchAmount(tx, req.Amount)
}

func (c *Client) sign(fields map[string]string, names string) (string, error) {
	parts := strings.Split(names, ",")
	if len(parts) < 3 {
		return "", fmt.Errorf("%w: esewa signed_field_names %q", paynp.ErrInvalidCallback, names)
	}
	for i, n := range parts {
		v, ok := fields[n]
		if !ok {
			return "", fmt.Errorf("%w: esewa signed field %q missing", paynp.ErrInvalidCallback, n)
		}
		parts[i] = n + "=" + v
	}
	mac := hmac.New(sha256.New, []byte(c.cfg.SecretKey))
	mac.Write([]byte(strings.Join(parts, ",")))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func formatAmount(p paynp.Paisa) string {
	if p%100 == 0 {
		return strconv.FormatInt(int64(p/100), 10)
	}
	return p.Rupees()
}

func mapStatus(s string) paynp.Status {
	switch s {
	case "COMPLETE":
		return paynp.StatusSuccess
	case "FULL_REFUND":
		return paynp.StatusRefunded
	case "PARTIAL_REFUND":
		return paynp.StatusPartiallyRefunded
	case "CANCELED":
		return paynp.StatusCanceled
	case "NOT_FOUND":
		return paynp.StatusNotFound
	default: // PENDING, AMBIGUOUS
		return paynp.StatusPending
	}
}

// Numbers stay verbatim because eSewa signs the exact text it sent.
func decodeFlat(b []byte) (map[string]string, error) {
	var m map[string]any
	if err := decodeJSON(b, &m); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if v != nil {
			out[k] = fmt.Sprint(v)
		}
	}
	return out, nil
}

func decodeJSON(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}
