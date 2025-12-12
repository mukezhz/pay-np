package utils

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"log"
)

func HmacSHA256(secretKey string, data string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	_, err := h.Write([]byte(data))
	if err != nil {
		log.Fatalf("Failed to write data to HMAC hash: %v", err)
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func Base64Decode(data string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return []byte{}, err
	}
	return decoded, nil
}

func ComputeSHA256Digest(payload string) []byte {
	hash := sha256.Sum256([]byte(payload))
	return hash[:]
}

type GenerateDigitalSignatureWithRSAParams struct {
	PfxCertPath string
	Password    string
	Payload     string
	PrivKey     *rsa.PrivateKey
}

func GenerateDigitalSignatureWithRSA(params GenerateDigitalSignatureWithRSAParams) (string, error) {
	digest := ComputeSHA256Digest(params.Payload)
	signature, err := rsa.SignPKCS1v15(rand.Reader, params.PrivKey, crypto.SHA256, digest)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}
