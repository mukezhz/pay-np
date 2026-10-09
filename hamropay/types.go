package hamropay

import (
	"encoding/json"
	"net/http"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

type Config struct {
	MerchantID    string
	ClientID      string
	ClientAPIKey  string
	ClientSecret  string
	WebhookSecret string
	Environment   paynp.Environment
	APIBaseURL    string
	GatewayURL    string
	HTTPClient    *http.Client
	Now           func() time.Time
}

type product struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

type sessionRequest struct {
	MerchantTxnID     string    `json:"merchantTxnId"`
	MerchantID        string    `json:"merchantId"`
	TransactionAmount string    `json:"transactionAmount"`
	FailedURL         string    `json:"failedRedirectionUrl"`
	SuccessURL        string    `json:"successRedirectionUrl"`
	ProductList       []product `json:"productList"`
	Remarks           string    `json:"remarks,omitempty"`
	PhoneNumber       string    `json:"phone_number,omitempty"`
}

type transaction struct {
	MerchantTransactionID string      `json:"merchantTransactionId"`
	MerchantTxnID         string      `json:"merchantTxnId"` // UAT has been seen using this name
	Status                string      `json:"status"`
	Amount                json.Number `json:"amount"`
	Message               string      `json:"message"`
}

type webhook struct {
	MerchantTxnID string            `json:"merchantTxnId"`
	MerchantID    string            `json:"merchantId"`
	Amount        json.Number       `json:"amount"`
	Status        string            `json:"status"`
	Metadata      map[string]string `json:"metadata"`
}
