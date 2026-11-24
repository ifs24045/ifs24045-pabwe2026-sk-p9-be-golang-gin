package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"io"
	"log"
)

// ChatClient mengirim prompt ke OpenAI Chat Completions.
type ChatClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type openAIClient struct {
	endpoint string
	apiKey   string
	model    string
	http     *http.Client
}

// NewChatClient membuat client HTTP untuk OpenAI Chat Completions.
func NewChatClient(endpoint, apiKey, model string) ChatClient {
	return &openAIClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		http:     &http.Client{Timeout: 120 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Complete memanggil POST {endpoint}/chat/completions dan mengembalikan isi jawaban.
func (c *openAIClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload, _ := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})

	endpoint := strings.TrimRight(c.endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		log.Printf("[ai] LLM mengembalikan status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
		return "", fmt.Errorf("openai mengembalikan status %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("openai tidak mengembalikan jawaban")
	}
	return parsed.Choices[0].Message.Content, nil
}