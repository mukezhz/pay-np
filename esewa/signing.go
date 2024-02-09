package esewa

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"log"
)

func hmacSHA256(secretKey string, data string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	_, err := h.Write([]byte(data))
	if err != nil {
		log.Fatalf("Failed to write data to HMAC hash: %v", err)
	}
	// Encode the hash to a hexadecimal string
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func base64Decode(data string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return []byte{}, err
	}
	return decoded, nil
}
