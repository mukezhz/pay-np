package paynp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	paynp "github.com/mukezhz/pay-np"
)

func TestParseRupees(t *testing.T) {
	ok := map[string]paynp.Paisa{"100": 10000, "133.0": 13300, "1,000.50": 100050, " 0.5 ": 50, "12.34": 1234, "10.0000": 1000}
	for in, want := range ok {
		if got, err := paynp.ParseRupees(in); err != nil || got != want {
			t.Errorf("ParseRupees(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", ".5", "1.234", "1.0001", "abc", "1.a", "-", "+", "-.5"} {
		if _, err := paynp.ParseRupees(in); err == nil {
			t.Errorf("ParseRupees(%q) should fail", in)
		}
	}
	if s := paynp.Paisa(100050).Rupees(); s != "1000.50" {
		t.Errorf("Rupees = %s", s)
	}
}

func TestCheckoutWrite(t *testing.T) {
	post := &paynp.Checkout{Method: http.MethodPost, URL: "https://p/form", Fields: map[string]string{"a": `"><script>`}}
	rec := httptest.NewRecorder()
	if err := post.Write(rec, httptest.NewRequest(http.MethodGet, "/", nil)); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="https://p/form"`) || strings.Contains(body, "<script>") {
		t.Fatalf("form not rendered/escaped: %s", body)
	}

	get := &paynp.Checkout{Method: http.MethodGet, URL: "https://p/pay?x=1", Fields: map[string]string{"y": "2"}}
	rec = httptest.NewRecorder()
	if err := get.Write(rec, httptest.NewRequest(http.MethodGet, "/", nil)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://p/pay?x=1&y=2" {
		t.Fatalf("redirect %d %s", rec.Code, rec.Header().Get("Location"))
	}
}
