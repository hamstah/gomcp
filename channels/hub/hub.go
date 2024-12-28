package hub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/hamstah/gomcp/channels/hub/events"
	"github.com/hamstah/gomcp/channels/hubmcpserver"
	"github.com/hamstah/gomcp/config"
	"github.com/hamstah/gomcp/logger"
	"github.com/hamstah/gomcp/prompts"
	"github.com/hamstah/gomcp/tools"
	"github.com/hamstah/gomcp/transport"
	"github.com/hamstah/gomcp/types"
	"golang.org/x/sync/errgroup"
)

type ModelContextProtocolImpl struct {
	logging         *config.LoggingInfo
	toolsRegistry   *tools.ToolsRegistry
	promptsRegistry *prompts.PromptsRegistry
	tools           []config.ToolConfig
	stateManager    *StateManager
	events          events.Events
	logger          types.Logger
}

func newModelContextProtocolServer(
	serverInfo *config.ServerInfo,
	logging *config.LoggingInfo,
	promptsConfig *config.PromptConfig,
	toolsConfig []config.ToolConfig,
) (*ModelContextProtocolImpl, error) {
	// we initialize the logger
	logger, err := logger.NewLogger(logging, false)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %v", err)
	}

	// Initialize tools registry
	toolsRegistry := tools.NewToolsRegistry(logger)

	// Initialize prompts registry
	promptsRegistry := prompts.NewEmptyPromptsRegistry()
	if promptsConfig != nil {
		promptsRegistry, err = prompts.NewPromptsRegistry(promptsConfig.File)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize prompts registry: %v", err)
		}
	}

	// initialize the state manager
	stateManager := NewStateManager(
		serverInfo.Name,
		serverInfo.Version,
		toolsRegistry,
		promptsRegistry,
		logger,
	)
	events := stateManager.AsEvents()

	return &ModelContextProtocolImpl{
		logging:         logging,
		toolsRegistry:   toolsRegistry,
		promptsRegistry: promptsRegistry,

		stateManager: stateManager,
		tools:        toolsConfig,
		events:       events,
		logger:       logger,
	}, nil

}

func NewModelContextProtocolServer(configFilePath string) (*ModelContextProtocolImpl, error) {
	// we load the conf file
	conf, err := config.LoadServerConfig(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file %s: %v", configFilePath, err)
	}

	return newModelContextProtocolServer(
		&conf.ServerInfo,
		conf.Logging,
		conf.Prompts,
		conf.Tools,
	)

}

func (mcp *ModelContextProtocolImpl) StdioTransport() types.Transport {
	// delete the protocol debug file if it exists
	if mcp.logging.ProtocolDebugFile != "" {
		if _, err := os.Stat(mcp.logging.ProtocolDebugFile); err == nil {
			os.Remove(mcp.logging.ProtocolDebugFile)
		}
	}

	// we create the transport
	transport := transport.NewStdioTransport(
		mcp.logging.ProtocolDebugFile,
		mcp.logger,
	)

	// we return the transport
	return transport
}

// Start starts the server and the inspector
func (mcp *ModelContextProtocolImpl) Start(transport types.Transport) error {
	mcp.logger.Info("Starting MCP server", types.LogArg{})

	// create a context that will be used to cancel the server and the inspector
	ctx := context.Background()

	// All the tools are initialized, we can prepare the tools registry
	// so that it can be used by the server

	toolsConfig := make(map[string]config.ToolConfig)
	for _, tool := range mcp.tools {
		toolsConfig[tool.Name] = tool
	}

	// we prepare the tools registry
	err := mcp.toolsRegistry.Prepare(ctx, toolsConfig)
	if err != nil {
		return fmt.Errorf("error preparing tools registry: %s", err)
	}

	mcp.logger.Info("Starting inspector", types.LogArg{})

	// we create an errgroup that will be used to cancel
	// all the components of the server
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		mcp.logger.Info("[A] for MCP server to stop", types.LogArg{})

		// Listen for OS signals (e.g., Ctrl+C)
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGABRT, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGCHLD)

		select {
		case <-egCtx.Done():
			mcp.logger.Info("[A.1] Context cancelled, shutting down", types.LogArg{})
			return egCtx.Err()
		case signal := <-signalChan:
			mcp.logger.Info("[A.2] Received an interrupt, shutting down", types.LogArg{
				"signal": signal.String(),
			})
			return fmt.Errorf("received an interrupt (%s)", signal.String())
		}
	})

	eg.Go(func() error {
		mcp.logger.Info("[C] Starting MCP server", types.LogArg{})

		// Initialize server
		server := hubmcpserver.NewMCPServer(transport,
			mcp.events,
			mcp.logger)

		// set the server in the state manager
		mcp.stateManager.SetMcpServer(server)

		// Start server
		err := server.Start(egCtx)
		if err != nil {
			// check if the error is because the context was cancelled
			if errors.Is(err, context.Canceled) {
				mcp.logger.Info("[C.1] context cancelled, stopping MCP server", types.LogArg{})
			} else {
				mcp.logger.Error("[C.2] error starting MCP server", types.LogArg{
					"error": err,
				})
			}
		}
		mcp.logger.Info("[C.3] MCP server stopped", types.LogArg{})
		return err
	})

	if false {
		eg.Go(func() error {
			count := 0
			timer := time.NewTimer(10 * time.Second)
			defer timer.Stop()

			for {
				count++
				parentPID := syscall.Getppid()
				mcp.logger.Info("Monitoring parent process", types.LogArg{
					"pid":   parentPID,
					"count": count,
				})
				if parentPID == 1 {
					mcp.logger.Info("Parent process is init. Shutting down...", types.LogArg{
						"pid": parentPID,
					})
					return fmt.Errorf("parent process is init")
				}
				logGoroutineStacks(mcp.logger)

				if count > 10 {
					mcp.logger.Info("Stopping parent process monitor", types.LogArg{})
					return nil
				}

				timer.Reset(10 * time.Second)
				select {
				case <-egCtx.Done():
					mcp.logger.Info("Context cancelled, stopping parent process monitor", types.LogArg{})
					return egCtx.Err()
				case <-timer.C:
					continue
				}
			}
		})
	}

	err = eg.Wait()
	if err != nil {
		mcp.logger.Info("Stopping hub server", types.LogArg{
			"reason": err.Error(),
		})
	}
	mcp.logger.Info("Hub server stopped", types.LogArg{})
	return nil
}

func (mcp *ModelContextProtocolImpl) GetToolRegistry() types.ToolRegistry {
	return mcp.toolsRegistry
}

func logGoroutineStacks(logger types.Logger) {
	// Get number of goroutines
	numGoroutines := runtime.NumGoroutine()

	// Get stack traces
	buf := make([]byte, 1024*10)
	n := runtime.Stack(buf, true)
	stacks := string(buf[:n])

	logger.Info("Goroutine dump", types.LogArg{
		"num_goroutines": numGoroutines,
	})

	// print the goroutine stacks formatted
	fmt.Println("Goroutine stacks:")
	fmt.Println(stacks)
}
