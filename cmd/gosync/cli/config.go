package cli

import (
	"fmt"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func initConfig(path string) error {
	// Load .env files from current directory
	envFiles := []string{".env", ".env.local"}
	for _, envFile := range envFiles {
		if err := godotenv.Load(envFile); err != nil {
			// Silently ignore missing .env files
			continue
		}
	}

	if path != "" {
		viper.SetConfigFile(path)
		// Also try to load .env file from the same directory as config
		configDir := filepath.Dir(path)
		for _, envFile := range envFiles {
			envPath := filepath.Join(configDir, envFile)
			godotenv.Load(envPath) // Ignore errors
		}
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/gosync")
		viper.AddConfigPath("$HOME/.gosync")

		// Load .env files from config directories
		configPaths := []string{".", "./config", "/etc/gosync", "$HOME/.gosync"}
		for _, configPath := range configPaths {
			for _, envFile := range envFiles {
				envPath := filepath.Join(configPath, envFile)
				godotenv.Load(envPath) // Ignore errors
			}
		}
	}

	viper.SetEnvPrefix("GOSYNC")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	return nil
}
