package httpsvr

type ErrorObject struct {
	Meta   interface{}  `json:"meta,omitempty"`
	Source *ErrorSource `json:"source,omitempty"`
	Links  *ErrorLinks  `json:"links,omitempty"`
	Status string       `json:"status,omitempty"`
	Detail string       `json:"detail,omitempty"`
	Title  string       `json:"title,omitempty"`
	Code   string       `json:"code,omitempty"`
	ID     string       `json:"id"`
}

type ErrorLinks struct {
	About string `json:"about,omitempty"`
	Type  string `json:"type,omitempty"`
}

type ErrorSource struct {
	Pointer   string `json:"pointer,omitempty"`
	Parameter string `json:"parameter,omitempty"`
	Header    string `json:"header,omitempty"`
}

type ErrorResponse struct {
	Errors []ErrorObject `json:"errors"`
}
