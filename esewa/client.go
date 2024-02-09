package esewa

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

func New(secret string, payload *EsewaPayload) (*EsewaConfig, error) {
	p := EsewaConfig{
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

func (e *EsewaConfig) Validate() error {
	// total_amount,transaction_uuid,product_code mandatory fields
	if e.Payload.TotalAmount == "" {
		return ErrTotalAmount
	}
	if e.Payload.TransactionUUID == "" {
		return ErrTransactionUUID
	}
	if e.Payload.ProductCode == "" {
		return ErrProductCode
	}
	return nil
}

func (e *EsewaConfig) getInputForSignature(signedFieldNames string) (string, error) {
	splittedSignedFieldNames := strings.Split(signedFieldNames, ",")
	if len(splittedSignedFieldNames) < 3 {
		return "", ErrInvalidDataForSignature
	}

	var signatureDate []string
	for _, signedFieldName := range splittedSignedFieldNames {
		if e.signatureMap[signedFieldName] == "" {
			return "", ErrInvalidDataForSignature
		}
		signatureDate = append(signatureDate, fmt.Sprintf("%s=%s", signedFieldName, e.signatureMap[signedFieldName]))
	}
	return strings.Join(signatureDate, ","), nil
}

func (e *EsewaConfig) GenerateSignature() (string, error) {
	data, err := e.getInputForSignature(e.Payload.SignedFieldNames)
	if err != nil {
		return "", err
	}
	fmt.Println(data)
	return hmacSHA256(e.Secret, data), nil
}

func (e *EsewaConfig) VerifySignature(data string) error {
	d, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	json.Unmarshal(d, &e.ReponsePayload)
	sm, err := setupSignatureMap[EsewaVerifyPayload](*e.ReponsePayload)
	if err != nil {
		return err
	}
	e.signatureMap = *sm
	i, err := e.getInputForSignature(e.ReponsePayload.SignedFieldNames)
	if err != nil {
		return err
	}
	signature := hmacSHA256(e.Secret, i)
	if e.ReponsePayload.Signature != signature {
		return ErrInvalidSignature
	}

	return nil
}
