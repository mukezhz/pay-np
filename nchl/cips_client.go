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

type CIPSClient struct {
	apiURL string
	client *http.Client
}

func New(apiURL string) *CIPSClient {
	return &CIPSClient{
		apiURL: apiURL,
		client: &http.Client{},
	}
}

func (c *CIPSClient) LoadPrivateKeyFromPFX(pfxPath, password string) (*rsa.PrivateKey, error) {
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

func (c *CIPSClient) GenerateDigitalSignatureWithRSA(params utils.GenerateDigitalSignatureWithRSAParams) (string, error) {
	signature, err := utils.GenerateDigitalSignatureWithRSA(params)
	if err != nil {
		return "", errorz.ErrCIPSSigningFailed.Wrap(err)
	}
	return signature, nil
}

func (c *CIPSClient) ValidateTxn(req CIPSJSONRequest, authHeader string) (*ValidateTxnResponse, int, error) {
	endpoint := c.apiURL + "/connectipswebws/api/creditor/validatetxn"
	respBytes, statusCode, err := c.postJSON(endpoint, req, authHeader)
	if err != nil {
		return nil, statusCode, err
	}

	var resp ValidateTxnResponse
	if err := json.Unmarshal([]byte(respBytes), &resp); err != nil {
		return nil, statusCode, errorz.ErrCIPSInvalidValidateTxnResponse.Wrap(err)
	}

	return &resp, statusCode, nil
}

func (c *CIPSClient) GetTxnDetail(req CIPSJSONRequest, authHeader string) (*GetTxnDetailResponse, int, error) {
	endpoint := c.apiURL + "/connectipswebws/api/creditor/gettxndetail"
	respBytes, statusCode, err := c.postJSON(endpoint, req, authHeader)
	if err != nil {
		return nil, statusCode, err
	}

	var resp GetTxnDetailResponse
	if err := json.Unmarshal([]byte(respBytes), &resp); err != nil {
		return nil, statusCode, errorz.ErrCIPSInvalidGetTxnDetailResponse.Wrap(err)
	}

	return &resp, statusCode, nil
}

func (c *CIPSClient) postJSON(endpoint string, payload interface{}, authHeader string) (string, int, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	return c.doRequest(req)
}

func (c *CIPSClient) doRequest(req *http.Request) (string, int, error) {
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
