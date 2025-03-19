package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type OllamaEndpoint string

const (
	GenerateEndpoint  OllamaEndpoint = "generate"
	EmbeddingEndpoint OllamaEndpoint = "embeddings"
)

type OllamaRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Stream bool     `json:"stream"`
	Images []string `json:"images"`
}

type OllamaConnection struct {
	Path          OllamaEndpoint
	Model         string
	OllamaRequest OllamaRequest
}

type OllamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

func NewOllamaConnection(path OllamaEndpoint, model string, request OllamaRequest) *OllamaConnection {
	return &OllamaConnection{
		Path:          path,
		Model:         model,
		OllamaRequest: request,
	}
}

func (c *OllamaConnection) Request() (*http.Response, error) {
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "localhost"
	}

	ollamaURL := fmt.Sprintf("http://%s:11434/api/%s", ollamaHost, c.Path)

	requestBody, _ := json.Marshal(c.OllamaRequest)

	resp, err := http.Post(ollamaURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama at %s: %v", ollamaURL, err)
	}
	return resp, err
}
