package server

import (
	"fmt"

	"github.com/spf13/viper"
)

type BaseServerConfig struct {
	ShutdownTimeout string `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`

	Log LogServerConfig `mapstructure:"log" yaml:"log"`
}

func LoadServerConfig() (*BaseServerConfig, error) {
	cfg := &BaseServerConfig{}

	setDefaults()

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return cfg, nil
}
