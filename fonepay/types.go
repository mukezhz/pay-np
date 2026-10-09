package fonepay

import (
	"net/http"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

type Config struct {
	MerchantCode string // PID
	SecretKey    string
	Environment  paynp.Environment
	Host         string
	HTTPClient   *http.Client
	Now          func() time.Time
}

type verifyResponse struct {
	Success      bool   `xml:"success"`
	ResponseCode string `xml:"response_code"`
	Message      string `xml:"message"`
	TxnAmount    string `xml:"txnAmount"`
}
