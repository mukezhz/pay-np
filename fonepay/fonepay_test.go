package fonepay_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/fonepay"
)

const secret = "a7e3512f5032480a83137793cb2021dc"

func hmac512(parts ...string) string {
	m := hmac.New(sha512.New, []byte(secret))
	m.Write([]byte(strings.Join(parts, ",")))
	return hex.EncodeToString(m.Sum(nil))
}

func client(t *testing.T, host string) *fonepay.Client {
	t.Helper()
	c, err := fonepay.New(fonepay.Config{MerchantCode: "NBQM", SecretKey: secret, Host: host,
		Now: func() time.Time { return time.Date(2025, 12, 8, 20, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInitiate(t *testing.T) {
	co, err := client(t, "https://f").Initiate(context.Background(), paynp.InitiateRequest{TxnID: "ord-1", Amount: 150000, Description: "Fee", SuccessURL: "https://m/r"})
	if err != nil {
		t.Fatal(err)
	}
	want := hmac512("NBQM", "P", "ord-1", "1500.00", "NPR", "12/09/2025", "Fee", "N/A", "https://m/r")
	if co.Method != http.MethodGet || co.Fields["DV"] != want || co.Fields["DT"] != "12/09/2025" {
		t.Fatalf("unexpected checkout %+v", co.Fields)
	}
}

func callback() url.Values {
	q := url.Values{"PRN": {"ord-1"}, "PID": {"NBQM"}, "PS": {"true"}, "RC": {"successful"}, "UID": {"U1"}, "BC": {"NMB"}, "INI": {"x"}, "P_AMT": {"1500.00"}, "R_AMT": {"1500.00"}}
	q.Set("DV", strings.ToUpper(hmac512("ord-1", "NBQM", "true", "successful", "U1", "NMB", "x", "1500.00", "1500.00")))
	return q
}

func TestParseCallback(t *testing.T) {
	cb, err := client(t, "https://f").ParseCallback(callback())
	if err != nil || cb.Status != paynp.StatusSuccess || cb.ProviderRef != "U1" || cb.Amount != 150000 {
		t.Fatalf("unexpected %+v %v", cb, err)
	}
	forged := callback()
	forged.Set("P_AMT", "1.00")
	if _, err := client(t, "https://f").ParseCallback(forged); !errors.Is(err, paynp.ErrInvalidSignature) {
		t.Fatalf("want ErrInvalidSignature, got %v", err)
	}
}

func TestLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("DV") != hmac512("NBQM", "1500.00", "ord-1", "", "U1") {
			_, _ = w.Write([]byte(`<response><success>false</success><response_code>failed</response_code></response>`))
			return
		}
		_, _ = w.Write([]byte(`<response><success>true</success><message>ok</message><response_code>successful</response_code><txnAmount>1500.00</txnAmount></response>`))
	}))
	defer srv.Close()
	c := client(t, srv.URL)
	cb, _ := c.ParseCallback(callback())

	tx, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 150000, Callback: cb})
	if err != nil || tx.Status != paynp.StatusSuccess {
		t.Fatalf("unexpected %+v %v", tx, err)
	}
	tx, err = c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 100, Callback: cb})
	if err != nil || tx.Status != paynp.StatusPending {
		t.Fatalf("wrong amount must fail verification: %+v %v", tx, err)
	}
	if _, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 150000}); !errors.Is(err, paynp.ErrCallbackRequired) {
		t.Fatalf("want ErrCallbackRequired, got %v", err)
	}
}
