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

// Response shape from eSewa's mobile SDK docs.
const docsBody = `[{"productId":"ord-1","productName":"Android SDK Payment","totalAmount":"25.0","code":"00",
"message":{"technicalSuccessMessage":"Your transaction has been completed.","successMessage":"Your transaction has been completed."},
"transactionDetails":{"date":"Mon Dec 26 12:58:14 NPT 2022","referenceId":"0004VZR","status":"COMPLETE"},"merchantName":"Android SDK Payment"}]`

func mobile(t *testing.T, body string, gotQuery *url.Values) *esewa.MobileClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("merchantId") != esewa.SandboxMobileClientID || r.Header.Get("merchantSecret") != esewa.SandboxMobileClientSecret {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if gotQuery != nil {
			*gotQuery = r.URL.Query()
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := esewa.NewMobile(esewa.MobileConfig{ClientID: esewa.SandboxMobileClientID, ClientSecret: esewa.SandboxMobileClientSecret, VerifyURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestMobileVerifyByRefID(t *testing.T) {
	var q url.Values
	tx, err := mobile(t, docsBody, &q).Verify(context.Background(), esewa.MobileVerifyRequest{ProductID: "ord-1", RefID: "0004VZR", Amount: 2500})
	if err != nil || tx.Status != paynp.StatusSuccess || tx.ProviderRef != "0004VZR" || tx.Amount != 2500 {
		t.Fatalf("unexpected %+v %v", tx, err)
	}
	if q.Get("txnRefId") != "0004VZR" || q.Has("productId") {
		t.Fatalf("query %v", q)
	}
}

func TestMobileVerifyByProduct(t *testing.T) {
	var q url.Values
	if _, err := mobile(t, docsBody, &q).Verify(context.Background(), esewa.MobileVerifyRequest{ProductID: "ord-1", Amount: 2500}); err != nil {
		t.Fatal(err)
	}
	if q.Get("productId") != "ord-1" || q.Get("amount") != "25" {
		t.Fatalf("query %v", q)
	}
}

func TestMobileVerifyRejectsReplayAndTampering(t *testing.T) {
	// Valid refId, but it paid for another order.
	tx, err := mobile(t, docsBody, nil).Verify(context.Background(), esewa.MobileVerifyRequest{ProductID: "ord-2", RefID: "0004VZR", Amount: 2500})
	if err != nil || tx.Status != paynp.StatusNotFound {
		t.Fatalf("other order's refId must not verify: %+v %v", tx, err)
	}
	// Right order, smaller payment.
	tx, err = mobile(t, docsBody, nil).Verify(context.Background(), esewa.MobileVerifyRequest{ProductID: "ord-1", RefID: "0004VZR", Amount: 10000})
	if !errors.Is(err, paynp.ErrAmountMismatch) || tx != nil {
		t.Fatalf("want ErrAmountMismatch, got %+v %v", tx, err)
	}
}

func TestMobileVerifyUnknownRef(t *testing.T) {
	// Observed from the sandbox for an unknown refId: HTTP 200 with an object.
	body := `{"message":{"errorMessage":"Error occurred. Please try again later.","technicalErrorMessage":"Error occurred. Please try again later."}}`
	tx, err := mobile(t, body, nil).Verify(context.Background(), esewa.MobileVerifyRequest{ProductID: "ord-1", RefID: "nope", Amount: 2500})
	if err != nil || tx.Status != paynp.StatusNotFound || tx.Status.Final() {
		t.Fatalf("unexpected %+v %v", tx, err)
	}
	if _, err := mobile(t, `<html>`, nil).Verify(context.Background(), esewa.MobileVerifyRequest{ProductID: "ord-1", Amount: 2500}); err == nil {
		t.Fatal("garbage must be an error")
	}
}

func TestMobileVerifyValidates(t *testing.T) {
	if _, err := mobile(t, docsBody, nil).Verify(context.Background(), esewa.MobileVerifyRequest{RefID: "0004VZR", Amount: 2500}); !errors.Is(err, paynp.ErrInvalidRequest) {
		t.Fatalf("ProductID must be required, got %v", err)
	}
}
