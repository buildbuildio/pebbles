package requests

import "github.com/buildbuildio/pebbles/gqlerrors"

const (
	SubConnectionInit      = "connection_init"
	SubConnectionAck       = "connection_ack"
	SubConnectionKeepAlive = "ka"
	SubConnectionError     = "connection_error"
	SubConnectionTerminate = "connection_terminate"
	SubStart               = "start"
	SubData                = "data"
	SubError               = "error"
	SubComplete            = "complete"
	SubStop                = "stop"
)

// ClientSubMsg defines possible client messages
type ClientSubMsg struct {
	ID      string   `json:"id,omitempty"`
	Type    string   `json:"type"`
	Payload *Request `json:"payload,omitempty"`
}

// ServerSubMsg defines possible server messages
type ServerSubMsg struct {
	ID      string    `json:"id,omitempty"`
	Type    string    `json:"type"`
	Payload *Response `json:"payload,omitempty"`
}

// ServerSubErrorMsg defines msg for type error
type ServerSubErorrMsg struct {
	ID      string              `json:"id,omitempty"`
	Type    string              `json:"type"`
	Payload gqlerrors.ErrorList `json:"payload,omitempty"`
}
