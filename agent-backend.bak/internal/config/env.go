package config

import (
	"fmt"

	"github.com/spf13/viper"
)

const defaultEnvFile = ".env"

// NewViperFromEnv initializes Viper by always reading .env and enabling
// environment variable overrides.
func NewViperFromEnv() (*viper.Viper, error) {
	cfg := viper.New()
	cfg.AutomaticEnv()
	cfg.SetConfigFile(defaultEnvFile)
	cfg.SetConfigType("env")
	if err := cfg.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %q: %w", defaultEnvFile, err)
	}

	return cfg, nil
}

func MustViper() *viper.Viper {
	cfg, err := NewViperFromEnv()
	if err != nil {
		panic(err)
	}
	return cfg
}
