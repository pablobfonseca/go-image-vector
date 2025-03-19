package services

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/viper"
)

func GenerateEmbedding(text string) ([]float32, error) {
	model := viper.GetString("EMBEDDING_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}

	ollamaConnection := NewOllamaConnection(EmbeddingEndpoint, model, OllamaRequest{
		Model:  model,
		Prompt: text,
	})

	resp, err := ollamaConnection.Request()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result OllamaResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result.Embedding, nil
}
