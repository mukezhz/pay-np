package errorz

import (
	"errors"
	"fmt"
)

type EsewaErrorz struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewEsewaError(code, message string) *EsewaErrorz {
	return &EsewaErrorz{
		Code:    code,
		Message: message,
	}
}

func (e *EsewaErrorz) Error() string {
	return e.Message
}

func (e *EsewaErrorz) Is(target error) bool {
	return errors.Is(target, e)
}

func (e *EsewaErrorz) Wrap(target error) error {
	if target == nil {
		return e
	}
	return fmt.Errorf("%w: %w", e, target)
}

var (
	ErrEsewaTotalAmount             = NewEsewaError("TOTAL_AMOUNT_MANDATORY", "total_amount is mandatory")
	ErrEsewaTransactionUUID         = NewEsewaError("TRANSACTION_UUID_IS_MANDATORY", "transaction_uuid is mandatory")
	ErrEsewaProductCode             = NewEsewaError("PRODUCT_CODE_MANDATORY", "product_code is mandatory")
	ErrEsewaInvalidDataForSignature = NewEsewaError("INVALID_DATA_FOR_SIGNATURE", "invalid data for signature")
	ErrEsewaInvalidSignature        = NewEsewaError("INVALID_SIGNATURE", "invalid signature")
)
