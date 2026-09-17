package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wishmatic/neo-mcp/internal/utils"
)

type DescribeRequest struct {
	ImageData    []byte
	MediaType    string
	Prompt       string
	SystemPrompt string
	Model        string
	Temperature  float64
	MaxTokens    int
	TopP         float64
	Detail       string
}

type Result struct {
	Text  string
	Model string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type contentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *imageURLPart `json:"image_url,omitempty"`
}

type imageURLPart struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	TopP        float64       `json:"top_p"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) Describe(ctx context.Context, req DescribeRequest) (Result, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(c.defaultModel)
	}

	if model == "" {
		return Result{}, fmt.Errorf("openai: no model configured")
	}

	if strings.TrimSpace(req.SystemPrompt) == "" {
		req.SystemPrompt = c.defaultSystemPrompt
	}

	if len(req.ImageData) == 0 {
		return Result{}, fmt.Errorf("openai: empty image data")
	}

	body, err := json.Marshal(buildPayload(req, model))
	if err != nil {
		return Result{}, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("openai: build request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("openai: call endpoint: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Result{}, c.httpError(resp)
	}

	text, err := decodeCompletion(resp.Body)
	if err != nil {
		return Result{}, err
	}

	return Result{Text: text, Model: model}, nil
}

func buildPayload(req DescribeRequest, model string) chatCompletionRequest {
	messages := make([]chatMessage, 0, 2)

	if system := strings.TrimSpace(req.SystemPrompt); system != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}

	messages = append(messages, chatMessage{
		Role: "user",
		Content: []contentPart{
			{Type: "text", Text: req.Prompt},
			{Type: "image_url", ImageURL: &imageURLPart{URL: dataURI(req.MediaType, req.ImageData), Detail: req.Detail}},
		},
	})

	return chatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
	}
}

func dataURI(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func decodeCompletion(r io.Reader) (string, error) {
	var out chatCompletionResponse
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return "", fmt.Errorf("openai: decode response: %w", err)
	}

	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai: response contained no choices")
	}

	text, err := decodeContent(out.Choices[0].Message.Content)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("openai: response contained no text")
	}

	return text, nil
}

func decodeContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("openai: decode message content: %w", err)
	}

	var b strings.Builder
	for _, part := range parts {
		if part.Type == "text" {
			b.WriteString(part.Text)
		}
	}

	return b.String(), nil
}

func (c *Client) httpError(resp *http.Response) error {
	body := utils.ReadLimited(resp.Body)

	if message := upstreamError(body); message != "" {
		return fmt.Errorf("openai: endpoint returned HTTP %d: %s", resp.StatusCode, message)
	}

	if body != "" {
		return fmt.Errorf("openai: endpoint returned HTTP %d: %s", resp.StatusCode, body)
	}

	return fmt.Errorf("openai: endpoint returned HTTP %d", resp.StatusCode)
}

func upstreamError(body string) string {
	var out struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil || out.Error == nil {
		return ""
	}

	return strings.TrimSpace(out.Error.Message)
}
