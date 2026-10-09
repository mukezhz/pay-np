package khalti_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/khalti"
)

func fakeKhalti(t *testing.T, lookupStatus int, lookupBody string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/epayment/initiate/":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["amount"] != float64(1000) || body["purchase_order_id"] != "ord-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"pidx":"P1","payment_url":"https://pay.khalti.com/?pidx=P1","expires_at":"2026-10-09T16:51:03.140744+05:45","expires_in":1800}`))
		case "/epayment/lookup/":
			w.WriteHeader(lookupStatus)
			_, _ = w.Write([]byte(lookupBody))
		}
	}))
}

func client(t *testing.T, base string) *khalti.Client {
	t.Helper()
	c, err := khalti.New(khalti.Config{SecretKey: "secret", WebsiteURL: "https://m", BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInitiate(t *testing.T) {
	srv := fakeKhalti(t, 200, "")
	defer srv.Close()
	co, err := client(t, srv.URL).Initiate(context.Background(), paynp.InitiateRequest{TxnID: "ord-1", Amount: 1000, SuccessURL: "https://m/r"})
	if err != nil {
		t.Fatal(err)
	}
	if co.Method != http.MethodGet || co.ProviderRef != "P1" || co.ExpiresAt.IsZero() {
		t.Fatalf("unexpected checkout %+v", co)
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		name   string
		code   int
		body   string
		amount paynp.Paisa
		want   paynp.Status
		err    error
	}{
		{"completed", 200, `{"pidx":"P1","total_amount":1000,"status":"Completed","transaction_id":"T1"}`, 1000, paynp.StatusSuccess, nil},
		{"tampered amount", 200, `{"pidx":"P1","total_amount":10,"status":"Completed"}`, 1000, paynp.StatusSuccess, paynp.ErrAmountMismatch},
		{"expired via 400", 400, `{"pidx":"P1","total_amount":1000,"status":"Expired"}`, 1000, paynp.StatusExpired, nil},
		{"canceled", 400, `{"pidx":"P1","total_amount":1000,"status":"User canceled"}`, 1000, paynp.StatusCanceled, nil},
		{"unknown pidx", 404, `{"detail":"Not found.","status_code":404}`, 1000, paynp.StatusNotFound, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeKhalti(t, tc.code, tc.body)
			defer srv.Close()
			tx, err := client(t, srv.URL).Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: tc.amount, ProviderRef: "P1"})
			if !errors.Is(err, tc.err) {
				t.Fatalf("err %v, want %v", err, tc.err)
			}
			if tc.err != nil {
				if tx != nil {
					t.Fatal("tx must be withheld on error")
				}
				return
			}
			if tx.Status != tc.want {
				t.Fatalf("status %s, want %s", tx.Status, tc.want)
			}
		})
	}
}

func TestLookupServerErrorIsAPIError(t *testing.T) {
	srv := fakeKhalti(t, 500, "boom")
	defer srv.Close()
	_, err := client(t, srv.URL).Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 1000, ProviderRef: "P1"})
	var apiErr *paynp.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 {
		t.Fatalf("want APIError 500, got %v", err)
	}
}

func TestParseCallback(t *testing.T) {
	cb, err := client(t, "http://x").ParseCallback(url.Values{"pidx": {"P1"}, "status": {"Completed"}, "purchase_order_id": {"ord-1"}, "total_amount": {"1000"}})
	if err != nil || cb.TxnID != "ord-1" || cb.Status != paynp.StatusPending || cb.Amount != 1000 {
		t.Fatalf("unexpected %+v %v", cb, err)
	}
}
