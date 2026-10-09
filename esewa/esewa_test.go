package esewa_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/esewa"
)

// Real sandbox callback: COMPLETE, 133.0, uuid 1234567890, signed with EPAYTEST secret.
const sandboxData = "eyJ0cmFuc2FjdGlvbl9jb2RlIjoiMDAwNlRZMyIsInN0YXR1cyI6IkNPTVBMRVRFIiwidG90YWxfYW1vdW50IjoiMTMzLjAiLCJ0cmFuc2FjdGlvbl91dWlkIjoiMTIzNDU2Nzg5MCIsInByb2R1Y3RfY29kZSI6IkVQQVlURVNUIiwic2lnbmVkX2ZpZWxkX25hbWVzIjoidHJhbnNhY3Rpb25fY29kZSxzdGF0dXMsdG90YWxfYW1vdW50LHRyYW5zYWN0aW9uX3V1aWQscHJvZHVjdF9jb2RlLHNpZ25lZF9maWVsZF9uYW1lcyIsInNpZ25hdHVyZSI6Ik1GRWNNWi8zMFdWZXphblZaSEg0SDFuSVY4cEd3eXpaeGdndGt5ZTJWWHc9In0="

func newClient(t *testing.T, statusURL string) *esewa.Client {
	t.Helper()
	c, err := esewa.New(esewa.Config{ProductCode: esewa.SandboxProductCode, SecretKey: esewa.SandboxSecretKey, StatusURL: statusURL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInitiateSignsDocumentedExample(t *testing.T) {
	c := newClient(t, "")
	// eSewa docs: total_amount=110,transaction_uuid=241028,product_code=EPAYTEST
	co, err := c.Initiate(context.Background(), paynp.InitiateRequest{
		TxnID: "241028", Amount: 11000, SuccessURL: "https://m/s", FailureURL: "https://m/f",
	})
	if err != nil {
		t.Fatal(err)
	}
	if co.Method != http.MethodPost || co.Fields["total_amount"] != "110" {
		t.Fatalf("unexpected checkout %+v", co)
	}
	if got, want := co.Fields["signature"], "i94zsd3oXF6ZsSr/kGqT4sSzYQzjj1W/waxjWyRwaME="; got != want {
		t.Fatalf("signature %s, want %s", got, want)
	}
}

func TestParseCallbackVerifiesSandboxSignature(t *testing.T) {
	cb, err := newClient(t, "").ParseCallback(url.Values{"data": {sandboxData}})
	if err != nil {
		t.Fatal(err)
	}
	if cb.TxnID != "1234567890" || cb.Status != paynp.StatusSuccess || cb.Amount != 13300 || cb.ProviderRef != "0006TY3" {
		t.Fatalf("unexpected callback %+v", cb)
	}
}

func TestParseCallbackRejectsWrongSecret(t *testing.T) {
	c, _ := esewa.New(esewa.Config{ProductCode: "EPAYTEST", SecretKey: "wrong"})
	_, err := c.ParseCallback(url.Values{"data": {sandboxData}})
	if !errors.Is(err, paynp.ErrInvalidSignature) {
		t.Fatalf("want ErrInvalidSignature, got %v", err)
	}
}

func TestLookupLegacyShapeAndOutage(t *testing.T) {
	body := `{"pid":"x","scd":"EPAYTEST","totalAmount":150.0,"status":"PENDING","refId":null}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	tx, err := newClient(t, srv.URL).Lookup(context.Background(), paynp.LookupRequest{TxnID: "x", Amount: 15000})
	if err != nil || tx.Status != paynp.StatusPending || tx.Amount != 15000 {
		t.Fatalf("unexpected %+v %v", tx, err)
	}

	body = `{"code":0,"error_message":"Service is currently unavailable"}`
	_, err = newClient(t, srv.URL).Lookup(context.Background(), paynp.LookupRequest{TxnID: "x", Amount: 15000})
	var apiErr *paynp.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
}

func TestLookup(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"product_code":"EPAYTEST","transaction_uuid":"abc","total_amount":100.0,"status":"COMPLETE","ref_id":"0001TS9"}`))
	}))
	defer srv.Close()

	tx, err := newClient(t, srv.URL).Lookup(context.Background(), paynp.LookupRequest{TxnID: "abc", Amount: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != paynp.StatusSuccess || tx.ProviderRef != "0001TS9" || tx.Amount != 10000 {
		t.Fatalf("unexpected tx %+v", tx)
	}
	if gotQuery.Get("total_amount") != "100" || gotQuery.Get("transaction_uuid") != "abc" {
		t.Fatalf("unexpected query %v", gotQuery)
	}

	_, err = newClient(t, srv.URL).Lookup(context.Background(), paynp.LookupRequest{TxnID: "abc", Amount: 20000})
	if !errors.Is(err, paynp.ErrAmountMismatch) {
		t.Fatalf("want ErrAmountMismatch, got %v", err)
	}
}
