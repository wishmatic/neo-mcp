package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/wishmatic/neo-mcp/internal/utils"
)

const (
	finishReasonLength        = "length"
	finishReasonContentFilter = "content_filter"

	maxResponseBytes = 4 << 20
)

type DescribeRequest struct {
	ImageData []byte
	MediaType string
	Prompt    string
}

type Result struct {
	Text      string
	Model     string
	Truncated bool
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
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatCompletionMessage struct {
	Content          json.RawMessage `json:"content"`
	ReasoningContent string          `json:"reasoning_content"`
	Reasoning        string          `json:"reasoning"`
}

type chatCompletionChoice struct {
	FinishReason string                `json:"finish_reason"`
	Message      chatCompletionMessage `json:"message"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

type completion struct {
	text         string
	finishReason string
}

func (c *Client) Describe(ctx context.Context, req DescribeRequest) (Result, error) {
	model := strings.TrimSpace(c.defaultModel)
	if model == "" {
		return Result{}, fmt.Errorf("openai: no model configured")
	}

	if len(req.ImageData) == 0 {
		return Result{}, fmt.Errorf("openai: empty image data")
	}

	body, err := json.Marshal(c.buildPayload(req, model))
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

	out, err := decodeCompletion(resp.Body, c.maxTokens)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Text:      out.text,
		Model:     model,
		Truncated: out.finishReason == finishReasonLength,
	}, nil
}

func (c *Client) buildPayload(req DescribeRequest, model string) chatCompletionRequest {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = c.prompt()
	}

	messages := make([]chatMessage, 0, 2)

	if system := strings.TrimSpace(c.systemPrompt()); system != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}

	messages = append(messages, chatMessage{
		Role: "user",
		Content: []contentPart{
			{Type: "text", Text: prompt},
			{Type: "image_url", ImageURL: &imageURLPart{URL: dataURI(req.MediaType, req.ImageData), Detail: "high"}},
		},
	})

	return chatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0,
		MaxTokens:   c.maxTokens,
	}
}

func dataURI(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func decodeCompletion(r io.Reader, maxTokens int) (completion, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxResponseBytes))
	if err != nil {
		return completion{}, fmt.Errorf("openai: read response: %w", err)
	}

	var out chatCompletionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return completion{}, fmt.Errorf("openai: decode response: %w", err)
	}

	if len(out.Choices) == 0 {
		return completion{}, fmt.Errorf("openai: response contained no choices")
	}

	choice := out.Choices[0]

	text, err := decodeContent(choice.Message.Content)
	if err != nil {
		return completion{}, err
	}

	if strings.TrimSpace(text) == "" {
		return completion{}, emptyTextError(choice, body, maxTokens)
	}

	return completion{text: text, finishReason: choice.FinishReason}, nil
}

func emptyTextError(choice chatCompletionChoice, body []byte, maxTokens int) error {
	if choice.FinishReason == finishReasonLength {
		return fmt.Errorf("openai: response truncated at max_tokens %s before any text was produced", maxTokensLabel(maxTokens))
	}

	if reasoningText(choice.Message) != "" {
		return fmt.Errorf("openai: model produced reasoning but no answer text (finish_reason %q)", choice.FinishReason)
	}

	if choice.FinishReason == finishReasonContentFilter {
		return fmt.Errorf("openai: response was blocked by the provider's content filter")
	}

	return fmt.Errorf("openai: response contained no text (finish_reason %q): %s", choice.FinishReason, responseSnippet(body))
}

func maxTokensLabel(maxTokens int) string {
	if maxTokens <= 0 {
		return "unset"
	}

	return strconv.Itoa(maxTokens)
}

func reasoningText(message chatCompletionMessage) string {
	if content := strings.TrimSpace(message.ReasoningContent); content != "" {
		return content
	}

	return strings.TrimSpace(message.Reasoning)
}

func responseSnippet(body []byte) string {
	return utils.ReadLimited(bytes.NewReader(body))
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
