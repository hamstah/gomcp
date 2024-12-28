package types

import (
	"context"

	"github.com/invopop/jsonschema"
)

type ToolDefinition struct {
	ToolName            string
	ToolHandlerFunction interface{}
	Description         string
	InputSchema         *jsonschema.Schema
	InputTypeName       string
	// for a tool to be available from a proxy, we need to set the ToolProxyId
	ToolProxyId string
}
type ToolDefinitionsFunction func(ctx context.Context, toolContext interface{}) ([]*ToolDefinition, error)

type ToolProvider interface {
	AddTool(toolName string, description string, toolHandler interface{}) error
	SetToolDefinitionsFunction(toolDefinitionsFunction ToolDefinitionsFunction) error
}

type ToolRegistry interface {
	DeclareToolProvider(toolName string, toolInitFunction interface{}) (ToolProvider, error)
}

type ModelContextProtocol interface {
	StdioTransport() Transport
	GetToolRegistry() ToolRegistry
	Start(transport Transport) error
}
