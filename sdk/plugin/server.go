package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a transport-safe JSON-RPC failure.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

// Serve handles JSON-RPC requests until input closes or the context is
// cancelled. Protocol data is written only to stdout; plugins should send
// diagnostic logs to stderr.
func Serve(ctx context.Context, input io.Reader, output io.Writer, manifest Manifest, handler Handler) error {
	if handler == nil {
		return errors.New("plugin handler is required")
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	manifest.Executable = ""
	manifest.Arguments = nil
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), MaxMessageBytes)
	encoder := json.NewEncoder(output)
	initialized := false
	granted := []Scope(nil)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		request, decodeErr := decodeRequest(scanner.Bytes())
		if decodeErr != nil {
			if err := encoder.Encode(rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &RPCError{Code: -32700, Message: decodeErr.Error()}}); err != nil {
				return err
			}
			continue
		}
		result, rpcErr := dispatch(request, manifest, handler, &initialized, &granted)
		if err := encoder.Encode(rpcResponse{JSONRPC: "2.0", ID: request.ID, Result: result, Error: rpcErr}); err != nil {
			return fmt.Errorf("write plugin response: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read plugin request: %w", err)
	}
	return nil
}

func decodeRequest(raw []byte) (rpcRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request rpcRequest
	if err := decoder.Decode(&request); err != nil {
		return rpcRequest{}, errors.New("invalid JSON-RPC request")
	}
	if request.JSONRPC != "2.0" || len(request.ID) == 0 || string(request.ID) == "null" || strings.TrimSpace(request.Method) == "" {
		return rpcRequest{}, errors.New("invalid JSON-RPC envelope")
	}
	if err := ensureEOF(decoder); err != nil {
		return rpcRequest{}, errors.New("invalid JSON-RPC request")
	}
	return request, nil
}

func dispatch(request rpcRequest, manifest Manifest, handler Handler, initialized *bool, granted *[]Scope) (result any, rpcErr *RPCError) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			rpcErr = &RPCError{Code: -32603, Message: "plugin handler failed"}
		}
	}()
	if request.Method == "initialize" {
		return initialize(request.Params, manifest, initialized, granted)
	}
	if !*initialized {
		return nil, &RPCError{Code: -32002, Message: "initialize must be called first"}
	}
	return dispatchInitialized(request, manifest, handler, *granted)
}

func initialize(raw json.RawMessage, manifest Manifest, initialized *bool, granted *[]Scope) (any, *RPCError) {
	if *initialized {
		return nil, &RPCError{Code: -32002, Message: "plugin is already initialized"}
	}
	var params InitializeParams
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}
	if params.ProtocolVersion != ProtocolVersion {
		return nil, &RPCError{Code: -32001, Message: fmt.Sprintf("protocol mismatch: plugin requires %s", ProtocolVersion)}
	}
	for _, scope := range params.GrantedScopes {
		if !slices.Contains(manifest.RequestedScopes, scope) {
			return nil, &RPCError{Code: -32003, Message: fmt.Sprintf("host granted undeclared scope %s", scope)}
		}
	}
	*initialized = true
	*granted = slices.Clone(params.GrantedScopes)
	return InitializeResult{ProtocolVersion: ProtocolVersion, Plugin: manifest, GrantedScopes: slices.Clone(*granted)}, nil
}

func dispatchInitialized(request rpcRequest, manifest Manifest, handler Handler, granted []Scope) (any, *RPCError) {
	switch request.Method {
	case "plugin.health":
		health := handler.Health()
		if health.Status != "healthy" && health.Status != "degraded" && health.Status != "unhealthy" {
			return nil, &RPCError{Code: -32603, Message: "plugin returned an invalid health status"}
		}
		if health.Checked.IsZero() {
			health.Checked = time.Now().UTC()
		}
		return health, nil
	case "project.inspect":
		if !slices.Contains(manifest.Capabilities, CapabilityProjectInspect) || !slices.Contains(granted, ScopeProjectMetadataRead) {
			return nil, &RPCError{Code: -32003, Message: "project.inspect capability was not granted"}
		}
		var params InspectRequest
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		if params.Project.Root != "" && !slices.Contains(granted, ScopeProjectFilesRead) {
			return nil, &RPCError{Code: -32003, Message: "project root requires project.files.read"}
		}
		return callInspect(handler, params)
	case "project.operate":
		if !slices.Contains(manifest.Capabilities, CapabilityProjectOperate) || !slices.Contains(granted, ScopeProjectOperate) {
			return nil, &RPCError{Code: -32003, Message: "project.operate capability was not granted"}
		}
		var params OperateRequest
		if err := decodeParams(request.Params, &params); err != nil {
			return nil, err
		}
		return callOperate(handler, params)
	default:
		return nil, &RPCError{Code: -32601, Message: "method not found"}
	}
}

func callInspect(handler Handler, request InspectRequest) (InspectResult, *RPCError) {
	result, err := handler.Inspect(request)
	if err != nil {
		return InspectResult{}, applicationError(err)
	}
	if result.Facts == nil {
		result.Facts = []Fact{}
	}
	if result.Actions == nil {
		result.Actions = []Action{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	if result.Observed.IsZero() {
		result.Observed = time.Now().UTC()
	}
	return result, nil
}

func callOperate(handler Handler, request OperateRequest) (OperateResult, *RPCError) {
	result, err := handler.Operate(request)
	if err != nil {
		return OperateResult{}, applicationError(err)
	}
	if result.Status != "succeeded" && result.Status != "partially_succeeded" && result.Status != "failed" {
		return OperateResult{}, &RPCError{Code: -32603, Message: "plugin returned an invalid operation status"}
	}
	if len(result.Output) == 0 {
		result.Output = json.RawMessage(`{}`)
	}
	return result, nil
}

func applicationError(err error) *RPCError {
	message := strings.TrimSpace(err.Error())
	if len(message) > 1024 {
		message = message[:1024]
	}
	if message == "" {
		message = "plugin operation failed"
	}
	return &RPCError{Code: -32000, Message: message}
}

func decodeParams(raw json.RawMessage, target any) *RPCError {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &RPCError{Code: -32602, Message: "invalid method parameters"}
	}
	if err := ensureEOF(decoder); err != nil {
		return &RPCError{Code: -32602, Message: "invalid method parameters"}
	}
	return nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	return err
}
