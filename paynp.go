// Package paynp is a provider-neutral SDK for Nepali payment gateways.
package paynp

import (
	"context"
	"net/url"
	"time"
)

// Provider is implemented by every gateway package. Only Lookup is authoritative.
type Provider interface {
	Name() ProviderName
	Initiate(ctx context.Context, req InitiateRequest) (*Checkout, error)
	// ParseCallback's status is a hint; unsigned callbacks never report SUCCESS.
	ParseCallback(query url.Values) (*Callback, error)
	Lookup(ctx context.Context, req LookupRequest) (*Transaction, error)
}

type ProviderName string

const (
	Esewa      ProviderName = "esewa"
	Khalti     ProviderName = "khalti"
	ConnectIPS ProviderName = "connectips"
	Fonepay    ProviderName = "fonepay"
	IMEPay     ProviderName = "imepay"
	HamroPay   ProviderName = "hamropay"
)

type Environment int

const (
	Sandbox Environment = iota
	Production
)

type Customer struct {
	Name  string
	Email string
	Phone string
}

type InitiateRequest struct {
	// TxnID is unique per attempt; ≤ 20 chars of [A-Za-z0-9-] suits every provider.
	TxnID       string
	Amount      Paisa
	Description string
	SuccessURL  string
	FailureURL  string
	Customer    Customer
}

func (r InitiateRequest) Validate() error {
	switch {
	case r.TxnID == "":
		return invalid("TxnID is required")
	case r.Amount <= 0:
		return invalid("Amount must be positive")
	}
	return nil
}

type Checkout struct {
	Provider ProviderName
	Method   string
	URL      string
	Fields   map[string]string
	// ProviderRef (Khalti pidx, IME Pay token) must be persisted for Lookup.
	ProviderRef string
	ExpiresAt   time.Time
}

type Callback struct {
	TxnID       string
	ProviderRef string
	Status      Status
	Amount      Paisa
	Values      url.Values
}

type LookupRequest struct {
	TxnID string
	// Amount comes from your records, never the callback.
	Amount      Paisa
	ProviderRef string
	// Callback is needed only by Fonepay and IME Pay Confirm.
	Callback *Callback
}

type Transaction struct {
	Provider    ProviderName
	TxnID       string
	ProviderRef string
	Status      Status
	Amount      Paisa
	Raw         []byte
}

type Status string

const (
	StatusPending           Status = "PENDING"
	StatusSuccess           Status = "SUCCESS"
	StatusFailed            Status = "FAILED"
	StatusCanceled          Status = "CANCELED"
	StatusExpired           Status = "EXPIRED"
	StatusNotFound          Status = "NOT_FOUND"
	StatusRefunded          Status = "REFUNDED"
	StatusPartiallyRefunded Status = "PARTIALLY_REFUNDED"
)

// Final is false for NOT_FOUND: the user may still be paying.
func (s Status) Final() bool {
	switch s {
	case StatusSuccess, StatusFailed, StatusCanceled, StatusExpired, StatusRefunded, StatusPartiallyRefunded:
		return true
	}
	return false
}

func MatchAmount(tx *Transaction, want Paisa) (*Transaction, error) {
	if tx.Status == StatusSuccess && tx.Amount != want {
		return nil, amountMismatch(want, tx.Amount)
	}
	return tx, nil
}
