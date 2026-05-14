package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func summarizeWithOllama(content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("no content provided for summarization")
	}

	prompt := fmt.Sprintf(
		`You are a sharp personal assistant. Summarize this Notion page into concise actionable insights. Be direct, no fluff.

Content:
%s

Briefing:`, content)

	body, err := json.Marshal(map[string]any{
		"model":  "llama3",
		"prompt": prompt,
		"stream": false,
	})
	if err != nil {
		return "", err
	}

	resp, err := http.Post(
		"http://localhost:11434/api/generate",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("Ollama not reachable. Make sure it is running: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("could not parse Ollama response")
	}
	return strings.TrimSpace(result.Response), nil
}
