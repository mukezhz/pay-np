// Package fonepay implements paynp.Provider for the Fonepay web redirect flow.
//
// Fonepay publishes no public spec; field order and hosts are cross-checked
// against several independent integrations. Verify in Fonepay's dev environment.
package fonepay

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/internal/httpx"
)

var hosts = map[paynp.Environment]string{
	paynp.Sandbox:    "https://dev-clientapi.fonepay.com",
	paynp.Production: "https://clientapi.fonepay.com",
}

var nepalTime = time.FixedZone("NPT", 5*3600+45*60)

type Config struct {
	MerchantCode string // PID
	SecretKey    string
	Environment  paynp.Environment
	Host         string
	HTTPClient   *http.Client
	Now          func() time.Time
}

type Client struct {
	cfg  Config
	http *http.Client
}

var _ paynp.Provider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	if cfg.MerchantCode == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("%w: fonepay needs MerchantCode and SecretKey", paynp.ErrInvalidConfig)
	}
	if cfg.Host == "" {
		var ok bool
		if cfg.Host, ok = hosts[cfg.Environment]; !ok {
			return nil, fmt.Errorf("%w: fonepay unknown environment", paynp.ErrInvalidConfig)
		}
	}
	cfg.Host = strings.TrimRight(cfg.Host, "/")
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Client{cfg: cfg, http: httpx.DefaultClient(cfg.HTTPClient)}, nil
}

func (c *Client) Name() paynp.ProviderName { return paynp.Fonepay }

// Initiate builds the signed redirect. Fonepay has one return URL (SuccessURL).
func (c *Client) Initiate(_ context.Context, req paynp.InitiateRequest) (*paynp.Checkout, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.SuccessURL == "" {
		return nil, fmt.Errorf("%w: fonepay needs SuccessURL", paynp.ErrInvalidRequest)
	}
	if n := len(req.TxnID); n < 3 || n > 25 {
		return nil, fmt.Errorf("%w: fonepay TxnID (PRN) must be 3–25 chars", paynp.ErrInvalidRequest)
	}
	r1 := cut(req.Description, 160)
	if r1 == "" {
		r1 = req.TxnID
	}
	f := map[string]string{
		"PID": c.cfg.MerchantCode,
		"MD":  "P",
		"PRN": req.TxnID,
		"AMT": req.Amount.Rupees(),
		"CRN": "NPR",
		"DT":  c.cfg.Now().In(nepalTime).Format("01/02/2006"),
		"R1":  r1,
		"R2":  "N/A",
		"RU":  req.SuccessURL,
	}
	f["DV"] = c.sign(f["PID"], f["MD"], f["PRN"], f["AMT"], f["CRN"], f["DT"], f["R1"], f["R2"], f["RU"])
	return &paynp.Checkout{
		Provider: paynp.Fonepay,
		Method:   http.MethodGet,
		URL:      c.cfg.Host + "/api/merchantRequest",
		Fields:   f,
	}, nil
}

// ParseCallback verifies the DV Fonepay signs over its return query.
func (c *Client) ParseCallback(q url.Values) (*paynp.Callback, error) {
	if q.Get("PRN") == "" || q.Get("DV") == "" {
		return nil, fmt.Errorf("%w: fonepay PRN/DV missing", paynp.ErrInvalidCallback)
	}
	want := c.sign(q.Get("PRN"), q.Get("PID"), q.Get("PS"), q.Get("RC"), q.Get("UID"), q.Get("BC"), q.Get("INI"), q.Get("P_AMT"), q.Get("R_AMT"))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(q.Get("DV")))) {
		return nil, fmt.Errorf("%w: fonepay callback", paynp.ErrInvalidSignature)
	}
	status := paynp.StatusFailed
	if q.Get("PS") == "true" && strings.EqualFold(q.Get("RC"), "successful") {
		status = paynp.StatusSuccess
	}
	amount, err := paynp.ParseRupees(q.Get("P_AMT"))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", paynp.ErrInvalidCallback, err)
	}
	return &paynp.Callback{TxnID: q.Get("PRN"), ProviderRef: q.Get("UID"), Status: status, Amount: amount, Values: q}, nil
}

type verifyResponse struct {
	Success      bool   `xml:"success"`
	ResponseCode string `xml:"response_code"`
	Message      string `xml:"message"`
	TxnAmount    string `xml:"txnAmount"`
}

// Lookup calls verificationMerchant, which is keyed by the UID Fonepay returns
// on redirect: it needs req.Callback and cannot reconcile abandoned payments.
func (c *Client) Lookup(ctx context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	if req.Callback == nil || req.Callback.ProviderRef == "" {
		return nil, paynp.ErrCallbackRequired
	}
	if req.TxnID == "" || req.Amount <= 0 {
		return nil, fmt.Errorf("%w: fonepay lookup needs TxnID and Amount", paynp.ErrInvalidRequest)
	}
	amt, uid, bid := req.Amount.Rupees(), req.Callback.ProviderRef, req.Callback.Values.Get("BID")
	q := url.Values{
		"PRN": {req.TxnID},
		"PID": {c.cfg.MerchantCode},
		"BID": {bid},
		"AMT": {amt},
		"UID": {uid},
		"DV":  {c.sign(c.cfg.MerchantCode, amt, req.TxnID, bid, uid)},
	}
	_, raw, err := httpx.Do(ctx, c.http, paynp.Fonepay, httpx.Request{
		Method: http.MethodGet,
		URL:    c.cfg.Host + "/api/merchantRequest/verificationMerchant?" + q.Encode(),
	})
	if err != nil {
		return nil, err
	}
	var r verifyResponse
	if err := xml.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("fonepay: decode verification: %w", err)
	}
	// Decline codes are undocumented: anything but an explicit success stays PENDING (not final).
	tx := &paynp.Transaction{Provider: paynp.Fonepay, TxnID: req.TxnID, ProviderRef: uid, Status: paynp.StatusPending, Raw: raw}
	if r.Success && strings.EqualFold(r.ResponseCode, "successful") {
		tx.Status = paynp.StatusSuccess
		// Verification is signed over our AMT, so Fonepay already matched it; txnAmount is a second check.
		tx.Amount = req.Amount
		if r.TxnAmount != "" {
			if tx.Amount, err = paynp.ParseRupees(r.TxnAmount); err != nil {
				return nil, err
			}
		}
	}
	return paynp.MatchAmount(tx, req.Amount)
}

// sign is lower-hex HMAC-SHA512 over the comma-joined values.
func (c *Client) sign(values ...string) string {
	mac := hmac.New(sha512.New, []byte(c.cfg.SecretKey))
	mac.Write([]byte(strings.Join(values, ",")))
	return hex.EncodeToString(mac.Sum(nil))
}

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
