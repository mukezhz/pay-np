package khalti

import (
	"net/http"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

type Config struct {
	SecretKey   string
	WebsiteURL  string
	Environment paynp.Environment
	BaseURL     string
	HTTPClient  *http.Client
}

type customerInfo struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type initiateRequest struct {
	ReturnURL         string        `json:"return_url"`
	WebsiteURL        string        `json:"website_url"`
	Amount            int64         `json:"amount"`
	PurchaseOrderID   string        `json:"purchase_order_id"`
	PurchaseOrderName string        `json:"purchase_order_name"`
	CustomerInfo      *customerInfo `json:"customer_info,omitempty"`
}

type initiateResponse struct {
	Pidx       string    `json:"pidx"`
	PaymentURL string    `json:"payment_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type lookupResponse struct {
	Pidx          string  `json:"pidx"`
	TotalAmount   int64   `json:"total_amount"`
	Status        string  `json:"status"`
	TransactionID *string `json:"transaction_id"`
}
