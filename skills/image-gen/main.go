// image-gen skill: generate images using the OpenAI DALL-E API.
// Requires OPENAI_API_KEY environment variable.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Input struct {
	Arguments map[string]any `json:"arguments"`
}

type Output struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

type dalleRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

type dalleResponse struct {
	Data []struct {
		URL string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeError("failed to read input: " + err.Error())
		return
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		writeError("invalid input JSON: " + err.Error())
		return
	}

	query, _ := input.Arguments["query"].(string)
	if query == "" {
		writeError("missing 'query' argument - describe the image you want to generate")
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		writeError("OPENAI_API_KEY is required for image generation")
		return
	}

	url, err := generateImage(apiKey, query)
	if err != nil {
		writeError(err.Error())
		return
	}
	writeOutput(fmt.Sprintf("Generated image: %s\n\nPrompt: %s", url, query))
}

func generateImage(apiKey, prompt string) (string, error) {
	reqBody := dalleRequest{
		Model:  "dall-e-3",
		Prompt: prompt,
		N:      1,
		Size:   "1024x1024",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 55 * time.Second}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/images/generations", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("DALL-E API request failed: %w", err)
	}
	defer resp.Body.Close()

	var dalleResp dalleResponse
	if err := json.NewDecoder(resp.Body).Decode(&dalleResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if dalleResp.Error != nil {
		return "", fmt.Errorf("DALL-E error: %s", dalleResp.Error.Message)
	}

	if len(dalleResp.Data) == 0 {
		return "", fmt.Errorf("no image generated")
	}

	return dalleResp.Data[0].URL, nil
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
