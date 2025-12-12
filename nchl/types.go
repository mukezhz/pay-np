package nchl

type CIPSLoginRequest struct {
	Endpoint            string `json:"endpoint"`
	MerchantID          int    `json:"merchant_id"`
	AppID               string `json:"app_id"`
	AppName             string `json:"app_name"`
	TransactionID       string `json:"transaction_id"`
	TransactionDate     string `json:"transaction_date"`
	TransactionCurrency string `json:"transaction_currency"`
	TransactionAmount   int    `json:"transaction_amount"`
	ReferenceID         string `json:"reference_id"`
	Remarks             string `json:"remarks"`
	Particulars         string `json:"particulars"`
	Token               string `json:"token"`
}

type CIPSJSONRequest struct {
	MerchantID  int    `json:"merchantId"`
	AppID       string `json:"appId"`
	TxnAmount   string `json:"txnAmt"`
	ReferenceID string `json:"referenceId"`
	Token       string `json:"token,omitempty"`
}

type ValidateTxnResponse struct {
	MerchantID  int    `json:"merchantId"`
	AppID       string `json:"appId"`
	ReferenceID string `json:"referenceId"`
	TxnAmount   string `json:"txnAmt"`
	Token       string `json:"token"`
	Status      string `json:"status"`
	StatusDesc  string `json:"statusDesc"`
}

type GetTxnDetailResponse struct {
	Status          string  `json:"status"`
	StatusDesc      string  `json:"statusDesc"`
	MerchantID      int     `json:"merchantId"`
	AppID           string  `json:"appId"`
	ReferenceID     string  `json:"referenceId"`
	TxnAmount       float64 `json:"txnAmt"`
	Token           string  `json:"token"`
	DebitBankCode   string  `json:"debitBankCode"`
	TxnID           int64   `json:"txnId"`
	BatchID         int64   `json:"batchId"`
	TxnDate         int64   `json:"txnDate"`
	TxnCrncy        string  `json:"txnCrncy"`
	ChargeAmt       float64 `json:"chargeAmt"`
	ChargeLiability string  `json:"chargeLiability"`
	RefID           string  `json:"refId"`
	Remarks         string  `json:"remarks"`
	Particulars     string  `json:"particulars"`
	CreditStatus    string  `json:"creditStatus"`
}
