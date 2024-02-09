# Payment Nepal SDK for Golang

### Flow of Esewa

- Get the merchant account credentials (Secret to sign the payload)
- Sign the payload using secret
- If signature you generate using the secret key matches you will be redirect to the Esewa payment page
- Complete the payment
- On payment `success` you will be redirected to the `success_url` you have provided
- On payment `failure` you will be redirected to the `failure_url` you have provided
- In redirected url the `data` query param will be appended (which is base64 encoded text) 

### Integrate Esewa with ease

- Install Dependency
```go
go get github.com/mukezhz/pay-np/esewa@latest
```
- Initialize the config for esewa
```go
secret := "8gBm/:&EnhH.1/q"
payload := &esewa.EsewaPayload{
		Amount:                "100",
		TaxAmount:             "13",
		TotalAmount:           "133",
		TransactionUUID:       "1234567890", // this should be unique for each transaction
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
```
- Generate the signature
```go
s, err := e.GenerateSignature()
if err != nil {
 	fmt.Println(err)
}
```
- Pass the `signature` obtained to [Esewa URL[this is dev url]](https://rc-epay.esewa.com.np/api/epay/main/v2/form)
- After payment you will get the `data` in the redirected url
- Use that `base64 encoded text` to verify weather the payload in changed or not
```go
err = e.VerifySignature("eyJ0cmFuc2FjdGlvbl9jb2RlIjoiMDAwNlRZMyIsInN0YXR1cyI6IkNPTVBMRVRFIiwidG90YWxfYW1vdW50IjoiMTMzLjAiLCJ0cmFuc2FjdGlvbl91dWlkIjoiMTIzNDU2Nzg5MCIsInByb2R1Y3RfY29kZSI6IkVQQVlURVNUIiwic2lnbmVkX2ZpZWxkX25hbWVzIjoidHJhbnNhY3Rpb25fY29kZSxzdGF0dXMsdG90YWxfYW1vdW50LHRyYW5zYWN0aW9uX3V1aWQscHJvZHVjdF9jb2RlLHNpZ25lZF9maWVsZF9uYW1lcyIsInNpZ25hdHVyZSI6Ik1GRWNNWi8zMFdWZXphblZaSEg0SDFuSVY4cEd3eXpaeGdndGt5ZTJWWHc9In0=")
if err != nil {
  fmt.Println(err)
}
```

**Note:** For complete reference please visit official docs [here](https://developer.esewa.com.np/pages/Epay-V2)

---

**THANK YOU 🙏**
