package jsonrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
)

var requestID uint64

// ClientRequest represents a JSON-RPC request
type ClientRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      uint64      `json:"id"`
	Version string      `json:"jsonrpc"`
}

// ClientResponse represents a JSON-RPC response
type ClientResponse struct {
	Result  json.RawMessage `json:"result"`
	Error   *Error          `json:"error"`
	ID      uint64          `json:"id"`
	Version string          `json:"jsonrpc"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("jsonrpc: code %d message: %s", e.Code, e.Message)
}

// EncodeClientRequest encodes a JSON-RPC client request
func EncodeClientRequest(method string, args interface{}) ([]byte, error) {
	id := atomic.AddUint64(&requestID, 1)
	req := &ClientRequest{
		Method:  method,
		Params:  args,
		ID:      id,
		Version: "2.0",
	}
	return json.Marshal(req)
}

// DecodeClientResponse decodes a JSON-RPC response
func DecodeClientResponse(r io.Reader, reply interface{}) error {
	var resp ClientResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return fmt.Errorf("jsonrpc: failed to decode response: %w", err)
	}
	
	if resp.Error != nil {
		return resp.Error
	}
	
	if reply != nil && len(resp.Result) > 0 {
		return json.Unmarshal(resp.Result, reply)
	}
	
	return nil
}