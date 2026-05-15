package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// -----------------------------------------------------------------------
// Provider types
// -----------------------------------------------------------------------

type AIProvider string

const (
	ProviderOllama AIProvider = "ollama"
	ProviderClaude AIProvider = "claude"
	ProviderOpenAI AIProvider = "openai"
)

type AIModel struct {
	Provider AIProvider
	Name     string   // display name shown in picker
	ModelID  string   // actual model string sent to API
	NeedsKey bool     // whether an API key is required
	EnvKey   string   // env var name for the key
}

// All selectable models — add more Ollama models here freely
var AvailableModels = []AIModel{
	{ProviderOllama, "Ollama — llama3", "llama3", false, ""},
	{ProviderOllama, "Ollama — mistral", "mistral", false, ""},
	{ProviderOllama, "Ollama — phi4", "phi4", false, ""},
	{ProviderOllama, "Ollama — gemma3", "gemma3", false, ""},
	{ProviderClaude, "Claude — claude-sonnet-4-20250514", "claude-sonnet-4-20250514", true, "ANTHROPIC_API_KEY"},
	{ProviderClaude, "Claude — claude-haiku-4-5-20251001", "claude-haiku-4-5-20251001", true, "ANTHROPIC_API_KEY"},
	{ProviderOpenAI, "OpenAI — gpt-4o", "gpt-4o", true, "OPENAI_API_KEY"},
	{ProviderOpenAI, "OpenAI — gpt-4o-mini", "gpt-4o-mini", true, "OPENAI_API_KEY"},
}

// -----------------------------------------------------------------------
// Summarize — dispatches to the right provider
// -----------------------------------------------------------------------

func summarizeWithAI(content string, model AIModel) (string, error) {
	if content == "" {
		return "", fmt.Errorf("no content provided")
	}

	prompt := fmt.Sprintf(
		`You are a sharp personal assistant. Summarize this Notion page into concise actionable insights. Be direct, no fluff.

Content:
%s

Briefing:`, content)

	switch model.Provider {
	case ProviderOllama:
		return callOllama(prompt, model.ModelID)
	case ProviderClaude:
		return callClaude(prompt, model)
	case ProviderOpenAI:
		return callOpenAI(prompt, model)
	}
	return "", fmt.Errorf("unknown provider: %s", model.Provider)
}

// -----------------------------------------------------------------------
// Ollama
// -----------------------------------------------------------------------

func callOllama(prompt, modelID string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  modelID,
		"prompt": prompt,
		"stream": false,
	})

	resp, err := http.Post(
		"http://localhost:11434/api/generate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("Ollama not reachable — run: ollama serve (%v)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("could not parse Ollama response")
	}
	return strings.TrimSpace(result.Response), nil
}

// -----------------------------------------------------------------------
// Claude (Anthropic)
// -----------------------------------------------------------------------

func callClaude(prompt string, model AIModel) (string, error) {
	apiKey, err := resolveKey(model)
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]any{
		"model":      model.ModelID,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Claude %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Content) == 0 {
		return "", fmt.Errorf("could not parse Claude response")
	}
	return strings.TrimSpace(result.Content[0].Text), nil
}

// -----------------------------------------------------------------------
// OpenAI
// -----------------------------------------------------------------------

func callOpenAI(prompt string, model AIModel) (string, error) {
	apiKey, err := resolveKey(model)
	if err != nil {
		return "", err
	}

	payload, _ := json.Marshal(map[string]any{
		"model": model.ModelID,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Choices) == 0 {
		return "", fmt.Errorf("could not parse OpenAI response")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

// -----------------------------------------------------------------------
// Key resolution — env var first, then config file field
// -----------------------------------------------------------------------

func resolveKey(model AIModel) (string, error) {
	if !model.NeedsKey {
		return "", nil
	}
	cfg, _ := loadConfig()
	var key string
	switch model.Provider {
	case ProviderClaude:
		key = cfg.AnthropicKey
	case ProviderOpenAI:
		key = cfg.OpenAIKey
	}
	if key == "" {
		return "", fmt.Errorf("%s not set — add it in the model picker (press m)", model.EnvKey)
	}
	return key, nil
}