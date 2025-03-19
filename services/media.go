package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablobfonseca/go-image-vector/models"
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

// DetectMediaType determines if a file is an image or video based on extension
func DetectMediaType(filePath string) models.MediaType {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp4", ".mov", ".avi", ".mkv", ".webm":
		return models.MediaTypeVideo
	default:
		return models.MediaTypeImage
	}
}

// ExtractTextFromMedia extracts text description from media file (image or video)
func ExtractTextFromMedia(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}

	defer file.Close()

	mediaBytes, _ := io.ReadAll(file)
	mediaBase64 := base64.StdEncoding.EncodeToString(mediaBytes)

	model := viper.GetString("MODEL")
	if model == "" {
		model = "gemma3"
	}

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "localhost"
	}

	ollamaURL := fmt.Sprintf("http://%s:11434/api/generate", ollamaHost)

	mediaType := DetectMediaType(filePath)
	prompt := "Tell me what's happening in this image and figure out the context in natural language, always respond using the markdown syntax"
	if mediaType == models.MediaTypeVideo {
		prompt = "Tell me what's happening in this video and figure out the context in natural language, always respond using the markdown syntax"
	}

	requestBody, _ := json.Marshal(OllamaRequest{
		Model:  model,
		Prompt: prompt,
		Images: []string{mediaBase64},
		Stream: false,
	})

	resp, err := http.Post(ollamaURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama at %s: %v", ollamaURL, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if the response field exists and convert it to string properly
	if response, ok := result["response"]; ok {
		switch v := response.(type) {
		case string:
			return v, nil
		case bool, float64, int:
			return fmt.Sprintf("%v", v), nil
		default:
			return "", fmt.Errorf("unexpected response type: %T", v)
		}
	}

	return "", fmt.Errorf("no response field in API result")
}

// ExtractTextFromImage is kept for backward compatibility
func ExtractTextFromImage(imagePath string) (string, error) {
	return ExtractTextFromMedia(imagePath)
}
