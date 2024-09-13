package util

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// configType defines the types of supported config files
type configType string

const (
	ConfigJSON configType = "json"
	ConfigYAML configType = "yaml"
)

// AppConfig holds the necessary details for loading the config
type AppConfig struct {
	FilePath string
	FileName string
	Type     configType
}

// LoadConfig loads a JSON or YAML config file from the specified path
func LoadConfig(config *AppConfig, target interface{}) error {
	// Enable environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Detect the environment: default to "docker" if ENVIRONMENT is not set
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "docker"
	}

	// Adjust the config file name based on the environment
	config.FileName = fmt.Sprintf("%s.%s.config", config.FileName, env)
	log.Println("Current config file:", config.FileName)

	// Set the config file details
	viper.AddConfigPath(config.FilePath)
	viper.SetConfigName(config.FileName)
	viper.SetConfigType(string(config.Type))

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	// Unmarshal the config into the target struct
	if err := viper.Unmarshal(target); err != nil {
		return err
	}

	return nil
}
