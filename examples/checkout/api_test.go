package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	paynp "github.com/mukezhz/pay-np"
)

type fakeProvider struct{ paid bool }

func (f *fakeProvider) Name() paynp.ProviderName { return "fake" }

func (f *fakeProvider) Initiate(_ context.Context, req paynp.InitiateRequest) (*paynp.Checkout, error) {
	return &paynp.Checkout{Method: http.MethodPost, URL: "https://fake/pay", Fields: map[string]string{"txn": req.TxnID}, ProviderRef: "REF1"}, nil
}

func (f *fakeProvider) ParseCallback(q url.Values) (*paynp.Callback, error) {
	return &paynp.Callback{TxnID: q.Get("txn"), Status: paynp.StatusPending, Values: q}, nil
}

func (f *fakeProvider) Lookup(_ context.Context, req paynp.LookupRequest) (*paynp.Transaction, error) {
	if !f.paid {
		return &paynp.Transaction{TxnID: req.TxnID, Status: paynp.StatusPending}, nil
	}
	return &paynp.Transaction{TxnID: req.TxnID, Status: paynp.StatusSuccess, Amount: req.Amount}, nil
}

func TestMobilePaymentFlow(t *testing.T) {
	fake := &fakeProvider{}
	s := &shop{baseURL: "http://shop", providers: map[paynp.ProviderName]paynp.Provider{"fake": fake}, attempts: map[string]*attempt{}, appSchemes: []string{"paynp"}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/payments", s.apiCreatePayment)
	mux.HandleFunc("GET /api/payments/{txn}", s.apiGetPayment)
	mux.HandleFunc("GET /pay/{txn}", s.openCheckout)
	mux.HandleFunc("/return/{provider}/{txn}", s.handleReturn)
	do := func(method, target, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
		return rec
	}

	rec := do("POST", "/api/payments", `{"provider":"fake","amount":"250","app_return_url":"paynp://paid?order=7"}`)
	var created paymentView
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if rec.Code != http.StatusCreated || created.ProviderRef != "REF1" || created.CheckoutURL != "http://shop/pay/"+created.TxnID {
		t.Fatalf("create: %d %+v", rec.Code, created)
	}

	if rec = do("GET", "/pay/"+created.TxnID, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `action="https://fake/pay"`) {
		t.Fatalf("open checkout: %d %s", rec.Code, rec.Body)
	}

	fake.paid = true
	rec = do("GET", "/return/fake/"+created.TxnID, "")
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if loc.Scheme != "paynp" || loc.Query().Get("txn_id") != created.TxnID || loc.Query().Get("status") != "SUCCESS" || loc.Query().Get("order") != "7" {
		t.Fatalf("app redirect: %s", rec.Header().Get("Location"))
	}

	var got paymentView
	_ = json.Unmarshal(do("GET", "/api/payments/"+created.TxnID, "").Body.Bytes(), &got)
	if got.Status != paynp.StatusSuccess || !got.Final || got.CheckoutURL != "" {
		t.Fatalf("status: %+v", got)
	}

	if rec = do("POST", "/api/payments", `{"provider":"fake","amount":"250","app_return_url":"https://evil.example"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("disallowed return scheme accepted: %d", rec.Code)
	}
}
