package esewa

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mukezhz/pay-np/errorz"
	"github.com/mukezhz/pay-np/utils"
)

type EsewaClient struct {
	Payload        *EsewaPayload
	Signature      string
	Secret         string
	signatureMap   map[string]string
	ReponsePayload *EsewaVerifyPayload
}

func New(secret string, payload *EsewaPayload) (*EsewaClient, error) {
	p := EsewaClient{
		Payload: payload,
		Secret:  secret,
	}
	sm, err := setupSignatureMap[EsewaPayload](*p.Payload)
	if err != nil {
		return nil, err
	}
	p.signatureMap = *sm
	return &p, nil
}

func setupSignatureMap[T EsewaPayload | EsewaVerifyPayload](p T) (*map[string]string, error) {
	j, _ := json.Marshal(p)

	var data map[string]any
	err := json.Unmarshal(j, &data)
	if err != nil {
		return nil, err
	}

	stringMap := make(map[string]string)
	for key, value := range data {
		stringMap[key] = fmt.Sprintf("%v", value)
	}
	return &stringMap, nil
}

func (e *EsewaClient) validate() error {
	// total_amount,transaction_uuid,product_code mandatory fields
	if e.Payload.TotalAmount == "" {
		return errorz.ErrEsewaTotalAmount
	}
	if e.Payload.TransactionUUID == "" {
		return errorz.ErrEsewaTransactionUUID
	}
	if e.Payload.ProductCode == "" {
		return errorz.ErrEsewaProductCode
	}
	return nil
}

func (e *EsewaClient) getInputForSignature(signedFieldNames string) (string, error) {
	splittedSignedFieldNames := strings.Split(signedFieldNames, ",")
	if len(splittedSignedFieldNames) < 3 {
		return "", errorz.ErrEsewaInvalidDataForSignature
	}

	var signatureDate []string
	for _, signedFieldName := range splittedSignedFieldNames {
		if e.signatureMap[signedFieldName] == "" {
			return "", errorz.ErrEsewaInvalidDataForSignature
		}
		signatureDate = append(signatureDate, fmt.Sprintf("%s=%s", signedFieldName, e.signatureMap[signedFieldName]))
	}
	return strings.Join(signatureDate, ","), nil
}

func (e *EsewaClient) GenerateSignature() (string, error) {
	err := e.validate()
	if err != nil {
		return "", err
	}
	data, err := e.getInputForSignature(e.Payload.SignedFieldNames)
	if err != nil {
		return "", err
	}
	return utils.HmacSHA256(e.Secret, data), nil
}

func (e *EsewaClient) VerifySignature(data string) error {
	d, err := utils.Base64Decode(data)
	if err != nil {
		return err
	}
	err = json.Unmarshal(d, &e.ReponsePayload)
	if err != nil {
		return err
	}
	sm, err := setupSignatureMap[EsewaVerifyPayload](*e.ReponsePayload)
	if err != nil {
		return err
	}
	e.signatureMap = *sm
	i, err := e.getInputForSignature(e.ReponsePayload.SignedFieldNames)
	if err != nil {
		return err
	}
	signature := utils.HmacSHA256(e.Secret, i)
	if e.ReponsePayload.Signature != signature {
		return errorz.ErrEsewaInvalidSignature
	}

	return nil
}
