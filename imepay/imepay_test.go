package imepay_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/imepay"
)

func setup(t *testing.T, confirmBody string) (*imepay.Client, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, _ := r.BasicAuth(); u != "user" || p != "pass" || r.Header.Get("Module") != base64.StdEncoding.EncodeToString([]byte("MOD")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		paths = append(paths, r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/api/Web/GetToken":
			if body["Amount"] != 100.0 || body["RefId"] != "ord-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"ResponseCode":0,"ResponseDescription":"Success","TokenId":202103111046564183,"Amount":100.0,"RefId":"ord-1"}`))
		default:
			_, _ = w.Write([]byte(confirmBody))
		}
	}))
	t.Cleanup(srv.Close)
	c, err := imepay.New(imepay.Config{MerchantCode: "M", Module: "MOD", APIUser: "user", APIPassword: "pass", Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c, &paths
}

func TestInitiateAndCallback(t *testing.T) {
	c, _ := setup(t, "")
	co, err := c.Initiate(context.Background(), paynp.InitiateRequest{TxnID: "ord-1", Amount: 10000, SuccessURL: "https://m/ok", FailureURL: "https://m/no"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := base64.StdEncoding.DecodeString(co.Fields["data"])
	if co.ProviderRef != "202103111046564183" || string(data) != "202103111046564183|M|ord-1|100.00|GET|https://m/ok|https://m/no" {
		t.Fatalf("unexpected checkout %+v %s", co, data)
	}

	ret := base64.StdEncoding.EncodeToString([]byte("0|Success|9800000000|TXN9|ord-1|100.0000|202103111046564183"))
	cb, err := c.ParseCallback(url.Values{"data": {ret}})
	if err != nil || cb.Status != paynp.StatusPending || cb.Amount != 10000 || cb.Values.Get("TransactionId") != "TXN9" {
		t.Fatalf("unexpected %+v %v", cb, err)
	}
}

func TestLookupConfirmVsRecheck(t *testing.T) {
	c, paths := setup(t, `{"ResponseCode":"0","ResponseDescription":"Success","TransactionId":"TXN9","RefId":"ord-1","TokenId":"T","TranAmount":"100.0000","Amount":100}`)
	cb := &paynp.Callback{ProviderRef: "T", Values: url.Values{"TransactionId": {"TXN9"}, "Msisdn": {"98"}}}

	tx, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 10000, Callback: cb})
	if err != nil || tx.Status != paynp.StatusSuccess || tx.ProviderRef != "TXN9" {
		t.Fatalf("unexpected %+v %v", tx, err)
	}
	if _, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 10000, ProviderRef: "T"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(*paths, ",") != "/api/Web/Confirm,/api/Web/Recheck" {
		t.Fatalf("paths %v", *paths)
	}
}

func TestLookupRejectsReplyForOtherOrder(t *testing.T) {
	c, _ := setup(t, `{"ResponseCode":0,"TokenId":"OTHER","RefId":"ord-1"}`)
	var apiErr *paynp.APIError
	if _, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 10000, ProviderRef: "T"}); !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
}

func TestLookupCanceledAndBadToken(t *testing.T) {
	c, _ := setup(t, `{"ResponseCode":3,"ResponseDescription":"Operation Cancelled By User"}`)
	tx, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 10000, ProviderRef: "T"})
	if err != nil || tx.Status != paynp.StatusCanceled {
		t.Fatalf("unexpected %+v %v", tx, err)
	}
	if _, err := c.Lookup(context.Background(), paynp.LookupRequest{TxnID: "ord-1", Amount: 10000}); !errors.Is(err, paynp.ErrInvalidRequest) {
		t.Fatalf("want ErrInvalidRequest, got %v", err)
	}
}
