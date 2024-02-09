package esewa

import "errors"

var (
	ErrTotalAmount             = errors.New("total_amount is mandatory")
	ErrTransactionUUID         = errors.New("transaction_uuid is mandatory")
	ErrProductCode             = errors.New("product_code is mandatory")
	ErrInvalidDataForSignature = errors.New("invalid data for signature")
	ErrInvalidSignature        = errors.New("invalid signature")
)
