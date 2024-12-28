package hub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hamstah/gomcp/channels/hub/events"
	"github.com/hamstah/gomcp/channels/hubmcpserver"
	"github.com/hamstah/gomcp/jsonrpc"
	"github.com/hamstah/gomcp/prompts"
	"github.com/hamstah/gomcp/protocol/mcp"
	"github.com/hamstah/gomcp/tools"
	"github.com/hamstah/gomcp/types"
)

// MCP client information (eg Claude)
type ClientInfo struct {
	name    string
	version string
}

type StateManager struct {
	// mcp related state
	serverName          string
	serverVersion       string
	clientInfo          *ClientInfo
	isClientInitialized bool
	toolsRegistry       *tools.ToolsRegistry
	promptsRegistry     *prompts.PromptsRegistry

	logger       types.Logger
	mcpServer    *hubmcpserver.MCPServer
	reqIdMapping *jsonrpc.ReqIdMapping
}

func NewStateManager(
	serverName string,
	serverVersion string,
	toolsRegistry *tools.ToolsRegistry,
	promptsRegistry *prompts.PromptsRegistry,
	logger types.Logger,
) *StateManager {
	return &StateManager{
		serverName:          serverName,
		serverVersion:       serverVersion,
		isClientInitialized: false,
		toolsRegistry:       toolsRegistry,
		promptsRegistry:     promptsRegistry,
		logger:              logger,
		reqIdMapping:        jsonrpc.NewReqIdMapping(),
	}
}

func (s *StateManager) SetMcpServer(server *hubmcpserver.MCPServer) {
	s.mcpServer = server
}

func (s *StateManager) AsEvents() events.Events {
	return s
}

func (s *StateManager) EventMcpRequestInitialize(params *mcp.JsonRpcRequestInitializeParams, reqId *jsonrpc.JsonRpcRequestId) {
	// store client information
	if params.ProtocolVersion != mcp.ProtocolVersion {
		s.logger.Error("protocol version mismatch", types.LogArg{
			"expected": mcp.ProtocolVersion,
			"received": params.ProtocolVersion,
		})
	}
	s.clientInfo = &ClientInfo{
		name:    params.ClientInfo.Name,
		version: params.ClientInfo.Version,
	}

	// prepare response
	response := mcp.JsonRpcResponseInitializeResult{
		ProtocolVersion: mcp.ProtocolVersion,
		Capabilities: mcp.ServerCapabilities{
			Tools: &mcp.ServerCapabilitiesTools{
				ListChanged: jsonrpc.BoolPtr(true),
			},
			Prompts: &mcp.ServerCapabilitiesPrompts{
				ListChanged: jsonrpc.BoolPtr(true),
			},
		},
		ServerInfo: mcp.ServerInfo{Name: s.serverName, Version: s.serverVersion},
	}
	s.mcpServer.SendJsonRpcResponse(&response, reqId)

}

func (s *StateManager) EventMcpNotificationInitialized() {
	// that's a notification, no response is needed
	s.isClientInitialized = true
}

func (s *StateManager) EventMcpRequestToolsList(params *mcp.JsonRpcRequestToolsListParams, reqId *jsonrpc.JsonRpcRequestId) {
	var response = mcp.JsonRpcResponseToolsListResult{
		Tools: make([]mcp.ToolDescription, 0, len(s.toolsRegistry.Tools)),
	}

	// we build the response
	for _, tool := range s.toolsRegistry.Tools {

		response.Tools = append(response.Tools, mcp.ToolDescription{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}

	s.mcpServer.SendJsonRpcResponse(&response, reqId)
}

func (s *StateManager) EventMcpRequestToolsCall(ctx context.Context, params *mcp.JsonRpcRequestToolsCallParams, reqId *jsonrpc.JsonRpcRequestId) {
	// we get the tool name and arguments
	toolName := params.Name
	toolArgs := params.Arguments

	response, err := s.toolsRegistry.CallTool(ctx, toolName, toolArgs)
	if err != nil {
		s.mcpServer.SendError(jsonrpc.RpcInternalError, fmt.Sprintf("tool call failed: %v", err), reqId)
		return
	}
	s.mcpServer.SendJsonRpcResponse(&response, reqId)

}

func (s *StateManager) EventMcpRequestResourcesList(params *mcp.JsonRpcRequestResourcesListParams, reqId *jsonrpc.JsonRpcRequestId) {
	var response = mcp.JsonRpcResponseResourcesListResult{
		Resources: make([]mcp.ResourceDescription, 0),
	}

	s.mcpServer.SendJsonRpcResponse(&response, reqId)
}

func (s *StateManager) EventMcpRequestPromptsList(params *mcp.JsonRpcRequestPromptsListParams, reqId *jsonrpc.JsonRpcRequestId) {
	var response = mcp.JsonRpcResponsePromptsListResult{
		Prompts: make([]mcp.PromptDescription, 0),
	}

	prompts := s.promptsRegistry.GetListOfPrompts()
	for _, prompt := range prompts {
		arguments := make([]mcp.PromptArgumentDescription, 0, len(prompt.Arguments))
		for _, argument := range prompt.Arguments {
			arguments = append(arguments, mcp.PromptArgumentDescription{
				Name:        argument.Name,
				Description: argument.Description,
				Required:    argument.Required,
			})
		}
		response.Prompts = append(response.Prompts, mcp.PromptDescription{
			Name:        prompt.Name,
			Description: prompt.Description,
			Arguments:   arguments,
		})
	}

	s.mcpServer.SendJsonRpcResponse(&response, reqId)
}

func (s *StateManager) EventMcpRequestPromptsGet(params *mcp.JsonRpcRequestPromptsGetParams, reqId *jsonrpc.JsonRpcRequestId) {
	var templateArgs = map[string]string{}
	// copy the arguments, as strings
	for key, value := range params.Arguments {
		templateArgs[key] = fmt.Sprintf("%v", value)
	}
	promptName := params.Name

	response, err := s.promptsRegistry.GetPrompt(promptName, templateArgs)
	if err != nil {
		s.mcpServer.SendError(jsonrpc.RpcInvalidParams, fmt.Sprintf("prompt processing error: %s", err), reqId)
		return
	}

	// marshal response
	responseBytes, err := json.Marshal(response)
	if err != nil {
		s.mcpServer.SendError(jsonrpc.RpcInternalError, "failed to marshal response", reqId)
	}
	jsonResponse := json.RawMessage(responseBytes)

	// we send the response
	s.mcpServer.SendJsonRpcResponse(&jsonResponse, reqId)
}

func (s *StateManager) EventNewProxyTools() {
	// s.mcpServer.OnNewProxyTools()
}

func (s *StateManager) EventMcpError(code int, message string, data *json.RawMessage, id *jsonrpc.JsonRpcRequestId) {
	s.mcpServer.SendError(code, message, id)
}
