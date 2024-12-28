package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hamstah/gomcp/config"
	"github.com/hamstah/gomcp/types"
	"github.com/hamstah/gomcp/utils"
)

type ToolRpcHandler func(input json.RawMessage) (json.RawMessage, error)

type ToolsRegistry struct {
	ToolProviders map[string]types.ToolProvider
	Tools         map[string]types.Tool
	logger        types.Logger
}

func NewToolsRegistry(logger types.Logger) *ToolsRegistry {
	toolsRegistry := &ToolsRegistry{
		ToolProviders: make(map[string]types.ToolProvider),
		Tools:         make(map[string]types.Tool),
		logger:        logger,
	}
	return toolsRegistry
}

func (r *ToolsRegistry) AddToolProvider(toolProvider types.ToolProvider) error {
	r.ToolProviders[toolProvider.Name()] = toolProvider

	r.logger.Info("registered tool provider", types.LogArg{
		"name": toolProvider.Name(),
	})
	return nil
}

func (r *ToolsRegistry) initializeProviders(ctx context.Context, toolConfigs map[string]config.ToolConfig) error {
	for _, toolProvider := range r.ToolProviders {

		config, ok := toolConfigs[toolProvider.Name()]
		if !ok {
			continue
		}

		err := toolProvider.Init(ctx, config.Configuration)
		if err != nil {
			return fmt.Errorf("error initializing tool provider %s: %w", toolProvider.Name(), err)
		}
	}
	return nil
}

func (r *ToolsRegistry) Prepare(ctx context.Context, toolConfigs map[string]config.ToolConfig) error {
	ctx = makeContextWithLogger(ctx, r.logger)

	// initialize the tool providers with their configuration
	err := r.initializeProviders(ctx, toolConfigs)
	if err != nil {
		return fmt.Errorf("error initializing tool providers: %w", err)
	}

	// let's prepare the different functions for each tool provider
	for _, toolProvider := range r.ToolProviders {

		tools, err := toolProvider.ListTools(ctx)
		if err != nil {
			return fmt.Errorf("error getting tool definitions: %w", err)
		}

		for _, tool := range tools {
			r.Tools[tool.Name()] = tool
		}
	}

	return nil
}

func (r *ToolsRegistry) CallTool(ctx context.Context, toolName string, toolArgs map[string]interface{}) (interface{}, error) {
	tool, ok := r.Tools[toolName]
	if !ok {
		return nil, fmt.Errorf("tool %s not found", toolName)
	}

	// let's check if the arguments patch the schema
	err := utils.ValidateJsonSchemaWithObject(tool.InputSchema(), toolArgs)
	if err != nil {
		return nil, err
	}

	// let's call the tool
	logger := types.NewSubLogger(r.logger, types.LogArg{
		"tool": tool.Name(),
	})
	goCtx := makeContextWithLogger(ctx, logger)

	result, err := tool.Call(goCtx, toolArgs)
	if err != nil {
		return nil, err
	}

	return result, nil
}
