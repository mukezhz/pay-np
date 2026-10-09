package hamropay_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/hamropay"
)

const secret, hookSecret = "client-secret", "hook-secret"

func sig(key string, parts ...string) string {
	m := hmac.New(sha512.New, []byte(key))
	m.Write([]byte(strings.Join(parts, ",")))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

func setup(t *testing.T, txnBody string) *hamropay.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Client-Id") != "cid" || r.Header.Get("Client-API-Key") != "key" || !r.Close {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/v1/checkout/sessionId":
			if r.Header.Get("Signature") != sig(secret, "ord-1", "1000", "M1", "cid", "key") || body["transactionAmount"] != "1000" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"code":3,"message":"bad signature"}`))
				return
			}
			_, _ = w.Write([]byte(`{"sessionId":"S1","merchantId":"M1"}`))
		case "/v1/checkout/transaction":
			if r.Header.Get("Signature") != sig(secret, "ord-1", "M1", "cid", "key") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(txnBody))
		}
	}))
	t.Cleanup(srv.Close)
	c, err := hamropay.New(hamropay.Config{MerchantID: "M1", ClientID: "cid", ClientAPIKey: "key", ClientSecret: secret, WebhookSecret: hookSecret,
		APIBaseURL: srv.URL, GatewayURL: "https://gw"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInitiate(t *testing.T) {
	co, err := setup(t, "").Initiate(context.Background(), paynp.InitiateRequest{TxnID: "ord-1", Amount: 1000, SuccessURL: "https://m/ok"})
	if err != nil {
		t.Fatal(err)
	}
	if co.Method != http.MethodPost || co.URL != "https://gw/api/checkout" || co.ProviderRef != "S1" ||
		co.Fields["token"] != sig(secret, "M1", "ord-1", "S1", "1000", "cid", "key") {
		t.Fatalf("unexpected checkout %+v", co)
	}
	if _, err := setup(t, "").Initiate(context.Background(), paynp.InitiateRequest{TxnID: "ord-1", Amount: 999, SuccessURL: "https://m/ok"}); !errors.Is(err, paynp.ErrInvalidRequest) {
		t.Fatalf("below Rs 10 must be rejected, got %v", err)
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		body   string
		amount paynp.Paisa
		want   paynp.Status
		err    bool
	}{
		{`{"merchantTransactionId":"ord-1","status":"COMPLETED","amount":10.0}`, 1000, paynp.StatusSuccess, false},
		{`{"merchantTxnId":"ord-1","status":"COMPLETED","amount":10}`, 1000, paynp.StatusSuccess, false},
		{`{"merchantTransactionId":"ord-1","status":"COMPLETED","amount":1.0}`, 1000, "", true},
		{`{"merchantTransactionId":"other","status":"COMPLETED","amount":10.0}`, 1000, "", true},
		{`{"merchantTransactionId":"ord-1","status":"NOT_INITIATED","amount":0}`, 1000, paynp.StatusNotFound, false},
		{`{"merchantTransactionId":"ord-1","status":"PROCESSING","amount":0}`, 1000, paynp.StatusPending, false},
		{`{"merchantTransactionId":"ord-1","status":"FAILED","amount":0}`, 1000, paynp.StatusFailed, false},
	}
	for _, tc := range cases {
		tx, err := setup(t, tc.body).Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: tc.amount})
		if tc.err {
			if err == nil || tx != nil {
				t.Errorf("%s: want error and no tx, got %+v %v", tc.body, tx, err)
			}
			continue
		}
		if err != nil || tx.Status != tc.want {
			t.Errorf("%s: got %+v %v, want %s", tc.body, tx, err, tc.want)
		}
	}
}

func TestParseCallback(t *testing.T) {
	cb, err := setup(t, "").ParseCallback(url.Values{"MerchantTxnId": {"ord-1"}})
	if err != nil || cb.TxnID != "ord-1" || cb.Status != paynp.StatusPending {
		t.Fatalf("unexpected %+v %v", cb, err)
	}
}

func TestParseWebhook(t *testing.T) {
	c := setup(t, "")
	body := []byte(`{"merchantTxnId":"ord-1","merchantId":"M1","amount":1200.0,"status":"COMPLETED","metadata":{"order":"42"}}`)
	for _, signedAs := range []string{"1200", "1200.0", "1200.00"} {
		h := http.Header{"Signature": {sig(hookSecret, "ord-1", "M1", "COMPLETED", signedAs)}}
		cb, err := c.ParseWebhook(h, body)
		if err != nil || cb.Status != paynp.StatusSuccess || cb.Amount != 120000 || cb.Values.Get("order") != "42" {
			t.Fatalf("signed as %s: %+v %v", signedAs, cb, err)
		}
	}
	forged := http.Header{"Signature": {sig("wrong", "ord-1", "M1", "COMPLETED", "1200")}}
	if _, err := c.ParseWebhook(forged, body); !errors.Is(err, paynp.ErrInvalidSignature) {
		t.Fatalf("want ErrInvalidSignature, got %v", err)
	}
}

func TestProductionNeedsURLs(t *testing.T) {
	_, err := hamropay.New(hamropay.Config{MerchantID: "M", ClientID: "c", ClientAPIKey: "k", ClientSecret: "s", Environment: paynp.Production})
	if !errors.Is(err, paynp.ErrInvalidConfig) {
		t.Fatalf("want ErrInvalidConfig, got %v", err)
	}
}
