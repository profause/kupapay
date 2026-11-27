package apiresponse

// SuccessDoc is the swagger model for successful API responses.
// Use it in success annotations with a typed data-field override.
type SuccessDoc struct {
	Status  bool        `json:"status"`
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

// FailureDoc is the swagger model for failed API responses.
type FailureDoc struct {
	Status  bool        `json:"status"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
