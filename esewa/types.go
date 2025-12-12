package esewa

type EsewaPayload struct {
	Amount                string `json:"amount"`
	TaxAmount             string `json:"tax_amount"`
	ProductServiceCharge  string `json:"product_service_charge"`
	ProductDeliveryCharge string `json:"product_delivery_charge"`
	ProductCode           string `json:"product_code"`
	TotalAmount           string `json:"total_amount"`
	TransactionUUID       string `json:"transaction_uuid"`
	SuccessURL            string `json:"success_url"`
	FailureURL            string `json:"failure_url"`
	SignedFieldNames      string `json:"signed_field_names"`
	Signature             string `json:"signature"`
}

type EsewaVerifyPayload struct {
	ProductCode      string `json:"product_code"`
	Signature        string `json:"signature"`
	SignedFieldNames string `json:"signed_field_names"`
	Status           string `json:"status"`
	TotalAmount      string `json:"total_amount"`
	TransactionCode  string `json:"transaction_code"`
	TransactionUUID  string `json:"transaction_uuid"`
}

type EsewaResponse struct {
	PID         string `json:"pid"`
	SCD         string `json:"scd"`
	TotalAmount string `json:"total_amount"`
	Status      string `json:"status"`
	RefID       string `json:"ref_id"`
}
