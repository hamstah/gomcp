package proxymcpclient

import (
	"fmt"

	"github.com/hamstah/gomcp/protocol/mcp"
	"github.com/hamstah/gomcp/transport"
	"github.com/hamstah/gomcp/types"
)

func (c *ProxyMcpClient) handleIncomingMessage(message transport.JsonRpcMessage) error {
	if message.Response != nil {
		response := message.Response
		if response.Error != nil {
			c.logger.Error("error in response", types.LogArg{
				"method":        message.Method,
				"response":      fmt.Sprintf("%+v", response),
				"error_message": response.Error.Message,
				"error_code":    response.Error.Code,
				"error_data":    response.Error.Data,
			})

			switch message.Method {
			case mcp.RpcRequestMethodToolsCall:
				// we forward the response to the hubmux server
				c.events.EventMcpResponseToolCallError(response.Error, response.Id)
			}
			return nil
		}
		switch message.Method {
		case mcp.RpcRequestMethodInitialize:
			{
				initializeResponse, err := mcp.ParseJsonRpcResponseInitialize(response)
				if err != nil {
					c.logger.Error("error in handleMcpInitializeResponse", types.LogArg{
						"error": err,
					})
					return nil
				}
				c.logger.Info("init response", types.LogArg{
					"name":    initializeResponse.ServerInfo.Name,
					"version": initializeResponse.ServerInfo.Version,
				})
				c.events.EventMcpResponseInitialize(initializeResponse)
			}
		case mcp.RpcRequestMethodToolsList:
			{
				// the MCP server sent its tools list
				toolsListResponse, err := mcp.ParseJsonRpcResponseToolsList(response)
				if err != nil {
					c.logger.Error("error in handleMcpToolsListResponse", types.LogArg{
						"error": err,
					})
					return nil
				}

				c.events.EventMcpResponseToolsList(toolsListResponse)
			}
		case mcp.RpcRequestMethodToolsCall:
			{
				toolsCallResult, err := mcp.ParseJsonRpcResponseToolsCall(response)
				if err != nil {
					c.logger.Error("error parsing tools call params", types.LogArg{
						"error": err,
					})
					return nil
				}

				c.logger.Info("tools call result", types.LogArg{
					"content": toolsCallResult.Content,
					"isError": toolsCallResult.IsError,
				})

				// we forward the response to the hubmux server
				c.events.EventMcpResponseToolCall(toolsCallResult, response.Id)
			}

		default:
			c.logger.Error("received message with unexpected method", types.LogArg{
				"method": message.Method,
				"c":      "4cdu",
			})
		}
	} else if message.Request != nil {
		request := message.Request
		switch message.Method {
		case mcp.RpcNotificationMethodResourcesUpdated:
			{
				resourcesUpdated, err := mcp.ParseJsonRpcNotificationResourcesUpdatedParams(request.Params)
				if err != nil {
					c.logger.Error("error parsing resources updated", types.LogArg{
						"error": err,
					})
					return nil
				}
				c.events.EventMcpNotificationResourcesUpdated(resourcesUpdated)
			}
		case mcp.RpcNotificationMethodResourcesListChanged:
			{
				c.events.EventMcpNotificationResourcesListChanged()
			}
		default:
			c.logger.Error("received message with unexpected method", types.LogArg{
				"method":  message.Method,
				"request": request,
				"c":       "cjp1",
			})
		}
	} else {
		c.logger.Error("received message with unexpected nature", types.LogArg{
			"message": message,
		})
	}

	return nil
}
