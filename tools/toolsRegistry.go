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

type toolProviderPrepared struct {
	ToolProvider   *ToolProvider
	ToolDefinition *ToolDefinition
}

type ToolsRegistry struct {
	ToolProviders []*ToolProvider
	Tools         map[string]*toolProviderPrepared
	logger        types.Logger
}

func NewToolsRegistry(loadProxyTools bool, logger types.Logger) *ToolsRegistry {
	toolsRegistry := &ToolsRegistry{
		ToolProviders: []*ToolProvider{},
		Tools:         make(map[string]*toolProviderPrepared),
		logger:        logger,
	}
	// check if we need to load proxy tools
	if loadProxyTools {
		proxyTools := NewProxyTools()
		proxyTools.RegisterProxyTools(toolsRegistry)
	}
	return toolsRegistry
}

func (r *ToolsRegistry) RegisterToolProvider(toolProvider *ToolProvider) error {
	r.ToolProviders = append(r.ToolProviders, toolProvider)
	r.logger.Info("registered tool provider", types.LogArg{
		"tool":            toolProvider.toolName,
		"configTypeName":  toolProvider.configTypeName,
		"contextTypeName": toolProvider.contextTypeName,
		"proxyId":         toolProvider.proxyId,
	})
	return nil
}

func (r *ToolsRegistry) RegisterProxyToolProvider(proxyId string, proxyName string) (*ToolProvider, error) {
	// check if the proxy tool provider is already registered
	for _, toolProvider := range r.ToolProviders {
		if toolProvider.proxyId == proxyId {
			return toolProvider, nil
		}
	}

	provider, err := newProxyToolProvider(proxyId, proxyName)
	if err != nil {
		return nil, err
	}
	r.ToolProviders = append(r.ToolProviders, provider)
	return provider, nil
}

func (r *ToolsRegistry) PrepareProxyToolProvider(toolProvider *ToolProvider) error {
	for _, toolDefinition := range toolProvider.toolDefinitions {
		r.Tools[toolDefinition.ToolName] = &toolProviderPrepared{
			ToolProvider:   toolProvider,
			ToolDefinition: toolDefinition,
		}
	}
	return nil
}

func (r *ToolsRegistry) checkConfiguration(toolConfigs []config.ToolConfig) error {
	// we go through all the tool providers and check if the configuration is valid
	for _, toolProvider := range r.ToolProviders {
		// if the tool provider is a proxy, we don't need to check the configuration
		if toolProvider.proxyId != "" {
			continue
		}
		if toolProvider.configSchema != nil {
			r.logger.Info("checking config schema for tool provider", types.LogArg{
				"tool": toolProvider.toolName,
			})

			var toolConfigFound = false
			// we find the corresponding tool config
			for _, toolConfig := range toolConfigs {
				if toolConfig.Name == toolProvider.toolName {
					toolConfigFound = true
					if toolConfig.IsDisabled {
						// the tool is configured to be disabled
						r.logger.Info("tool is configured to be disabled", types.LogArg{
							"tool": toolProvider.toolName,
						})
						toolProvider.isDisabled = true
					} else {
						// we need to check if the configuration is valid
						err := utils.ValidateJsonSchemaWithObject(toolProvider.configSchema, toolConfig.Configuration)
						if err != nil {
							return err
						}
					}
				} else {
					return fmt.Errorf("tool config %s not found for tool provider %s", toolConfig.Name, toolProvider.toolName)
				}
			}
			if !toolConfigFound {
				return fmt.Errorf("tool config %s not found for tool provider %s", toolProvider.toolName, toolProvider.toolName)
			}
		} else {
			r.logger.Info("no config schema for tool provider", types.LogArg{
				"tool": toolProvider.toolName,
			})
		}
	}
	return nil
}

func (r *ToolsRegistry) initializeProviders(ctx context.Context, toolConfigs []config.ToolConfig) error {
	for _, toolProvider := range r.ToolProviders {
		if toolProvider.isDisabled {
			continue
		}

		// if the tool provider is a proxy, we don't need to initialize it
		if toolProvider.proxyId != "" {
			continue
		}

		// let's see if the tool provider has a configuration schema
		if toolProvider.configSchema != nil {
			// let's find the corresponding tool config
			for _, toolConfig := range toolConfigs {
				if toolConfig.Name == toolProvider.toolName {
					// we found the tool configuration
					// let's initialize the tool provider
					logger := types.NewSubLogger(r.logger, types.LogArg{
						"tool": toolProvider.toolName,
					})
					ctx := makeContextWithLogger(ctx, logger)
					result, callErr, err := utils.CallFunction(toolProvider.toolInitFunction, ctx, toolConfig.Configuration)
					if err != nil {
						return err
					}
					if callErr != nil {
						return callErr
					}
					logger.Info("tool provider initialized", types.LogArg{
						"result": result,
					})
					// we store the tool context
					toolProvider.toolContext = result
				}
			}
		}
	}
	return nil
}

func (r *ToolsRegistry) Prepare(ctx context.Context, toolConfigs []config.ToolConfig) error {
	// we check that the configuration for each tool provider is valid
	err := r.checkConfiguration(toolConfigs)
	if err != nil {
		return fmt.Errorf("error checking configuration: %w", err)
	}

	// let's prepare the different functions for each tool provider
	for _, toolProvider := range r.ToolProviders {
		if toolProvider.isDisabled {
			continue
		}
		// if the tool provider is a proxy, we don't need to prepare it
		// because it is already prepared by the proxy tools registry
		if toolProvider.proxyId != "" {
			continue
		}
		// for each tool definition, we prepare the function
		for _, toolDefinition := range toolProvider.toolDefinitions {
			// check that we don't already have a tool with this name
			if _, ok := r.Tools[toolDefinition.ToolName]; ok {
				return fmt.Errorf("tool %s already registered", toolDefinition.ToolName)
			}
			toolProviderPrepared := &toolProviderPrepared{
				ToolProvider:   toolProvider,
				ToolDefinition: toolDefinition,
			}
			r.Tools[toolDefinition.ToolName] = toolProviderPrepared
		}
	}

	// now, we can initialize the tool providers with their configuration
	err = r.initializeProviders(ctx, toolConfigs)
	if err != nil {
		return fmt.Errorf("error initializing tool providers: %w", err)
	}

	return nil
}

func (r *ToolsRegistry) GetListOfTools() []*ToolDefinition {
	tools := make([]*ToolDefinition, 0, len(r.Tools))
	for _, tool := range r.Tools {
		tools = append(tools, tool.ToolDefinition)
	}
	return tools
}

func (r *ToolsRegistry) getTool(toolName string) (*ToolDefinition, *ToolProvider, error) {
	tool, ok := r.Tools[toolName]
	if !ok {
		return nil, nil, fmt.Errorf("tool %s not found", toolName)
	}
	return tool.ToolDefinition, tool.ToolProvider, nil
}

func (r *ToolsRegistry) IsProxyTool(toolName string) (bool, string, error) {
	_, toolProvider, err := r.getTool(toolName)
	if err != nil {
		return false, "", err
	}
	return toolProvider.proxyId != "", toolProvider.proxyId, nil
}

func (r *ToolsRegistry) CallTool(ctx context.Context, toolName string, toolArgs map[string]interface{}) (interface{}, error) {
	toolDefinition, toolProvider, err := r.getTool(toolName)
	if err != nil {
		return nil, err
	}

	// let's check if the arguments patch the schema
	err = utils.ValidateJsonSchemaWithObject(toolDefinition.InputSchema, toolArgs)
	if err != nil {
		return nil, err
	}

	// let's call the tool
	logger := types.NewSubLogger(r.logger, types.LogArg{
		"tool": toolProvider.toolName,
	})
	goCtx := makeContextWithLogger(ctx, logger)

	// let's create the output
	output := NewToolCallResult()

	_, callErr, err := utils.CallFunction(toolDefinition.ToolHandlerFunction, goCtx, toolProvider.toolContext, toolArgs, output)
	if err != nil {
		return nil, err
	}
	if callErr != nil {
		return nil, callErr
	}

	return output, nil
}
