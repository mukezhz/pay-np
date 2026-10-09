// Package paynp is a provider-neutral SDK for Nepali payment gateways.
package paynp

import (
	"context"
	"net/url"
)

// Provider is implemented by every gateway package. Only Lookup is authoritative.
type Provider interface {
	Name() ProviderName
	Initiate(ctx context.Context, req InitiateRequest) (*Checkout, error)
	// ParseCallback's status is a hint; unsigned callbacks never report SUCCESS.
	ParseCallback(query url.Values) (*Callback, error)
	Lookup(ctx context.Context, req LookupRequest) (*Transaction, error)
}

func MatchAmount(tx *Transaction, want Paisa) (*Transaction, error) {
	if tx.Status == StatusSuccess && tx.Amount != want {
		return nil, amountMismatch(want, tx.Amount)
	}
	return tx, nil
}
