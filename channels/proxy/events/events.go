package events

import (
	"github.com/hamstah/gomcp/jsonrpc"
	"github.com/hamstah/gomcp/protocol/mcp"
	"github.com/hamstah/gomcp/protocol/mux"
)

type Events interface {
	EventMcpStarted()
	EventMcpResponseInitialize(initializeResponse *mcp.JsonRpcResponseInitializeResult)
	EventMcpResponseToolsList(toolsListResponse *mcp.JsonRpcResponseToolsListResult)
	EventMcpResponseToolCall(toolsCallResult *mcp.JsonRpcResponseToolsCallResult, reqId *jsonrpc.JsonRpcRequestId)
	EventMcpResponseToolCallError(error *jsonrpc.JsonRpcError, reqId *jsonrpc.JsonRpcRequestId)
	EventMcpNotificationResourcesListChanged()
	EventMcpNotificationResourcesUpdated(resourcesUpdated *mcp.JsonRpcNotificationResourcesUpdatedParams)

	EventMuxStarted()
	EventMuxRequestToolCall(params *mux.JsonRpcRequestToolsCallParams, mcpReqId *jsonrpc.JsonRpcRequestId)

	EventMuxResponseProxyRegistered(registerResponse *mux.JsonRpcResponseProxyRegisterResult)
}
