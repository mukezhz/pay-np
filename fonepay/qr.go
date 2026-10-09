package fonepay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/internal/httpx"
)

var qrHosts = map[paynp.Environment]string{
	paynp.Sandbox:    "https://dev-merchantapi.fonepay.com/convergent-merchantWeb",
	paynp.Production: "https://merchantapi.fonepay.com",
}

const qrPath = "/api/merchant/merchantDetailsForThirdParty/"

// GenerateQR creates a dynamic QR for in-person payment (no public spec; verify in dev).
func (c *Client) GenerateQR(ctx context.Context, req paynp.InitiateRequest) (*QR, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	host, err := c.qrHost()
	if err != nil {
		return nil, err
	}
	r1 := cut(req.Description, 160)
	if r1 == "" {
		r1 = req.TxnID
	}
	body := qrRequest{
		Amount:       req.Amount.Rupees(),
		Remarks1:     r1,
		Remarks2:     "N/A",
		PRN:          req.TxnID,
		MerchantCode: c.cfg.MerchantCode,
		Username:     c.cfg.Username,
		Password:     c.cfg.Password,
	}
	body.DataValidation = c.sign(body.Amount, body.PRN, body.MerchantCode, body.Remarks1, body.Remarks2)
	_, raw, err := httpx.Do(ctx, c.http, paynp.Fonepay, httpx.Request{Method: http.MethodPost, URL: host + qrPath + "thirdPartyDynamicQrDownload", JSON: body})
	if err != nil {
		return nil, err
	}
	var r qrResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("fonepay: decode qr: %w", err)
	}
	if !r.Success || r.QRMessage == "" {
		return nil, &paynp.APIError{Provider: paynp.Fonepay, StatusCode: http.StatusOK, Body: raw}
	}
	return &QR{TxnID: req.TxnID, Message: r.QRMessage, WebSocketURL: r.WebSocketURL}, nil
}

// QRStatus reports a dynamic QR payment. The API omits the amount, so a SUCCESS
// carries req.Amount from your records.
func (c *Client) QRStatus(ctx context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	if req.TxnID == "" {
		return nil, fmt.Errorf("%w: fonepay QRStatus needs TxnID", paynp.ErrInvalidRequest)
	}
	host, err := c.qrHost()
	if err != nil {
		return nil, err
	}
	body := qrStatusRequest{
		PRN:            req.TxnID,
		MerchantCode:   c.cfg.MerchantCode,
		DataValidation: c.sign(req.TxnID, c.cfg.MerchantCode),
		Username:       c.cfg.Username,
		Password:       c.cfg.Password,
	}
	_, raw, err := httpx.Do(ctx, c.http, paynp.Fonepay, httpx.Request{Method: http.MethodPost, URL: host + qrPath + "thirdPartyDynamicQrGetStatus", JSON: body})
	if err != nil {
		return nil, err
	}
	var r qrStatusResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("fonepay: decode qr status: %w", err)
	}
	tx := &paynp.Transaction{Provider: paynp.Fonepay, TxnID: req.TxnID, Status: paynp.StatusPending, Raw: raw}
	if r.FonepayTraceID != nil {
		tx.ProviderRef = fmt.Sprint(r.FonepayTraceID)
	}
	switch strings.ToLower(r.PaymentStatus) {
	case "success":
		tx.Status, tx.Amount = paynp.StatusSuccess, req.Amount
	case "failed":
		tx.Status = paynp.StatusFailed
	}
	return tx, nil
}

func (c *Client) qrHost() (string, error) {
	if c.cfg.Username == "" || c.cfg.Password == "" {
		return "", fmt.Errorf("%w: fonepay QR needs Username and Password", paynp.ErrInvalidConfig)
	}
	if c.cfg.QRHost != "" {
		return strings.TrimRight(c.cfg.QRHost, "/"), nil
	}
	if h, ok := qrHosts[c.cfg.Environment]; ok {
		return h, nil
	}
	return "", fmt.Errorf("%w: fonepay unknown environment", paynp.ErrInvalidConfig)
}
