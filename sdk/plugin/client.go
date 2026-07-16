package plugin

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Client exchanges bounded JSON-RPC messages over an already supervised
// plugin process. It is useful to host implementations and conformance tests.
type Client struct {
	encoder *json.Encoder
	scanner *bufio.Scanner
	nextID  int
}

// NewClient creates a protocol client over separate input and output streams.
func NewClient(input io.Reader, output io.Writer) *Client {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), MaxMessageBytes)
	return &Client{encoder: json.NewEncoder(output), scanner: scanner, nextID: 1}
}

// Call invokes one method and validates its matching response envelope.
func (c *Client) Call(method string, params, result any) error {
	id := c.nextID
	c.nextID++
	request := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := c.encoder.Encode(request); err != nil {
		return fmt.Errorf("write plugin request: %w", err)
	}
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return fmt.Errorf("read plugin response: %w", err)
		}
		return io.ErrUnexpectedEOF
	}
	line := c.scanner.Bytes()
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *RPCError       `json:"error"`
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil || response.JSONRPC != "2.0" || response.ID != id {
		return errors.New("plugin returned an invalid JSON-RPC response")
	}
	if response.Error != nil {
		return response.Error
	}
	if len(response.Result) == 0 {
		return errors.New("plugin response omitted result")
	}
	resultDecoder := json.NewDecoder(bytes.NewReader(response.Result))
	resultDecoder.DisallowUnknownFields()
	if err := resultDecoder.Decode(result); err != nil {
		return fmt.Errorf("decode plugin result: %w", err)
	}
	if err := ensureEOF(resultDecoder); err != nil {
		return errors.New("plugin result contains multiple JSON documents")
	}
	return nil
}
