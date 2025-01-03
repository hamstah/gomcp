package mux

import (
	"fmt"

	"github.com/hamstah/gomcp/jsonrpc"
	"github.com/hamstah/gomcp/protocol"
)

// this is an event
const (
	RpcRequestMethodCallTool = "tools/call"
)

type JsonRpcRequestToolsCallParams struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

func ParseJsonRpcRequestToolsCallParams(request *jsonrpc.JsonRpcRequest) (*JsonRpcRequestToolsCallParams, error) {
	var err error
	// parse params
	if request.Params == nil {
		return nil, fmt.Errorf("missing params")
	}
	if !request.Params.IsNamed() {
		return nil, fmt.Errorf("params must be an object")
	}
	namedParams := request.Params.NamedParams

	req := JsonRpcRequestToolsCallParams{
		Name: "",
		Args: map[string]interface{}{},
	}

	// read method name
	req.Name, err = protocol.GetStringField(namedParams, "name")
	if err != nil {
		return nil, fmt.Errorf("missing name")
	}

	// read args
	args, err := protocol.GetObjectField(namedParams, "args")
	if err != nil {
		return nil, fmt.Errorf("missing args")
	}
	for key, value := range args {
		req.Args[key] = value
	}

	return &req, nil
}
