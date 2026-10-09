package imepay

import (
	"encoding/json"
	"net/http"

	paynp "github.com/mukezhz/pay-np"
)

type Config struct {
	MerchantCode string
	Module       string
	APIUser      string
	APIPassword  string
	Environment  paynp.Environment
	Host         string
	HTTPClient   *http.Client
}

type apiResponse struct {
	ResponseCode        json.RawMessage `json:"ResponseCode"`
	ResponseDescription string          `json:"ResponseDescription"`
	TokenID             json.RawMessage `json:"TokenId"`
	RefID               json.RawMessage `json:"RefId"`
	TransactionID       json.RawMessage `json:"TransactionId"`
	Amount              json.RawMessage `json:"Amount"`
	TranAmount          json.RawMessage `json:"TranAmount"`
}
