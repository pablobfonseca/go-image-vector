package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "localhost"
	}

	ollamaURL := fmt.Sprintf("http://%s:11434/api/generate", ollamaHost)

	requestBody, _ := json.Marshal(OllamaRequest{
		Model:  model,
		Prompt: "Tell me what's happening in this image and figure out the context in natural language, always respond using the markdown syntax",
		Images: []string{imageBase64},
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

// ExtractTextFromMultipleImages analyzes multiple images at once to understand context connections
func ExtractTextFromMultipleImages(imagePaths []string) (string, error) {
	if len(imagePaths) == 0 {
		return "", fmt.Errorf("no image paths provided")
	}

	// Convert all images to base64
	imageBase64List := []string{}
	for _, path := range imagePaths {
		file, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("failed to open image %s: %v", path, err)
		}

		imageBytes, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read image %s: %v", path, err)
		}

		imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)
		imageBase64List = append(imageBase64List, imageBase64)
	}

	model := viper.GetString("MODEL")
	if model == "" {
		model = "gemma3"
	}

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "localhost"
	}

	ollamaURL := fmt.Sprintf("http://%s:11434/api/generate", ollamaHost)

	// Enhanced prompt for analyzing multiple images together
	batchPrompt := "I'm showing you multiple sequential screenshots from a user journey on a website. " +
		"Analyze these images as a sequence and describe the complete user journey. " +
		"Focus on identifying patterns, user actions, and transitions between pages. " +
		"What is the user trying to accomplish? What steps are they taking? " +
		"What might be their goals or pain points? " +
		"Provide a detailed narrative of the entire journey, not just individual images. " +
		"Always respond using markdown syntax."

	requestBody, _ := json.Marshal(OllamaRequest{
		Model:  model,
		Prompt: batchPrompt,
		Images: imageBase64List,
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
