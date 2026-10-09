package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"

	paynp "github.com/mukezhz/pay-np"
)

type createPayment struct {
	Provider     paynp.ProviderName `json:"provider"`
	Amount       string             `json:"amount"`
	Description  string             `json:"description"`
	Customer     paynp.Customer     `json:"customer"`
	AppReturnURL string             `json:"app_return_url"`
}

type paymentView struct {
	TxnID       string             `json:"txn_id,omitempty"`
	Provider    paynp.ProviderName `json:"provider,omitempty"`
	Amount      string             `json:"amount,omitempty"`
	Status      paynp.Status       `json:"status,omitempty"`
	Final       bool               `json:"final"`
	ProviderRef string             `json:"provider_ref,omitempty"`
	CheckoutURL string             `json:"checkout_url,omitempty"`
	ExpiresAt   string             `json:"expires_at,omitempty"`
	Error       string             `json:"error,omitempty"`
}

func (s *shop) apiCreatePayment(w http.ResponseWriter, r *http.Request) {
	var req createPayment
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, paymentView{Error: "invalid JSON body"})
		return
	}
	amount, err := paynp.ParseRupees(req.Amount)
	if err != nil || amount <= 0 {
		writeJSON(w, http.StatusBadRequest, paymentView{Error: "amount must be positive rupees, e.g. \"100.00\""})
		return
	}
	if req.AppReturnURL != "" && !s.allowedAppReturn(req.AppReturnURL) {
		writeJSON(w, http.StatusBadRequest, paymentView{Error: "app_return_url scheme not in APP_RETURN_SCHEMES"})
		return
	}
	a, err := s.start(r.Context(), startRequest{Provider: req.Provider, Amount: amount, Desc: strings.TrimSpace(req.Description), Customer: req.Customer, AppReturn: req.AppReturnURL})
	switch {
	case errors.Is(err, errUnknownProvider):
		writeJSON(w, http.StatusBadRequest, paymentView{Error: err.Error()})
	case err != nil:
		v := s.view(a)
		v.Error = err.Error()
		writeJSON(w, http.StatusBadGateway, v)
	default:
		writeJSON(w, http.StatusCreated, s.view(a))
	}
}

func (s *shop) apiGetPayment(w http.ResponseWriter, r *http.Request) {
	a := s.get(r.PathValue("txn"))
	if a == nil {
		writeJSON(w, http.StatusNotFound, paymentView{Error: "unknown transaction"})
		return
	}
	s.mu.Lock()
	recheck := a.checkout != nil && !a.Status.Final()
	s.mu.Unlock()
	if recheck {
		s.lookup(r.Context(), a)
	}
	writeJSON(w, http.StatusOK, s.view(a))
}

// openCheckout lets an in-app browser load a checkout that needs a form POST.
func (s *shop) openCheckout(w http.ResponseWriter, r *http.Request) {
	a := s.get(r.PathValue("txn"))
	if a == nil || a.checkout == nil {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	final := a.Status.Final()
	s.mu.Unlock()
	if final {
		http.Redirect(w, r, "/attempts/"+a.ID, http.StatusSeeOther)
		return
	}
	if err := a.checkout.Write(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *shop) view(a *attempt) paymentView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := paymentView{TxnID: a.ID, Provider: a.Provider, Amount: a.Amount.Rupees(), Status: a.Status, Final: a.Status.Final(), ProviderRef: a.Ref}
	if a.checkout != nil && !a.Status.Final() {
		v.CheckoutURL = s.baseURL + "/pay/" + a.ID
		if !a.checkout.ExpiresAt.IsZero() {
			v.ExpiresAt = a.checkout.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		}
	}
	return v
}

func (s *shop) allowedAppReturn(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && slices.Contains(s.appSchemes, u.Scheme)
}

func appReturnURL(raw, txn string, st paynp.Status) string {
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("txn_id", txn)
	q.Set("status", string(st))
	u.RawQuery = q.Encode()
	return u.String()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
