package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	SelectedModel int    `json:"selected_model"` // index into AvailableModels
	AnthropicKey  string `json:"anthropic_key,omitempty"`
	OpenAIKey     string `json:"openai_key,omitempty"`
}

func configPath() string {
	dir, _ := os.Getwd()
	return filepath.Join(dir,"data", "config.json")
}

func loadConfig() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg, err // file doesn't exist yet — caller uses defaults
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func saveConfig(cfg Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}