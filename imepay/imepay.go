// Package imepay implements paynp.Provider for IME Pay web checkout (no public spec; verify in staging).
package imepay

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/internal/httpx"
)

var hosts = map[paynp.Environment]string{
	paynp.Sandbox:    "https://stg.imepay.com.np:7979",
	paynp.Production: "https://payment.imepay.com.np:7979",
}

type Config struct {
	MerchantCode string
	Module       string
	APIUser      string
	APIPassword  string
	Environment  paynp.Environment
	Host         string
	HTTPClient   *http.Client
}

type Client struct {
	cfg    Config
	header http.Header
	http   *http.Client
}

var _ paynp.Provider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	if cfg.MerchantCode == "" || cfg.Module == "" || cfg.APIUser == "" || cfg.APIPassword == "" {
		return nil, fmt.Errorf("%w: imepay needs MerchantCode, Module, APIUser and APIPassword", paynp.ErrInvalidConfig)
	}
	if cfg.Host == "" {
		var ok bool
		if cfg.Host, ok = hosts[cfg.Environment]; !ok {
			return nil, fmt.Errorf("%w: imepay unknown environment", paynp.ErrInvalidConfig)
		}
	}
	cfg.Host = strings.TrimRight(cfg.Host, "/")
	b64 := base64.StdEncoding.EncodeToString
	header := http.Header{
		"Authorization": {"Basic " + b64([]byte(cfg.APIUser+":"+cfg.APIPassword))},
		"Module":        {b64([]byte(cfg.Module))},
	}
	return &Client{cfg: cfg, header: header, http: httpx.DefaultClient(cfg.HTTPClient)}, nil
}

func (c *Client) Name() paynp.ProviderName { return paynp.IMEPay }

type apiResponse struct {
	ResponseCode        json.RawMessage `json:"ResponseCode"`
	ResponseDescription string          `json:"ResponseDescription"`
	TokenID             json.RawMessage `json:"TokenId"`
	RefID               json.RawMessage `json:"RefId"`
	TransactionID       json.RawMessage `json:"TransactionId"`
	Amount              json.RawMessage `json:"Amount"`
	TranAmount          json.RawMessage `json:"TranAmount"`
}

func (c *Client) Initiate(ctx context.Context, req paynp.InitiateRequest) (*paynp.Checkout, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.SuccessURL == "" {
		return nil, fmt.Errorf("%w: imepay needs SuccessURL", paynp.ErrInvalidRequest)
	}
	amount := req.Amount.Rupees()
	r, raw, err := c.call(ctx, "/api/Web/GetToken", map[string]any{
		"MerchantCode": c.cfg.MerchantCode,
		"Amount":       json.Number(amount),
		"RefId":        req.TxnID,
	})
	if err != nil {
		return nil, err
	}
	token := scalar(r.TokenID)
	if scalar(r.ResponseCode) != "0" || token == "" {
		return nil, &paynp.APIError{Provider: paynp.IMEPay, StatusCode: http.StatusOK, Body: raw}
	}
	cancel := req.FailureURL
	if cancel == "" {
		cancel = req.SuccessURL
	}
	data := strings.Join([]string{token, c.cfg.MerchantCode, req.TxnID, amount, http.MethodGet, req.SuccessURL, cancel}, "|")
	return &paynp.Checkout{
		Provider:    paynp.IMEPay,
		Method:      http.MethodGet,
		URL:         c.cfg.Host + "/WebCheckout/Checkout",
		Fields:      map[string]string{"data": base64.StdEncoding.EncodeToString([]byte(data))},
		ProviderRef: token,
	}, nil
}

var callbackFields = []string{"ResponseCode", "ResponseDescription", "Msisdn", "TransactionId", "RefId", "TranAmount", "TokenId"}

func (c *Client) ParseCallback(query url.Values) (*paynp.Callback, error) {
	raw, err := base64.StdEncoding.DecodeString(query.Get("data"))
	parts := strings.Split(string(raw), "|")
	if err != nil || len(parts) != len(callbackFields) {
		return nil, fmt.Errorf("%w: imepay data malformed", paynp.ErrInvalidCallback)
	}
	values := url.Values{}
	for i, k := range callbackFields {
		values.Set(k, parts[i])
	}
	amount, err := paynp.ParseRupees(values.Get("TranAmount"))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paynp.ErrInvalidCallback, err)
	}
	status := mapCode(values.Get("ResponseCode"))
	if status == paynp.StatusSuccess {
		status = paynp.StatusPending // unsigned: only Lookup may say SUCCESS
	}
	return &paynp.Callback{
		TxnID:       values.Get("RefId"),
		ProviderRef: values.Get("TokenId"),
		Status:      status,
		Amount:      amount,
		Values:      values,
	}, nil
}

// Lookup uses Confirm when the callback has a TransactionId, else Recheck.
func (c *Client) Lookup(ctx context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	token := req.ProviderRef
	if token == "" && req.Callback != nil {
		token = req.Callback.ProviderRef
	}
	if req.TxnID == "" || token == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("%w: imepay lookup needs TxnID, token (ProviderRef) and Amount", paynp.ErrInvalidRequest)
	}
	body := map[string]any{"MerchantCode": c.cfg.MerchantCode, "RefId": req.TxnID, "TokenId": token}
	path := "/api/Web/Recheck"
	if req.Callback != nil && req.Callback.Values.Get("TransactionId") != "" {
		path = "/api/Web/Confirm"
		body["TransactionId"] = req.Callback.Values.Get("TransactionId")
		body["Msisdn"] = req.Callback.Values.Get("Msisdn")
	}
	r, raw, err := c.call(ctx, path, body)
	if err != nil {
		return nil, err
	}
	if t, ref := scalar(r.TokenID), scalar(r.RefID); (t != "" && t != token) || (ref != "" && ref != req.TxnID) {
		return nil, &paynp.APIError{Provider: paynp.IMEPay, StatusCode: http.StatusOK, Body: raw}
	}
	tx := &paynp.Transaction{
		Provider:    paynp.IMEPay,
		TxnID:       req.TxnID,
		ProviderRef: scalar(r.TransactionID),
		Status:      mapCode(scalar(r.ResponseCode)),
		Raw:         raw,
	}
	if tx.Status == paynp.StatusSuccess {
		// GetToken bound the amount, so fall back to it when the reply omits one.
		tx.Amount = req.Amount
		if s := cmp.Or(scalar(r.TranAmount), scalar(r.Amount)); s != "" {
			if tx.Amount, err = paynp.ParseRupees(s); err != nil {
				return nil, err
			}
		}
	}
	return paynp.MatchAmount(tx, req.Amount)
}

func (c *Client) call(ctx context.Context, path string, body any) (*apiResponse, []byte, error) {
	_, raw, err := httpx.Do(ctx, c.http, paynp.IMEPay, httpx.Request{Method: http.MethodPost, URL: c.cfg.Host + path, Header: c.header, JSON: body})
	if err != nil {
		return nil, nil, err
	}
	var r apiResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, nil, fmt.Errorf("imepay: decode %s: %w", path, err)
	}
	return &r, raw, nil
}

func mapCode(code string) paynp.Status {
	switch code {
	case "0":
		return paynp.StatusSuccess
	case "1":
		return paynp.StatusFailed
	case "3":
		return paynp.StatusCanceled
	default:
		return paynp.StatusPending
	}
}

// IME sends some fields as strings or numbers inconsistently.
func scalar(m json.RawMessage) string {
	s := strings.TrimSpace(string(m))
	if s == "null" {
		return ""
	}
	return strings.Trim(s, `"`)
}
