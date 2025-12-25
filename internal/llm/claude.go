package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ClaudeProvider implements the Provider interface for Anthropic's Claude
type ClaudeProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// ClaudeConfig holds configuration for the Claude provider
type ClaudeConfig struct {
	APIKey  string
	BaseURL string // default: https://api.anthropic.com
	Model   string // default: claude-sonnet-4-20250514
}

// NewClaudeProvider creates a new Claude provider
func NewClaudeProvider(cfg ClaudeConfig) *ClaudeProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}

	return &ClaudeProvider{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		httpClient: &http.Client{},
	}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) SupportsStreaming() bool {
	return true
}

type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []claudeMessage `json:"messages"`
	System      string          `json:"system,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	StopSeqs    []string        `json:"stop_sequences,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (p *ClaudeProvider) Generate(ctx context.Context, req *Request) (*Response, error) {
	claudeReq := p.buildRequest(req, false)

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var claudeResp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return p.parseResponse(&claudeResp), nil
}

func (p *ClaudeProvider) GenerateStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	claudeReq := p.buildRequest(req, true)

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	ch := make(chan StreamChunk, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- StreamChunk{Done: true}
				return
			}

			var event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}

			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
				ch <- StreamChunk{Content: event.Delta.Text}
			}

			if event.Type == "message_stop" {
				ch <- StreamChunk{Done: true}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: err}
		}
	}()

	return ch, nil
}

func (p *ClaudeProvider) buildRequest(req *Request, stream bool) *claudeRequest {
	model := req.Model
	if model == "" {
		model = p.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	messages := make([]claudeMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			continue // System handled separately
		}
		messages = append(messages, claudeMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	// Extract system message
	system := req.System
	for _, m := range req.Messages {
		if m.Role == RoleSystem {
			system = m.Content
			break
		}
	}

	return &claudeRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		Messages:    messages,
		System:      system,
		Temperature: req.Temperature,
		StopSeqs:    req.StopSeqs,
		Stream:      stream,
	}
}

func (p *ClaudeProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
}

func (p *ClaudeProvider) parseResponse(resp *claudeResponse) *Response {
	var content string
	for _, c := range resp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &Response{
		Content:      content,
		FinishReason: resp.StopReason,
		Usage: Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
	}
}
