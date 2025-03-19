package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/viper"
)

func GenerateEmbedding(text string) ([]float32, error) {
	model := viper.GetString("EMBEDDING_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}
	requestBody, _ := json.Marshal(OllamaRequest{
		Model:  model,
		Prompt: text,
	})

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "localhost"
	}

	ollamaURL := fmt.Sprintf("http://%s:11434/api/embeddings", ollamaHost)

	resp, err := http.Post(ollamaURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama at %s: %v", ollamaURL, err)
	}

	defer resp.Body.Close()

	var result OllamaResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result.Embedding, nil
}
