package connectips

import (
	"crypto/rsa"
	"net/http"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

type Config struct {
	MerchantID string
	AppID      string
	AppName    string
	// Username defaults to AppID.
	Username    string
	Password    string
	PrivateKey  *rsa.PrivateKey
	Environment paynp.Environment
	Host        string
	HTTPClient  *http.Client
	Now         func() time.Time
}

type txnRequest struct {
	MerchantID  int64  `json:"merchantId"`
	AppID       string `json:"appId"`
	ReferenceID string `json:"referenceId"`
	TxnAmt      int64  `json:"txnAmt"`
	Token       string `json:"token"`
}

type txnDetail struct {
	Status      string  `json:"status"`
	StatusDesc  string  `json:"statusDesc"`
	ReferenceID string  `json:"referenceId"`
	TxnAmt      float64 `json:"txnAmt"`
	TxnID       int64   `json:"txnId"`
}
