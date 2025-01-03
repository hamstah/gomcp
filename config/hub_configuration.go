package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hamstah/gomcp/defaults"
	"github.com/hamstah/gomcp/utils"
	"github.com/invopop/jsonschema"
)

type HubConfiguration struct {
	ConfigVersion int                `json:"v"`
	ServerInfo    ServerInfo         `json:"serverInfo"`
	Logging       *LoggingInfo       `json:"logging,omitempty"`
	Inspector     *InspectorInfo     `json:"inspector,omitempty"`
	Prompts       *PromptConfig      `json:"prompts,omitempty"`
	Proxy         *ServerProxyConfig `json:"proxy,omitempty"`
}

type ServerProxyConfig struct {
	Enabled       bool   `json:"enabled"`
	ListenAddress string `json:"listenAddress"`
}

var defaultHubConfigurationPath = filepath.Join(defaults.DefaultHubConfigurationDirectory, "hub.json")

func GetDefaultHubConfigurationPath() string {
	return defaultHubConfigurationPath
}

// LoadConfig loads the configuration from a file
func LoadHubConfiguration() (*HubConfiguration, error) {
	configFilePath := defaultHubConfigurationPath

	// Check if the file exists
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("hub configuration file does not exist: %s", configFilePath)
	}

	// let's generate the schema from the config struct
	configSchema := jsonschema.Reflect(&HubConfiguration{})
	if configSchema == nil {
		return nil, fmt.Errorf("failed to generate schema from config struct")
	}
	// let's check that the file is a valid json file
	jsonBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	err = utils.ValidateJsonSchemaWithBytes(configSchema, jsonBytes)
	if err != nil {
		return nil, err
	}

	var config HubConfiguration
	err = json.Unmarshal(jsonBytes, &config)
	if err != nil {
		return nil, err
	}

	// update the file path to be absolute
	if config.Logging != nil {
		config.Logging.UpdateFilePaths()
	}

	return &config, nil
}
