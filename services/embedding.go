package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func GenerateEmbedding(text string) ([]float32, error) {
	requestBody, _ := json.Marshal(OllamaRequest{
		Model:  "nomic-embed-text",
		Prompt: text,
	})

	resp, err := http.Post("http://localhost:11434/api/embeddings", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %v", err)
	}

	defer resp.Body.Close()

	var result OllamaResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result.Embedding, nil
}
