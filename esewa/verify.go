package esewa

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func VerifyMobileTransaction(input VerifyInput) ([]byte, error) {
	if input.TxnRefId == "" && (input.ProductId == "" || input.Amount == "") {
		return nil, errors.New("either txnRefId or both productId and amount must be provided")
	}

	baseURL := "https://rc.esewa.com.np/mobile/transaction"
	if input.Environment == "live" {
		baseURL = "https://esewa.com.np/mobile/transaction"
	}
	var url string
	if input.TxnRefId != "" {
		url = fmt.Sprintf("%s?txnRefId=%s", baseURL, input.TxnRefId)
	} else {
		url = fmt.Sprintf("%s?productId=%s&amount=%s", baseURL, input.ProductId, input.Amount)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// Set merchantId and merchantSecret as headers if provided
	if input.MerchantId != "" {
		req.Header.Set("merchantId", input.MerchantId)
	}
	if input.MerchantSecret != "" {
		req.Header.Set("merchantSecret", input.MerchantSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("verification failed: status %d", resp.StatusCode)
	}

	return body, nil
}

func ParseVerificationResponse(data []byte) ([]EsewaTransactionVerificationResponse, error) {
	var resp []EsewaTransactionVerificationResponse
	err := json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
