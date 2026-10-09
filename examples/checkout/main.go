// Command checkout is a local test shop for pay-np providers.
package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	paynp "github.com/mukezhz/pay-np"
	"github.com/mukezhz/pay-np/connectips"
	"github.com/mukezhz/pay-np/esewa"
	"github.com/mukezhz/pay-np/fonepay"
	"github.com/mukezhz/pay-np/hamropay"
	"github.com/mukezhz/pay-np/imepay"
	"github.com/mukezhz/pay-np/khalti"
)

//go:embed web
var webFS embed.FS

var pages = func() map[string]*template.Template {
	m := map[string]*template.Template{}
	for _, name := range []string{"landing", "checkout", "attempt", "provider"} {
		m[name] = template.Must(template.New("").Funcs(funcs).ParseFS(webFS, "web/templates/layout.html", "web/templates/"+name+".html"))
	}
	return m
}()

type attempt struct {
	ID        string
	Provider  paynp.ProviderName
	Amount    paynp.Paisa
	Desc      string
	Ref       string
	Status    paynp.Status
	CreatedAt time.Time
	Callback  *paynp.Callback
	Events    []event
	AppReturn string
	checkout  *paynp.Checkout
}

type event struct {
	At     time.Time
	Kind   string
	Status paynp.Status
	Detail string
}

type shop struct {
	baseURL    string
	env        string
	providers  map[paynp.ProviderName]paynp.Provider
	esewaApp   *esewa.MobileClient
	appSchemes []string

	mu       sync.Mutex
	attempts map[string]*attempt
}

func main() {
	addr := envOr("ADDR", ":8080")
	s := &shop{
		baseURL:    strings.TrimRight(envOr("BASE_URL", "http://localhost"+addr), "/"),
		env:        envOr("PAYNP_ENV", "sandbox"),
		providers:  map[paynp.ProviderName]paynp.Provider{},
		attempts:   map[string]*attempt{},
		appSchemes: strings.Split(envOr("APP_RETURN_SCHEMES", "paynp"), ","),
	}
	environment := paynp.Sandbox
	if s.env == "production" {
		environment = paynp.Production
	}
	if err := s.configure(environment); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	static, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /static/", http.FileServerFS(static))
	mux.HandleFunc("GET /{$}", s.landing)
	mux.HandleFunc("GET /checkout", s.index)
	mux.HandleFunc("POST /pay", s.pay)
	mux.HandleFunc("/return/{provider}/{txn}", s.handleReturn)
	mux.HandleFunc("/return/{provider}", s.handleReturn) // ConnectIPS: URL registered with NCHL, TXNID in query
	mux.HandleFunc("POST /webhook/hamropay", s.hamropayWebhook)
	mux.HandleFunc("GET /providers/{name}", s.providerDocs)
	mux.HandleFunc("POST /api/esewa/mobile-verify", s.esewaMobileVerify)
	mux.HandleFunc("POST /api/payments", s.apiCreatePayment)
	mux.HandleFunc("GET /api/payments/{txn}", s.apiGetPayment)
	mux.HandleFunc("GET /pay/{txn}", s.openCheckout)
	mux.HandleFunc("GET /attempts/{txn}", s.show)
	mux.HandleFunc("POST /attempts/{txn}/lookup", s.recheck)

	names := make([]string, 0, len(s.providers))
	for n := range s.providers {
		names = append(names, string(n))
	}
	slices.Sort(names)
	log.Printf("pay-np test shop on %s (%s) — providers: %s", s.baseURL, s.env, strings.Join(names, ", "))
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *shop) configure(env paynp.Environment) error {
	add := func(p paynp.Provider, err error) error {
		if err != nil {
			return err
		}
		s.providers[p.Name()] = p
		return nil
	}
	esewaCode, esewaSecret := os.Getenv("ESEWA_PRODUCT_CODE"), os.Getenv("ESEWA_SECRET_KEY")
	if esewaCode == "" && env == paynp.Sandbox {
		esewaCode, esewaSecret = esewa.SandboxProductCode, esewa.SandboxSecretKey
	}
	if esewaCode != "" {
		if err := add(esewa.New(esewa.Config{ProductCode: esewaCode, SecretKey: esewaSecret, Environment: env,
			StatusURL: os.Getenv("ESEWA_STATUS_URL")})); err != nil {
			return err
		}
	}
	appID, appSecret := os.Getenv("ESEWA_MOBILE_CLIENT_ID"), os.Getenv("ESEWA_MOBILE_CLIENT_SECRET")
	if appID == "" && env == paynp.Sandbox {
		appID, appSecret = esewa.SandboxMobileClientID, esewa.SandboxMobileClientSecret
	}
	if appID != "" {
		m, err := esewa.NewMobile(esewa.MobileConfig{ClientID: appID, ClientSecret: appSecret, Environment: env})
		if err != nil {
			return err
		}
		s.esewaApp = m
	}
	key := os.Getenv("KHALTI_SECRET_KEY")
	if key == "" && env == paynp.Sandbox {
		key = khalti.SandboxSecretKey
	}
	if key != "" {
		if err := add(khalti.New(khalti.Config{SecretKey: key, WebsiteURL: envOr("KHALTI_WEBSITE_URL", s.baseURL), Environment: env})); err != nil {
			return err
		}
	}
	if mid := os.Getenv("CONNECTIPS_MERCHANT_ID"); mid != "" {
		pfx, err := os.ReadFile(os.Getenv("CONNECTIPS_PFX_PATH"))
		if err != nil {
			return fmt.Errorf("CONNECTIPS_PFX_PATH: %w", err)
		}
		key, err := connectips.ParsePFX(pfx, os.Getenv("CONNECTIPS_PFX_PASSWORD"))
		if err != nil {
			return err
		}
		if err := add(connectips.New(connectips.Config{
			MerchantID: mid, AppID: os.Getenv("CONNECTIPS_APP_ID"), AppName: os.Getenv("CONNECTIPS_APP_NAME"),
			Username: os.Getenv("CONNECTIPS_USERNAME"), Password: os.Getenv("CONNECTIPS_PASSWORD"),
			PrivateKey: key, Environment: env, Host: os.Getenv("CONNECTIPS_HOST"),
		})); err != nil {
			return err
		}
	}
	if code := os.Getenv("FONEPAY_MERCHANT_CODE"); code != "" {
		if err := add(fonepay.New(fonepay.Config{MerchantCode: code, SecretKey: os.Getenv("FONEPAY_SECRET_KEY"), Environment: env})); err != nil {
			return err
		}
	}
	if mid := os.Getenv("HAMROPAY_MERCHANT_ID"); mid != "" {
		if err := add(hamropay.New(hamropay.Config{MerchantID: mid, ClientID: os.Getenv("HAMROPAY_CLIENT_ID"),
			ClientAPIKey: os.Getenv("HAMROPAY_CLIENT_API_KEY"), ClientSecret: os.Getenv("HAMROPAY_CLIENT_SECRET"),
			WebhookSecret: os.Getenv("HAMROPAY_WEBHOOK_SECRET"), Environment: env,
			APIBaseURL: os.Getenv("HAMROPAY_API_BASE_URL"), GatewayURL: os.Getenv("HAMROPAY_GATEWAY_URL")})); err != nil {
			return err
		}
	}
	if code := os.Getenv("IMEPAY_MERCHANT_CODE"); code != "" {
		if err := add(imepay.New(imepay.Config{MerchantCode: code, Module: os.Getenv("IMEPAY_MODULE"),
			APIUser: os.Getenv("IMEPAY_API_USER"), APIPassword: os.Getenv("IMEPAY_API_PASSWORD"), Environment: env})); err != nil {
			return err
		}
	}
	return nil
}

func (s *shop) landing(w http.ResponseWriter, _ *http.Request) {
	opts := s.catalog()
	s.mu.Lock()
	n := len(s.attempts)
	s.mu.Unlock()
	render(w, "landing.html", map[string]any{"Title": "Nepali payments in Go", "Page": "home", "Env": s.env, "Providers": opts, "Enabled": len(s.providers), "Total": len(catalog), "Attempts": n})
}

func (s *shop) catalog() []providerInfo {
	opts := slices.Clone(catalog)
	for i := range opts {
		_, opts[i].Enabled = s.providers[opts[i].Name]
	}
	return opts
}

func (s *shop) index(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	list := make([]*attempt, 0, len(s.attempts))
	for _, a := range s.attempts {
		list = append(list, a)
	}
	s.mu.Unlock()
	slices.SortFunc(list, func(a, b *attempt) int { return b.CreatedAt.Compare(a.CreatedAt) })

	opts := s.catalog()
	slices.SortStableFunc(opts, func(a, b providerInfo) int {
		if a.Enabled == b.Enabled {
			return 0
		}
		if a.Enabled {
			return -1
		}
		return 1
	})
	selected := paynp.ProviderName(r.URL.Query().Get("provider"))
	if _, ok := s.providers[selected]; !ok && len(opts) > 0 && opts[0].Enabled {
		selected = opts[0].Name
	}
	render(w, "checkout.html", map[string]any{"Selected": selected, "Title": "Checkout", "Page": "checkout", "Env": s.env, "Providers": opts, "Enabled": len(s.providers), "Total": len(catalog), "Attempts": list})
}

var errUnknownProvider = errors.New("provider not configured")

type startRequest struct {
	Provider  paynp.ProviderName
	Amount    paynp.Paisa
	Desc      string
	Customer  paynp.Customer
	AppReturn string
}

func (s *shop) start(ctx context.Context, req startRequest) (*attempt, error) {
	p, ok := s.providers[req.Provider]
	if !ok {
		return nil, errUnknownProvider
	}
	a := &attempt{ID: newTxnID(), Provider: p.Name(), Amount: req.Amount, Desc: req.Desc, CreatedAt: time.Now(), AppReturn: req.AppReturn}
	ret := fmt.Sprintf("%s/return/%s/%s", s.baseURL, p.Name(), a.ID)
	co, err := p.Initiate(ctx, paynp.InitiateRequest{
		TxnID: a.ID, Amount: req.Amount, Description: req.Desc,
		SuccessURL: ret, FailureURL: ret + "?failed=1", Customer: req.Customer,
	})
	if err != nil {
		a.Status = paynp.StatusFailed
		a.log("initiate", "", err.Error())
		s.save(a)
		return a, err
	}
	a.Ref, a.Status, a.checkout = co.ProviderRef, paynp.StatusPending, co
	a.log("initiate", paynp.StatusPending, fmt.Sprintf("%s %s ref=%q", co.Method, co.URL, co.ProviderRef))
	s.save(a)
	return a, nil
}

func (s *shop) pay(w http.ResponseWriter, r *http.Request) {
	amount, err := paynp.ParseRupees(r.FormValue("amount"))
	if err != nil || amount <= 0 {
		http.Error(w, "invalid amount", http.StatusBadRequest)
		return
	}
	a, err := s.start(r.Context(), startRequest{
		Provider: paynp.ProviderName(r.FormValue("provider")), Amount: amount, Desc: strings.TrimSpace(r.FormValue("description")),
		Customer: paynp.Customer{Name: r.FormValue("name"), Email: r.FormValue("email"), Phone: r.FormValue("phone")},
	})
	switch {
	case a == nil:
		http.Error(w, err.Error(), http.StatusBadRequest)
	case err != nil:
		http.Redirect(w, r, "/attempts/"+a.ID, http.StatusSeeOther)
	default:
		if err := a.checkout.Write(w, r); err != nil {
			log.Print(err)
		}
	}
}

func (s *shop) handleReturn(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providers[paynp.ProviderName(r.PathValue("provider"))]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm() // some providers POST back
	cb, cbErr := p.ParseCallback(r.Form)
	id := r.PathValue("txn")
	if id == "" && cb != nil {
		id = cb.TxnID
	}
	a := s.get(id)
	if a == nil {
		http.Error(w, "unknown transaction "+strconv.Quote(id), http.StatusNotFound)
		return
	}
	s.mu.Lock()
	switch {
	case cbErr != nil && r.Form.Get("failed") == "1":
		a.log("callback", "", "returned to failure URL (provider sent no signed data)")
	case cbErr != nil:
		a.log("callback", "", cbErr.Error())
	default:
		a.Callback = cb
		a.log("callback", cb.Status, "hint only: "+r.Form.Encode())
	}
	s.mu.Unlock()
	s.lookup(r.Context(), a)
	if a.AppReturn != "" {
		s.mu.Lock()
		target := appReturnURL(a.AppReturn, a.ID, a.Status)
		s.mu.Unlock()
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/attempts/"+a.ID, http.StatusSeeOther)
}

func (s *shop) hamropayWebhook(w http.ResponseWriter, r *http.Request) {
	p, ok := s.providers[paynp.HamroPay].(*hamropay.Client)
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	cb, err := p.ParseWebhook(r.Header, body)
	if err != nil {
		log.Print(err)
		http.Error(w, "invalid webhook", http.StatusUnauthorized)
		return
	}
	a := s.get(cb.TxnID)
	if a == nil {
		w.WriteHeader(http.StatusOK) // not ours; acknowledge so it is not retried
		return
	}
	s.mu.Lock()
	a.log("webhook", cb.Status, "signature verified: "+string(body))
	s.mu.Unlock()
	s.lookup(r.Context(), a)
	w.WriteHeader(http.StatusOK)
}

func (s *shop) esewaMobileVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	reply := func(code int, v map[string]any) {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	if s.esewaApp == nil {
		reply(http.StatusNotFound, map[string]any{"error": "eSewa mobile credentials not configured"})
		return
	}
	amount, err := paynp.ParseRupees(r.FormValue("amount"))
	if err != nil || amount <= 0 {
		reply(http.StatusBadRequest, map[string]any{"error": "invalid amount"})
		return
	}
	tx, err := s.esewaApp.Verify(r.Context(), esewa.MobileVerifyRequest{
		ProductID: strings.TrimSpace(r.FormValue("product_id")), RefID: strings.TrimSpace(r.FormValue("ref_id")), Amount: amount,
	})
	if err != nil {
		reply(http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	reply(http.StatusOK, map[string]any{"status": tx.Status, "final": tx.Status.Final(), "ref_id": tx.ProviderRef, "amount": tx.Amount.Rupees(), "raw": json.RawMessage(tx.Raw)})
}

func (s *shop) recheck(w http.ResponseWriter, r *http.Request) {
	if a := s.get(r.PathValue("txn")); a != nil {
		s.lookup(r.Context(), a)
	}
	http.Redirect(w, r, "/attempts/"+r.PathValue("txn"), http.StatusSeeOther)
}

func (s *shop) lookup(ctx context.Context, a *attempt) {
	s.mu.Lock()
	req := paynp.LookupRequest{TxnID: a.ID, Amount: a.Amount, ProviderRef: a.Ref, Callback: a.Callback}
	s.mu.Unlock()
	tx, err := s.providers[a.Provider].Lookup(ctx, req)
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case errors.Is(err, paynp.ErrAmountMismatch):
		a.Status = paynp.StatusFailed
		a.log("lookup", a.Status, err.Error())
	case err != nil:
		a.log("lookup", "", err.Error())
	default:
		a.Status = tx.Status
		if tx.ProviderRef != "" && a.Ref == "" {
			a.Ref = tx.ProviderRef
		}
		a.log("lookup", tx.Status, prettyJSON(tx.Raw))
	}
}

func (s *shop) show(w http.ResponseWriter, r *http.Request) {
	a := s.get(r.PathValue("txn"))
	if a == nil {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	render(w, "attempt.html", map[string]any{"Title": a.ID, "Page": "checkout", "A": a, "Env": s.env})
}

func (s *shop) save(a *attempt) {
	s.mu.Lock()
	s.attempts[a.ID] = a
	s.mu.Unlock()
}

func (s *shop) get(id string) *attempt {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts[id]
}

func (a *attempt) log(kind string, st paynp.Status, detail string) {
	a.Events = append(a.Events, event{At: time.Now(), Kind: kind, Status: st, Detail: detail})
}

func render(w http.ResponseWriter, name string, data map[string]any) {
	page := strings.TrimSuffix(name, ".html")
	data["Style"] = page
	if _, err := fs.Stat(webFS, "web/static/js/"+page+".js"); err == nil {
		data["Script"] = page
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages[page].ExecuteTemplate(w, "layout", data); err != nil {
		log.Print(err)
	}
}

// newTxnID fits every provider: ≤ 20 chars, [a-z0-9-].
func newTxnID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "np-" + hex.EncodeToString(b)
}

func prettyJSON(raw []byte) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
