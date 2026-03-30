package config

import (
	"log"

	"github.com/spf13/viper"
)

type Env struct {
	GeminiAPIKey string `mapstructure:"GEMINI_API_KEY"`
	PromptPath   string `mapstructure:"PROMPT_PATH"`
	Host         string `mapstructure:"HOST"`
	Port         string `mapstructure:"PORT"`
}

func NewEnv(filename string, override bool) *Env {
	env := Env{}

	viper.SetConfigFile(filename)

	// load system env
	if override {
		viper.AutomaticEnv()
	}

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("Error reading environment file", err)
	}

	err = viper.Unmarshal(&env)
	if err != nil {
		log.Fatal("Error loading environment file", err)
	}

	return &env
}
