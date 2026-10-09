package main

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

type providerInfo struct {
	Name     paynp.ProviderName
	Label    string
	Mono     string
	Color    string
	Checkout string
	Signed   bool
	Lookup   string
	Sandbox  string
	Env      string
	Enabled  bool
}

var catalog = []providerInfo{
	{Name: paynp.Esewa, Label: "eSewa", Mono: "e", Color: "#41a124", Checkout: "Form POST", Signed: true,
		Lookup: "Status API by TxnID", Sandbox: "ID 9806800001 · Nepal@123 · token 123456", Env: "ESEWA_PRODUCT_CODE, ESEWA_SECRET_KEY"},
	{Name: paynp.Khalti, Label: "Khalti", Mono: "K", Color: "#5c2d91", Checkout: "Redirect",
		Lookup: "Lookup by pidx", Sandbox: "ID 9800000000 · MPIN 1111 · OTP 987654 · min Rs 10", Env: "KHALTI_SECRET_KEY"},
	{Name: paynp.ConnectIPS, Label: "ConnectIPS", Mono: "C", Color: "#1f5aa6", Checkout: "Form POST",
		Lookup: "gettxndetail by TxnID", Sandbox: "UAT bank login from NCHL", Env: "CONNECTIPS_MERCHANT_ID, _APP_ID, _APP_NAME, _PASSWORD, _PFX_PATH"},
	{Name: paynp.Fonepay, Label: "Fonepay", Mono: "F", Color: "#d32f2f", Checkout: "Redirect", Signed: true,
		Lookup: "Needs callback UID", Sandbox: "Dev merchant from Fonepay", Env: "FONEPAY_MERCHANT_CODE, FONEPAY_SECRET_KEY"},
	{Name: paynp.HamroPay, Label: "Hamro Pay", Mono: "H", Color: "#c8102e", Checkout: "Form POST (session)",
		Lookup: "Get Transaction + signed webhook", Sandbox: "Wallet 9841414141 · T-PIN 0000 · OTP 000000 = success, 111111 = pending, 222222 = failed",
		Env: "HAMROPAY_MERCHANT_ID, _CLIENT_ID, _CLIENT_API_KEY, _CLIENT_SECRET (free UAT signup at pay-sandbox.hamropatro.com)"},
	{Name: paynp.IMEPay, Label: "IME Pay", Mono: "I", Color: "#e65100", Checkout: "Redirect (token)",
		Lookup: "Confirm / Recheck by token", Sandbox: "Staging merchant from IME", Env: "IMEPAY_MERCHANT_CODE, _MODULE, _API_USER, _API_PASSWORD"},
}

func infoFor(n paynp.ProviderName) providerInfo {
	for _, p := range catalog {
		if p.Name == n {
			return p
		}
	}
	return providerInfo{Name: n, Label: string(n), Mono: "?", Color: "#667085"}
}

var statusText = map[paynp.Status]string{
	paynp.StatusPending:           "Waiting on the provider. The user may still be paying; check again or let a reconciler poll.",
	paynp.StatusSuccess:           "The provider confirmed the payment server-to-server and the amount matches. Safe to fulfil.",
	paynp.StatusFailed:            "The payment failed or could not be started. Nothing was charged.",
	paynp.StatusCanceled:          "The user canceled on the provider's page.",
	paynp.StatusExpired:           "The provider session expired before payment.",
	paynp.StatusNotFound:          "The provider has no record of this transaction yet — not paid, or the user never got past login.",
	paynp.StatusRefunded:          "The provider reports the payment as refunded.",
	paynp.StatusPartiallyRefunded: "The provider reports a partial refund.",
}

type step struct {
	Title string
	State string // done | error | waiting | idle
	Note  string
}

func steps(a *attempt) []step {
	s := []step{
		{"Initiate", "idle", "Sign the request / create the provider session"},
		{"Callback", "idle", "Provider redirects the user back (a hint only)"},
		{"Lookup", "idle", "Ask the provider server-to-server (authoritative)"},
	}
	idx := map[string]int{"initiate": 0, "callback": 1, "webhook": 1, "lookup": 2}
	for _, e := range a.Events {
		i := idx[e.Kind]
		s[i].State = "done"
		if e.Status == "" {
			s[i].State = "error"
		}
	}
	if s[0].State == "done" && s[1].State == "idle" {
		s[1].State = "waiting"
	}
	return s
}

var funcs = template.FuncMap{
	"rupees":  func(p paynp.Paisa) string { return p.Rupees() },
	"final":   func(s paynp.Status) bool { return s.Final() },
	"info":    infoFor,
	"explain": func(s paynp.Status) string { return statusText[s] },
	"steps":   steps,
	"css":     func(s string) template.CSS { return template.CSS(s) },
	"inc":     func(i int) int { return i + 1 },
	"title": func(s paynp.Status) string {
		t := strings.ReplaceAll(strings.ToLower(string(s)), "_", " ")
		if t == "" {
			return ""
		}
		return strings.ToUpper(t[:1]) + t[1:]
	},
	"ago": func(t time.Time) string {
		switch d := time.Since(t); {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		default:
			return t.Format("Jan 2 15:04")
		}
	},
}
