// Package connectips implements paynp.Provider for NCHL ConnectIPS e-payment.
package connectips

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/internal/httpx"
	"golang.org/x/crypto/pkcs12"
)

var hosts = map[paynp.Environment]string{
	paynp.Sandbox:    "https://uat.connectips.com",
	paynp.Production: "https://login.connectips.com",
}

// nepalTime avoids depending on the host's tzdata; Nepal has no DST.
var nepalTime = time.FixedZone("NPT", 5*3600+45*60)

type Client struct {
	cfg        Config
	merchantID int64
	http       *http.Client
}

var _ paynp.Provider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	if cfg.MerchantID == "" || cfg.AppID == "" || cfg.AppName == "" || cfg.Password == "" || cfg.PrivateKey == nil {
		return nil, fmt.Errorf("%w: connectips needs MerchantID, AppID, AppName, Password and PrivateKey", paynp.ErrInvalidConfig)
	}
	mid, err := strconv.ParseInt(cfg.MerchantID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: connectips MerchantID must be numeric", paynp.ErrInvalidConfig)
	}
	if cfg.Host == "" {
		var ok bool
		if cfg.Host, ok = hosts[cfg.Environment]; !ok {
			return nil, fmt.Errorf("%w: connectips unknown environment", paynp.ErrInvalidConfig)
		}
	}
	cfg.Host = strings.TrimRight(cfg.Host, "/")
	if cfg.Username == "" {
		cfg.Username = cfg.AppID
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Client{cfg: cfg, merchantID: mid, http: httpx.DefaultClient(cfg.HTTPClient)}, nil
}

func ParsePFX(data []byte, password string) (*rsa.PrivateKey, error) {
	key, _, err := pkcs12.Decode(data, password)
	if err != nil {
		return nil, fmt.Errorf("%w: connectips pfx: %v", paynp.ErrInvalidConfig, err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: connectips pfx key is not RSA", paynp.ErrInvalidConfig)
	}
	return rsaKey, nil
}

func (c *Client) Name() paynp.ProviderName { return paynp.ConnectIPS }

// Return URLs are registered with NCHL, so the request's URLs are ignored.
func (c *Client) Initiate(_ context.Context, req paynp.InitiateRequest) (*paynp.Checkout, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if len(req.TxnID) > 20 {
		return nil, fmt.Errorf("%w: connectips TxnID must be ≤ 20 chars", paynp.ErrInvalidRequest)
	}
	desc := req.Description
	if desc == "" {
		desc = req.TxnID
	}
	f := [][2]string{
		{"MERCHANTID", c.cfg.MerchantID},
		{"APPID", c.cfg.AppID},
		{"APPNAME", c.cfg.AppName},
		{"TXNID", req.TxnID},
		{"TXNDATE", c.cfg.Now().In(nepalTime).Format("02-01-2006")},
		{"TXNCRNCY", "NPR"},
		{"TXNAMT", strconv.FormatInt(int64(req.Amount), 10)},
		{"REFERENCEID", req.TxnID},
		{"REMARKS", cut(desc, 50)},
		{"PARTICULARS", cut(desc, 100)},
	}
	fields := make(map[string]string, len(f)+1)
	parts := make([]string, 0, len(f)+1)
	for _, kv := range f {
		fields[kv[0]] = kv[1]
		parts = append(parts, kv[0]+"="+kv[1])
	}
	token, err := c.sign(strings.Join(append(parts, "TOKEN=TOKEN"), ","))
	if err != nil {
		return nil, err
	}
	fields["TOKEN"] = token
	return &paynp.Checkout{
		Provider: paynp.ConnectIPS,
		Method:   http.MethodPost,
		URL:      c.cfg.Host + "/connectipswebgw/loginpage",
		Fields:   fields,
	}, nil
}

func (c *Client) ParseCallback(query url.Values) (*paynp.Callback, error) {
	id := query.Get("TXNID")
	if id == "" {
		return nil, fmt.Errorf("%w: connectips TXNID missing", paynp.ErrInvalidCallback)
	}
	return &paynp.Callback{TxnID: id, Status: paynp.StatusPending, Values: query}, nil
}

func (c *Client) Lookup(ctx context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	if req.TxnID == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("%w: connectips lookup needs TxnID and Amount", paynp.ErrInvalidRequest)
	}
	amt := int64(req.Amount)
	token, err := c.sign(fmt.Sprintf("MERCHANTID=%s,APPID=%s,REFERENCEID=%s,TXNAMT=%d", c.cfg.MerchantID, c.cfg.AppID, req.TxnID, amt))
	if err != nil {
		return nil, err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.cfg.Username + ":" + c.cfg.Password))
	_, raw, err := httpx.Do(ctx, c.http, paynp.ConnectIPS, httpx.Request{
		Method: http.MethodPost,
		URL:    c.cfg.Host + "/connectipswebws/api/creditor/gettxndetail",
		Header: http.Header{"Authorization": {"Basic " + auth}},
		JSON:   txnRequest{MerchantID: c.merchantID, AppID: c.cfg.AppID, ReferenceID: req.TxnID, TxnAmt: amt, Token: token},
	})
	if err != nil {
		return nil, err
	}
	var r txnDetail
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("connectips: decode txn detail: %w", err)
	}
	if r.ReferenceID != "" && r.ReferenceID != req.TxnID {
		return nil, &paynp.APIError{Provider: paynp.ConnectIPS, StatusCode: http.StatusOK, Body: raw}
	}
	if r.TxnAmt != math.Trunc(r.TxnAmt) {
		return nil, fmt.Errorf("%w: connectips txnAmt %v is not whole paisa", paynp.ErrAmountMismatch, r.TxnAmt)
	}
	tx := &paynp.Transaction{
		Provider: paynp.ConnectIPS,
		TxnID:    req.TxnID,
		Status:   mapStatus(r.StatusDesc),
		Amount:   paynp.Paisa(r.TxnAmt),
		Raw:      raw,
	}
	if r.TxnID != 0 {
		tx.ProviderRef = strconv.FormatInt(r.TxnID, 10)
	}
	return paynp.MatchAmount(tx, req.Amount)
}

func (c *Client) sign(payload string) (string, error) {
	digest := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.cfg.PrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("connectips: sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func mapStatus(desc string) paynp.Status {
	switch strings.ToUpper(strings.TrimSpace(desc)) {
	case "TRANSACTION SUCCESSFUL":
		return paynp.StatusSuccess
	case "TRANSACTION UNSUCCESSFUL":
		return paynp.StatusFailed
	case "TRANSACTION NOT FOUND":
		return paynp.StatusNotFound
	default: // TRANSACTION INCOMPLETE and anything new
		return paynp.StatusPending
	}
}

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
