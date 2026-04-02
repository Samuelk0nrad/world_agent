package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Env struct {
	GeminiAPIKey   string `mapstructure:"GEMINI_API_KEY"`
	PromptPathSys  string `mapstructure:"PROMPT_PATH_SYS"`
	PromptPathTool string `mapstructure:"PROMPT_PATH_TOOL"`
	Host           string `mapstructure:"HOST"`
	Port           string `mapstructure:"PORT"`
}

func NewEnv(filename string, override bool) (*Env, error) {
	env := Env{}

	viper.SetConfigFile(filename)

	// load system env
	if override {
		viper.AutomaticEnv()
	}

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading environment file: %w", err)
	}

	err = viper.Unmarshal(&env)
	if err != nil {
		log.Fatal("Error loading environment file", err)
	}

	return &env, nil
}
