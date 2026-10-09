package esewa

import (
	"encoding/json"
	"net/http"

	paynp "github.com/mukezhz/pay-np"
)

type Config struct {
	ProductCode string
	SecretKey   string
	Environment paynp.Environment
	FormURL     string
	StatusURL   string
	HTTPClient  *http.Client
}

// eSewa returns either snake_case (v2) or camelCase fields.
type statusResponse struct {
	TotalAmount    json.Number `json:"total_amount"`
	TotalAmountOld json.Number `json:"totalAmount"`
	Status         string      `json:"status"`
	RefID          *string     `json:"ref_id"`
	RefIDOld       *string     `json:"refId"`
}

type MobileConfig struct {
	ClientID     string
	ClientSecret string
	Environment  paynp.Environment
	VerifyURL    string
	HTTPClient   *http.Client
}

type MobileVerifyRequest struct {
	// ProductID must be unique per order; it binds the payment to the order.
	ProductID string
	RefID     string
	Amount    paynp.Paisa
}

type mobileTxn struct {
	ProductID          string      `json:"productId"`
	TotalAmount        json.Number `json:"totalAmount"`
	TransactionDetails struct {
		ReferenceID string `json:"referenceId"`
		Status      string `json:"status"`
	} `json:"transactionDetails"`
}
