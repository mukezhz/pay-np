package esewa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/internal/httpx"
)

const (
	SandboxMobileClientID     = "JB0BBQ4aD0UqIThFJwAKBgAXEUkEGQUBBAwdOgABHD4DChwUAB0R"
	SandboxMobileClientSecret = "BhwIWQQADhIYSxILExMcAgFXFhcOBwAKBgAXEQ=="
)

var mobileVerifyURLs = map[paynp.Environment]string{
	paynp.Sandbox:    "https://rc.esewa.com.np/mobile/transaction",
	paynp.Production: "https://esewa.com.np/mobile/transaction",
}

type MobileConfig struct {
	ClientID     string
	ClientSecret string
	Environment  paynp.Environment
	VerifyURL    string
	HTTPClient   *http.Client
}

// MobileClient verifies payments made in-app with eSewa's Android/iOS/Flutter SDK.
type MobileClient struct {
	cfg  MobileConfig
	http *http.Client
}

func NewMobile(cfg MobileConfig) (*MobileClient, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("%w: esewa mobile needs ClientID and ClientSecret", paynp.ErrInvalidConfig)
	}
	if cfg.VerifyURL == "" {
		var ok bool
		if cfg.VerifyURL, ok = mobileVerifyURLs[cfg.Environment]; !ok {
			return nil, fmt.Errorf("%w: esewa mobile unknown environment", paynp.ErrInvalidConfig)
		}
	}
	return &MobileClient{cfg: cfg, http: httpx.DefaultClient(cfg.HTTPClient)}, nil
}

type MobileVerifyRequest struct {
	// ProductID must be unique per order; it binds the payment to the order.
	ProductID string
	RefID     string
	Amount    paynp.Paisa
}

type mobileTxn struct {
	ProductID          string      `json:"productId"`
	TotalAmount        json.Number `json:"totalAmount"`
	TransactionDetails struct {
		ReferenceID string `json:"referenceId"`
		Status      string `json:"status"`
	} `json:"transactionDetails"`
}

func (c *MobileClient) Verify(ctx context.Context, req MobileVerifyRequest) (*paynp.Transaction, error) {
	if req.ProductID == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("%w: esewa mobile verify needs ProductID and Amount", paynp.ErrInvalidRequest)
	}
	q := url.Values{"productId": {req.ProductID}, "amount": {formatAmount(req.Amount)}}
	if req.RefID != "" {
		q = url.Values{"txnRefId": {req.RefID}}
	}
	_, raw, err := httpx.Do(ctx, c.http, paynp.Esewa, httpx.Request{
		Method: http.MethodGet,
		URL:    c.cfg.VerifyURL + "?" + q.Encode(),
		Header: http.Header{"merchantId": {c.cfg.ClientID}, "merchantSecret": {c.cfg.ClientSecret}},
	})
	if err != nil {
		return nil, err
	}
	tx := &paynp.Transaction{Provider: paynp.Esewa, TxnID: req.ProductID, ProviderRef: req.RefID, Status: paynp.StatusNotFound, Raw: raw}
	var list []mobileTxn
	if err := decodeJSON(raw, &list); err != nil {
		// eSewa answers an unknown refId with 200 and an error object, not an array.
		var e struct {
			Message struct {
				ErrorMessage string `json:"errorMessage"`
			} `json:"message"`
		}
		if decodeJSON(raw, &e) == nil && e.Message.ErrorMessage != "" {
			return tx, nil
		}
		return nil, &paynp.APIError{Provider: paynp.Esewa, StatusCode: http.StatusOK, Body: raw}
	}
	for _, t := range list {
		if t.ProductID != req.ProductID || (req.RefID != "" && t.TransactionDetails.ReferenceID != req.RefID) {
			continue
		}
		tx.ProviderRef = t.TransactionDetails.ReferenceID
		tx.Status = mapStatus(t.TransactionDetails.Status)
		if tx.Amount, err = paynp.ParseRupees(t.TotalAmount.String()); err != nil {
			return nil, fmt.Errorf("esewa: mobile totalAmount %q: %w", t.TotalAmount, err)
		}
		if tx.Status == paynp.StatusSuccess {
			break
		}
	}
	return paynp.MatchAmount(tx, req.Amount)
}
