package fonepay

import (
	"net/http"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

type Config struct {
	MerchantCode string // PID
	SecretKey    string
	// Username and Password are the merchant-portal API credentials; only the QR API needs them.
	Username    string
	Password    string
	Environment paynp.Environment
	Host        string
	// QRHost overrides the merchant API host used by the QR API.
	QRHost     string
	HTTPClient *http.Client
	Now        func() time.Time
}

type verifyResponse struct {
	Success      bool   `xml:"success"`
	ResponseCode string `xml:"response_code"`
	Message      string `xml:"message"`
	TxnAmount    string `xml:"txnAmount"`
}

// QR is a dynamic Fonepay QR; render Message with any QR encoder.
type QR struct {
	TxnID   string
	Message string
	// WebSocketURL pushes the payment result; QRStatus is the authoritative check.
	WebSocketURL string
}

type qrRequest struct {
	Amount         string `json:"amount"`
	Remarks1       string `json:"remarks1"`
	Remarks2       string `json:"remarks2"`
	PRN            string `json:"prn"`
	MerchantCode   string `json:"merchantCode"`
	DataValidation string `json:"dataValidation"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

type qrResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	QRMessage    string `json:"qrMessage"`
	WebSocketURL string `json:"thirdpartyQrWebSocketUrl"`
}

type qrStatusRequest struct {
	PRN            string `json:"prn"`
	MerchantCode   string `json:"merchantCode"`
	DataValidation string `json:"dataValidation"`
	Username       string `json:"username"`
	Password       string `json:"password"`
}

type qrStatusResponse struct {
	PaymentStatus  string `json:"paymentStatus"`
	FonepayTraceID any    `json:"fonepayTraceId"`
}
