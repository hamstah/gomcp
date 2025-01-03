package proxy

import (
	"github.com/hamstah/gomcp/channels/proxy/events"
	"github.com/hamstah/gomcp/channels/proxymcpclient"
	"github.com/hamstah/gomcp/channels/proxymuxclient"
	"github.com/hamstah/gomcp/jsonrpc"
	"github.com/hamstah/gomcp/protocol/mcp"
	"github.com/hamstah/gomcp/protocol/mux"
	"github.com/hamstah/gomcp/tools"
	"github.com/hamstah/gomcp/transport"
	"github.com/hamstah/gomcp/types"
	"github.com/hamstah/gomcp/version"
)

type StateManager struct {
	logger    types.Logger
	options   *transport.ProxiedMcpServerDescription
	muxClient *proxymuxclient.ProxyMuxClient
	mcpClient *proxymcpclient.ProxyMcpClient
	registry  *tools.ProxyToolsRegistry
	// serverInfo is the info about the MCP server we are connected to
	serverInfo   mcp.ServerInfo
	reqIdMapping *jsonrpc.ReqIdMapping
}

func NewStateManager(options *transport.ProxiedMcpServerDescription,
	registry *tools.ProxyToolsRegistry, logger types.Logger) *StateManager {
	return &StateManager{
		options:      options,
		logger:       logger,
		serverInfo:   mcp.ServerInfo{},
		reqIdMapping: jsonrpc.NewReqIdMapping(),
		registry:     registry,
	}
}

func (s *StateManager) Stop(err error) {
	s.logger.Info("stopping state manager", types.LogArg{"error": err})
}

func (s *StateManager) SetMuxClient(muxClient *proxymuxclient.ProxyMuxClient) {
	s.muxClient = muxClient
}

func (s *StateManager) SetProxyClient(proxyClient *proxymcpclient.ProxyMcpClient) {
	s.mcpClient = proxyClient
}

func (s *StateManager) AsEvents() events.Events {
	return s
}

func (s *StateManager) EventMcpStarted() {
	// as soon as the MCP server is started, we send an initialize request
	// to the MCP server
	params := mcp.JsonRpcRequestInitializeParams{
		ProtocolVersion: mcp.ProtocolVersion,
		Capabilities:    mcp.ClientCapabilities{},
		ClientInfo: mcp.ClientInfo{
			Name:    s.options.ProxyName,
			Version: version.Version,
		},
	}
	s.mcpClient.SendRequestWithMethodAndParams(mcp.RpcRequestMethodInitialize, params)

}

func (s *StateManager) EventMcpResponseInitialize(resp *mcp.JsonRpcResponseInitializeResult) {
	s.logger.Debug("MCP Server initialized", types.LogArg{
		"name":    resp.ServerInfo.Name,
		"version": resp.ServerInfo.Version,
	})

	// we update the server information
	s.serverInfo.Name = resp.ServerInfo.Name
	s.serverInfo.Version = resp.ServerInfo.Version

	// we send the "notifications/initialized" notification
	s.mcpClient.SendNotification(mcp.RpcNotificationMethodInitialized)

	// we send the "tools/list" request
	s.mcpClient.SendRequestWithMethodAndParams(
		mcp.RpcRequestMethodToolsList, mcp.JsonRpcRequestToolsListParams{})
}

func (s *StateManager) EventMcpResponseToolsList(resp *mcp.JsonRpcResponseToolsListResult) {
	s.logger.Info("event mcp tools list response", types.LogArg{
		"tools": resp.Tools,
	})

	// we register the tools in the registry
	proxyToolsDefinition := tools.ProxyDefinition{
		ProxyId:          s.options.ProxyId,
		WorkingDirectory: s.options.CurrentWorkingDirectory,
		ProxyName:        s.options.ProxyName,
		ProgramName:      s.options.ProgramName,
		ProgramArguments: s.options.ProgramArgs,
		Tools:            []tools.ProxyToolDefinition{},
	}
	for _, tool := range resp.Tools {
		proxyToolsDefinition.Tools = append(proxyToolsDefinition.Tools, tools.ProxyToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	err := s.registry.AddProxyDefinition(&proxyToolsDefinition)
	if err != nil {
		s.logger.Error("failed to add proxy definition", types.LogArg{"error": err})
	}

	// update on 2024/12/25 , we don't need send the tools/register
	// to the server as the tools are already registered in the registry
	// // we send the "tools/register" request to the mux server
	// toolsMux := make([]mux.ToolDescription, len(resp.Tools))
	// for i, tool := range resp.Tools {
	// 	toolsMux[i] = mux.ToolDescription{
	// 		Name:        tool.Name,
	// 		Description: tool.Description,
	// 		InputSchema: tool.InputSchema,
	// 	}
	// }
	// params := mux.JsonRpcRequestToolsRegisterParams{
	// 	Tools: toolsMux,
	// }
	// s.muxClient.SendRequestWithMethodAndParams(mux.RpcRequestMethodToolsRegister, params)
}

func (s *StateManager) EventMuxStarted() {
	s.logger.Debug("Mux Server started", types.LogArg{})
	params := mux.JsonRpcRequestProxyRegisterParams{
		ProtocolVersion: mux.MuxProtocolVersion,
		ProxyId:         s.options.ProxyId,
		Proxy: mux.ProxyDescription{
			WorkingDirectory: s.options.CurrentWorkingDirectory,
			Command:          s.options.ProgramName,
			Args:             s.options.ProgramArgs,
		},
		ServerInfo: mux.ServerInfo{
			Name:    s.serverInfo.Name,
			Version: s.serverInfo.Version,
		},
	}

	// we register the proxy to the mux server
	s.muxClient.SendRequestWithMethodAndParams(mux.RpcRequestMethodProxyRegister, params)
}

func (s *StateManager) EventMuxResponseProxyRegistered(registerResponse *mux.JsonRpcResponseProxyRegisterResult) {
	s.logger.Info("event mux proxy registered", types.LogArg{
		"sessionId":  registerResponse.SessionId,
		"proxyId":    registerResponse.ProxyId,
		"persistent": registerResponse.Persistent,
		"denied":     registerResponse.Denied,
	})

}

// this is a tool call from the hub
func (s *StateManager) EventMuxRequestToolCall(params *mux.JsonRpcRequestToolsCallParams, reqId *jsonrpc.JsonRpcRequestId) {
	s.logger.Info("EventMuxRequestToolCall", types.LogArg{
		"name": params.Name,
		"args": params.Args,
	})

	req := mcp.JsonRpcRequestToolsCallParams{
		Name:      params.Name,
		Arguments: params.Args,
	}

	// we forward the tool call to the mcp client
	mcpReqId, err := s.mcpClient.SendRequestWithMethodAndParams(mcp.RpcRequestMethodToolsCall, req)
	if err != nil {
		s.logger.Error("failed to send request to mcp client", types.LogArg{"error": err})
		return
	}
	// we keep track of the mapping between the mcp request id
	// and the mux request id
	s.reqIdMapping.AddMapping(mcpReqId, reqId)
}

// got the response for the tool call from the mcp client
func (s *StateManager) EventMcpResponseToolCall(toolsCallResult *mcp.JsonRpcResponseToolsCallResult, reqId *jsonrpc.JsonRpcRequestId) {
	s.logger.Info("event mcp tool call response", types.LogArg{
		"content": toolsCallResult.Content,
		"isError": toolsCallResult.IsError,
	})
	//s.muxClient.SendToolCallResponse(toolsCallResult, reqId, mcpReqId)
	params := mux.JsonRpcResponseToolsCallResult{
		Content: toolsCallResult.Content,
		IsError: toolsCallResult.IsError,
	}
	// we parse the req id is the one coming from the hub
	// and we send the response to the hub with that id
	muxReqId := s.reqIdMapping.GetMapping(reqId)
	s.muxClient.SendJsonRpcResponse(params, muxReqId)
}

func (s *StateManager) EventMcpResponseToolCallError(error *jsonrpc.JsonRpcError, reqId *jsonrpc.JsonRpcRequestId) {
	// we parse the req id is the one coming from the hub
	// and we send the response to the hub with that id
	muxReqId := s.reqIdMapping.GetMapping(reqId)
	s.muxClient.SendError(error.Code, error.Message, muxReqId)
}

func (s *StateManager) EventMcpNotificationResourcesListChanged() {
	s.logger.Info("event mcp notification resources list changed", types.LogArg{})
}

func (s *StateManager) EventMcpNotificationResourcesUpdated(resourcesUpdated *mcp.JsonRpcNotificationResourcesUpdatedParams) {
	s.logger.Info("event mcp notification resources updated", types.LogArg{
		"uri": resourcesUpdated.Uri,
	})
}
