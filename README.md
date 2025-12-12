# Pay-NP 🇳🇵

**Pay-NP** is a unified Go SDK for integrating with all major payment providers in Nepal. This is your one-stop solution for handling digital payments in the Nepalese market.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 🚀 Features

- **Multiple Payment Providers**: Unified interface for eSewa, ConnectIPS (NCHL), and more
- **Type-Safe**: Strongly typed API for all payment operations
- **Secure**: Built-in signature generation and verification
- **Easy Integration**: Simple and intuitive API design
- **Well-Tested**: Comprehensive test coverage
- **Production Ready**: Used in production environments

## 📦 Supported Payment Providers

| Provider | Status | Features |
|----------|--------|----------|
| **eSewa** | ✅ Supported | Signature generation, Payment verification, HMAC-SHA256 |
| **ConnectIPS (NCHL/CIPS)** | ✅ Supported | Transaction validation, Transaction details, RSA digital signatures |

## 🔧 Installation

```bash
go get github.com/mukezhz/pay-np
```

## 📚 Usage

### eSewa Integration

eSewa is one of Nepal's leading digital payment platforms. Here's how to integrate it:

```go
package main

import (
    "fmt"
    "github.com/mukezhz/pay-np/esewa"
)

func main() {
    // Your eSewa secret key
    secret := "your_esewa_secret_key"
    
    // Create payment payload
    payload := &esewa.EsewaPayload{
        Amount:                "100",
        TaxAmount:             "13",
        TotalAmount:           "113",
        TransactionUUID:       "unique-transaction-id",
        ProductServiceCharge:  "0",
        ProductDeliveryCharge: "0",
        ProductCode:           "EPAYTEST",
        SuccessURL:            "https://yoursite.com/success",
        FailureURL:            "https://yoursite.com/failure",
        SignedFieldNames:      "total_amount,transaction_uuid,product_code",
    }
    
    // Create eSewa client
    client, err := esewa.New(secret, payload)
    if err != nil {
        panic(err)
    }
    
    // Generate signature for payment request
    signature, err := client.GenerateSignature()
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Signature:", signature)
    
    // Verify payment response (after payment completion)
    err = client.VerifySignature(base64EncodedResponse)
    if err != nil {
        fmt.Println("Payment verification failed:", err)
        return
    }
    
    fmt.Println("Payment verified successfully!")
    fmt.Println("Transaction Code:", client.ReponsePayload.TransactionCode)
    fmt.Println("Status:", client.ReponsePayload.Status)
}
```

### ConnectIPS (NCHL/CIPS) Integration

ConnectIPS is Nepal's interbank payment gateway. Here's how to use it:

```go
package main

import (
    "fmt"
    "github.com/mukezhz/pay-np/nchl"
    "github.com/mukezhz/pay-np/utils"
)

func main() {
    // Create CIPS client
    client := nchl.New("https://uat.connectips.com")
    
    // Load your PFX certificate
    privateKey, err := client.LoadPrivateKeyFromPFX("path/to/cert.pfx", "password")
    if err != nil {
        panic(err)
    }
    
    // Generate digital signature
    payload := "your_payload_string"
    signature, err := client.GenerateDigitalSignatureWithRSA(utils.GenerateDigitalSignatureWithRSAParams{
        PfxCertPath: "path/to/cert.pfx",
        Password:    "password",
        Payload:     payload,
        PrivKey:     privateKey,
    })
    if err != nil {
        panic(err)
    }
    
    // Validate transaction
    req := nchl.CIPSJSONRequest{
        MerchantID:  12345,
        AppID:       "YOUR_APP_ID",
        TxnAmount:   "1000",
        ReferenceID: "unique-ref-id",
        Token:       "your_token",
    }
    
    resp, statusCode, err := client.ValidateTxn(req, "Bearer "+signature)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Status: %s, Code: %d\n", resp.Status, statusCode)
    
    // Get transaction details
    details, statusCode, err := client.GetTxnDetail(req, "Bearer "+signature)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Transaction ID: %d\n", details.TxnID)
    fmt.Printf("Amount: %.2f\n", details.TxnAmount)
}
```

## 🏗️ Project Structure

```
pay-np/
├── esewa/              # eSewa payment integration
│   ├── esewa_client.go # Client implementation
│   ├── types.go        # Data structures
│   └── README.md       # eSewa-specific documentation
├── nchl/               # ConnectIPS (NCHL) integration
│   ├── cips_client.go  # CIPS client implementation
│   └── types.go        # Data structures
├── errorz/             # Custom error types
│   ├── esewa_error.go  # eSewa-specific errors
│   └── cips_error.go   # CIPS-specific errors
├── utils/              # Utility functions
│   └── crypto.go       # Cryptographic utilities (HMAC, RSA, SHA256)
└── esewa_test.go       # Test cases
```

## 🔐 Security

This SDK implements industry-standard security practices:

- **HMAC-SHA256** for eSewa signature generation and verification
- **RSA Digital Signatures** with PKCS#1 v1.5 for ConnectIPS
- **Base64 Encoding** for secure data transmission
- **Certificate-based Authentication** for ConnectIPS using PFX/PKCS12

## 🧪 Testing

Run the test suite:

```bash
go test ./...
```

Run specific tests:

```bash
go test esewa_test.go
```

## 🛠️ Error Handling

The SDK provides comprehensive error handling with custom error types:

```go
// eSewa errors
errorz.ErrEsewaTotalAmount          // Missing total amount
errorz.ErrEsewaTransactionUUID      // Missing transaction UUID
errorz.ErrEsewaProductCode          // Missing product code
errorz.ErrEsewaInvalidDataForSignature  // Invalid signature data

// CIPS errors
errorz.ErrCIPSFailedToReadPFX       // Failed to read PFX certificate
errorz.ErrCIPSFailedToDecodePFX     // Failed to decode PFX
errorz.ErrCIPSPrivateKeyNotRSA      // Private key is not RSA
errorz.ErrCIPSSigningFailed         // Signature generation failed
```

## 🌐 API Endpoints

### eSewa
- **Production**: `https://epay.esewa.com.np/api/epay/`
- **Testing**: `https://uat.esewa.com.np/epay/`

### ConnectIPS (NCHL)
- **Production**: `https://connectips.com/`
- **UAT**: `https://uat.connectips.com/`

## 📋 Requirements

- Go 1.24 or higher
- Valid merchant accounts with respective payment providers
- For ConnectIPS: PFX certificate from your bank

## 🤝 Contributing

Contributions are welcome! If you'd like to add support for more Nepalese payment providers or improve existing implementations:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/new-provider`)
3. Commit your changes (`git commit -am 'Add support for XYZ payment provider'`)
4. Push to the branch (`git push origin feature/new-provider`)
5. Open a Pull Request

## 📝 Roadmap

- [ ] Add Khalti payment integration
- [ ] Add IME Pay integration
- [ ] Add Prabhu Pay integration
- [ ] Add webhook handling utilities
- [ ] Add payment reconciliation tools
- [ ] Improve documentation with more examples
- [ ] Add integration tests with mock servers

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 💬 Support

If you encounter any issues or have questions:

- Open an issue on GitHub
- Check the documentation in respective provider directories
- Review the test files for usage examples

## 🙏 Acknowledgments

- eSewa for their payment gateway services
- NCHL (National Clearing and Settlement Limited) for ConnectIPS
- The Go community for excellent cryptographic libraries

## ⚠️ Disclaimer

This is an unofficial SDK. Please ensure you comply with all terms and conditions of the respective payment providers. Always test thoroughly in sandbox/UAT environments before going to production.

---

Made with ❤️ for the Nepalese developer community
