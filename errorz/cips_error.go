package errorz

import (
	"errors"
	"fmt"
)

type CIPSErrorz struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewCIPSError(code, message string) *CIPSErrorz {
	return &CIPSErrorz{
		Code:    code,
		Message: message,
	}
}

func (e *CIPSErrorz) Error() string {
	return e.Message
}

func (e *CIPSErrorz) Is(target error) bool {
	return errors.Is(target, e)
}

func (e *CIPSErrorz) Wrap(target error) error {
	if target == nil {
		return e
	}
	return fmt.Errorf("%w: %w", e, target)
}

var (
	ErrCIPSSigningFailed               = NewCIPSError("SIGNING_FAILED", "Digital signature generation failed")
	ErrCIPSFailedToReadPFX             = NewCIPSError("READ_PFX_FAILED", "Failed to read PFX file")
	ErrCIPSFailedToDecodePFX           = NewCIPSError("DECODE_PFX_FAILED", "Failed to decode PFX file")
	ErrCIPSPrivateKeyNotRSA            = NewCIPSError("PRIVATE_KEY_NOT_RSA", "Private key is not RSA")
	ErrCIPSInvalidValidateTxnResponse  = NewCIPSError("INVALID_VALIDATE_TXN_RESPONSE", "Invalid response from ValidateTxn API")
	ErrCIPSInvalidGetTxnDetailResponse = NewCIPSError("INVALID_GET_TXN_DETAIL_RESPONSE", "Invalid response from GetTxnDetail API")
)
