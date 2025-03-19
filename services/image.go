package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/spf13/viper"
)

type OllamaRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Stream bool     `json:"stream"`
	Images []string `json:"images"`
}

type OllamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

func ExtractTextFromImage(imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}

	defer file.Close()

	imageBytes, _ := io.ReadAll(file)
	imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)

	model := viper.GetString("MODEL")
	if model == "" {
		model = "gemma3"
	}

	requestBody, _ := json.Marshal(OllamaRequest{
		Model:  model,
		Prompt: "Tell me what's happening in this image and figure out the context in natural language, always respond using the markdown syntax",
		Images: []string{imageBase64},
		Stream: false,
	})
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	return result["response"], nil
}
