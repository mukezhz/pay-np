package connectips_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/connectips"
)

func verify(t *testing.T, key *rsa.PrivateKey, payload, sig string) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(payload))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], raw); err != nil {
		t.Fatalf("signature does not cover %q: %v", payload, err)
	}
}

func setup(t *testing.T, handler http.HandlerFunc) (*connectips.Client, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	host := "http://unused"
	if handler != nil {
		srv := httptest.NewServer(handler)
		t.Cleanup(srv.Close)
		host = srv.URL
	}
	c, err := connectips.New(connectips.Config{
		MerchantID: "550", AppID: "MER-550-APP-1", AppName: "App", Password: "pw", PrivateKey: key, Host: host,
		Now: func() time.Time { return time.Date(2025, 12, 8, 20, 0, 0, 0, time.UTC) }, // 9 Dec in Nepal
	})
	if err != nil {
		t.Fatal(err)
	}
	return c, key
}

func TestInitiateSignsLoginForm(t *testing.T) {
	c, key := setup(t, nil)
	co, err := c.Initiate(context.Background(), paynp.InitiateRequest{TxnID: "txn-123", Amount: 50000, Description: "Fee"})
	if err != nil {
		t.Fatal(err)
	}
	if co.Method != http.MethodPost || co.Fields["TXNDATE"] != "09-12-2025" || co.Fields["TXNAMT"] != "50000" {
		t.Fatalf("unexpected checkout %+v", co)
	}
	verify(t, key, "MERCHANTID=550,APPID=MER-550-APP-1,APPNAME=App,TXNID=txn-123,TXNDATE=09-12-2025,TXNCRNCY=NPR,TXNAMT=50000,REFERENCEID=txn-123,REMARKS=Fee,PARTICULARS=Fee,TOKEN=TOKEN", co.Fields["TOKEN"])
}

func TestLookup(t *testing.T) {
	var key *rsa.PrivateKey
	var c *connectips.Client
	c, key = setup(t, func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "MER-550-APP-1" || p != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body struct {
			ReferenceID string `json:"referenceId"`
			TxnAmt      int64  `json:"txnAmt"`
			Token       string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		verify(t, key, "MERCHANTID=550,APPID=MER-550-APP-1,REFERENCEID=txn-123,TXNAMT=50000", body.Token)
		_, _ = w.Write([]byte(`{"status":"SUCCESS","statusDesc":"TRANSACTION SUCCESSFUL","referenceId":"txn-123","txnAmt":50000.0,"txnId":987}`))
	})

	tx, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "txn-123", Amount: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != paynp.StatusSuccess || tx.ProviderRef != "987" || tx.Amount != 50000 {
		t.Fatalf("unexpected tx %+v", tx)
	}
}

func TestLookupUnauthorizedIsAPIError(t *testing.T) {
	c, _ := setup(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	_, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "txn-123", Amount: 50000})
	var apiErr *paynp.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Fatalf("want APIError 401, got %v", err)
	}
}
