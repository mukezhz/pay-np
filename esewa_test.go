package main

import (
	"fmt"

	"github.com/mukezhz/pay-np/esewa"
)

func main() {
	secret := "8gBm/:&EnhH.1/q"
	payload := &esewa.EsewaPayload{
		Amount:                "100",
		TaxAmount:             "13",
		TotalAmount:           "133",
		TransactionUUID:       "1234567890",
		ProductServiceCharge:  "10",
		ProductDeliveryCharge: "10",
		ProductCode:           "EPAYTEST",
		SuccessURL:            "http://localhost:8000/success",
		FailureURL:            "http://localhost:8000/failure",
		SignedFieldNames:      "total_amount,transaction_uuid,product_code",
		Signature:             "",
	}
	e, err := esewa.New(secret, payload)
	if err != nil {
		fmt.Println(err)
	}
	// s, err := e.GenerateSignature()
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// fmt.Println(s)

	err = e.VerifySignature("eyJ0cmFuc2FjdGlvbl9jb2RlIjoiMDAwNlRZMyIsInN0YXR1cyI6IkNPTVBMRVRFIiwidG90YWxfYW1vdW50IjoiMTMzLjAiLCJ0cmFuc2FjdGlvbl91dWlkIjoiMTIzNDU2Nzg5MCIsInByb2R1Y3RfY29kZSI6IkVQQVlURVNUIiwic2lnbmVkX2ZpZWxkX25hbWVzIjoidHJhbnNhY3Rpb25fY29kZSxzdGF0dXMsdG90YWxfYW1vdW50LHRyYW5zYWN0aW9uX3V1aWQscHJvZHVjdF9jb2RlLHNpZ25lZF9maWVsZF9uYW1lcyIsInNpZ25hdHVyZSI6Ik1GRWNNWi8zMFdWZXphblZaSEg0SDFuSVY4cEd3eXpaeGdndGt5ZTJWWHc9In0=")
	if err != nil {
		fmt.Println(err)
	}

}
