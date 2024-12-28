package types

import (
	"context"

	"github.com/invopop/jsonschema"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() *jsonschema.Schema
	Call(ctx context.Context, args any) (ToolCallResult, error)
}

type ToolProvider interface {
	Name() string
	Init(ctx context.Context, toolContext any) error
	ListTools(ctx context.Context) ([]Tool, error)
	GetTool(ctx context.Context, toolName string) (Tool, error)
}

type ToolRegistry interface {
	AddToolProvider(toolProvider ToolProvider) error
}

type ModelContextProtocol interface {
	StdioTransport() Transport
	GetToolRegistry() ToolRegistry
	Start(transport Transport) error
}
