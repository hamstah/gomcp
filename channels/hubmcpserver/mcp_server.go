package hubmcpserver

import (
	"context"
	"errors"

	"github.com/hamstah/gomcp/channels/hub/events"
	"github.com/hamstah/gomcp/jsonrpc"
	"github.com/hamstah/gomcp/transport"
	"github.com/hamstah/gomcp/types"
)

type MCPServer struct {
	transport *transport.JsonRpcTransport
	events    events.Events
	logger    types.Logger
}

func NewMCPServer(
	tran types.Transport,
	events events.Events,
	logger types.Logger,
) *MCPServer {
	jsonRpcTransport := transport.NewJsonRpcTransport(tran, "mcp server", logger)
	return &MCPServer{
		transport: jsonRpcTransport,
		events:    events,
		logger:    logger,
	}
}

func (s *MCPServer) Start(ctx context.Context) error {
	var err error

	errChan := make(chan error, 1)

	go func() {
		// Start the transport
		err := s.transport.Start(ctx, func(message transport.JsonRpcMessage, jsonRpcTransport *transport.JsonRpcTransport) {
			err = s.handleIncomingMessage(ctx, message)
			if err != nil {
				s.logError("failed to handle incoming message", err)
			}
		})
		if err != nil {
			s.logError("failed to start transport", err)
		}
		errChan <- err
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		s.transport.Close()
		return ctx.Err()
	}
}

func (s *MCPServer) Close() {
	s.transport.Close()
}

func (s *MCPServer) logError(message string, err error) {
	// check if the error is because the context was cancelled
	if errors.Is(err, context.Canceled) {
		s.logger.Info("transport - context cancelled", types.LogArg{})
	} else {
		s.logger.Error(message, types.LogArg{
			"message": message,
			"error":   err,
		})
	}
}

func (s *MCPServer) SendRequestWithMethodAndParams(method string, params interface{}) (*jsonrpc.JsonRpcRequestId, error) {
	return s.transport.SendRequestWithMethodAndParams(method, params)
}

func (s *MCPServer) SendJsonRpcResponse(response interface{}, id *jsonrpc.JsonRpcRequestId) {
	s.transport.SendResponse(&jsonrpc.JsonRpcResponse{
		JsonRpcVersion: jsonrpc.JsonRpcVersion,
		Id:             id,
		Result:         response,
		Error:          nil,
	})
}

func (s *MCPServer) SendError(code int, message string, id *jsonrpc.JsonRpcRequestId) {
	s.logger.Debug("JsonRpcError", types.LogArg{
		"code":    code,
		"message": message,
		"id":      id,
	})
	err := s.transport.SendError(code, message, id)
	if err != nil {
		s.logError("failed to send error", err)
	}
}

func (c *MCPServer) SendNotification(method string) {
	notification := jsonrpc.JsonRpcRequest{
		JsonRpcVersion: jsonrpc.JsonRpcVersion,
		Method:         method,
	}
	c.transport.SendRequest(&notification)
}
