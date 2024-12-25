package gomcp

import (
	"context"
	"fmt"

	"github.com/llmcontext/gomcp/channels/hub"
	"github.com/llmcontext/gomcp/tools"
	"github.com/llmcontext/gomcp/types"
)

func NewModelContextProtocolServer(configFilePath string) (types.ModelContextProtocol, error) {
	mcp, err := hub.NewModelContextProtocolServer(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create model context protocol server: %v", err)
	}
	return mcp, nil
}

func GetLogger(ctx context.Context) types.Logger {
	return tools.GetLogger(ctx)
}
