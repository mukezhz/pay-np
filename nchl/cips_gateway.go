package nchl

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mukezhz/pay-np/errorz"
	"github.com/mukezhz/pay-np/utils"
	"golang.org/x/crypto/pkcs12"
)

type CIPSGateway struct {
	apiURL string
	client *http.Client
}

func NewCIPSGateway(apiURL string) *CIPSGateway {
	return &CIPSGateway{
		apiURL: apiURL,
		client: &http.Client{},
	}
}

func (c *CIPSGateway) LoadPrivateKeyFromPFX(pfxPath, password string) (*rsa.PrivateKey, error) {
	pfxData, err := os.ReadFile(pfxPath)
	if err != nil {
		return nil, errorz.ErrCIPSFailedToReadPFX.Wrap(err)
	}

	privKey, _, err := pkcs12.Decode(pfxData, password)
	if err != nil {
		return nil, errorz.ErrCIPSFailedToDecodePFX.Wrap(err)
	}

	key, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errorz.ErrCIPSPrivateKeyNotRSA
	}

	return key, nil
}

func (c *CIPSGateway) GenerateDigitalSignatureWithRSA(params utils.GenerateDigitalSignatureWithRSAParams) (string, error) {
	signature, err := utils.GenerateDigitalSignatureWithRSA(params)
	if err != nil {
		return "", errorz.ErrCIPSSigningFailed.Wrap(err)
	}
	return signature, nil
}

func (c *CIPSGateway) ValidateTxn(req CIPSJSONRequest, authHeader string) (*ValidateTxnResponse, int, error) {
	endpoint := c.apiURL + "/connectipswebws/api/creditor/validatetxn"
	respBytes, statusCode, err := c.postJSON(endpoint, req, authHeader)
	if err != nil {
		return nil, statusCode, err
	}

	var resp ValidateTxnResponse
	if err := json.Unmarshal([]byte(respBytes), &resp); err != nil {
		return nil, statusCode, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	fmt.Printf("ValidateTxn Response: %+v\n", resp)
	return &resp, statusCode, nil
}

func (c *CIPSGateway) GetTxnDetail(req CIPSJSONRequest, authHeader string) (*GetTxnDetailResponse, int, error) {
	endpoint := c.apiURL + "/connectipswebws/api/creditor/gettxndetail"
	respBytes, statusCode, err := c.postJSON(endpoint, req, authHeader)
	if err != nil {
		return nil, statusCode, err
	}

	var resp GetTxnDetailResponse
	if err := json.Unmarshal([]byte(respBytes), &resp); err != nil {
		return nil, statusCode, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &resp, statusCode, nil
}

func (c *CIPSGateway) postJSON(endpoint string, payload interface{}, authHeader string) (string, int, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	return c.doRequest(req)
}

func (c *CIPSGateway) doRequest(req *http.Request) (string, int, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}

	return string(respBytes), resp.StatusCode, nil
}
