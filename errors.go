package paynp

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidRequest   = errors.New("paynp: invalid request")
	ErrInvalidConfig    = errors.New("paynp: invalid config")
	ErrInvalidCallback  = errors.New("paynp: invalid callback")
	ErrInvalidSignature = errors.New("paynp: invalid signature")
	ErrAmountMismatch   = errors.New("paynp: amount mismatch")
	ErrCallbackRequired = errors.New("paynp: lookup needs the provider callback")
)

type APIError struct {
	Provider   ProviderName
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("paynp: %s api returned %d: %s", e.Provider, e.StatusCode, truncate(e.Body, 300))
}

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalidRequest, msg) }

func amountMismatch(want, got Paisa) error {
	return fmt.Errorf("%w: expected %s, provider reported %s", ErrAmountMismatch, want.Rupees(), got.Rupees())
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
