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

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// OpenAIConfig holds configuration for the OpenAI provider
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // default: https://api.openai.com
	Model   string // default: gpt-4o
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}

	return &OpenAIProvider{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		httpClient: &http.Client{},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) SupportsStreaming() bool {
	return true
}

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stop        []string        `json:"stop,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (p *OpenAIProvider) Generate(ctx context.Context, req *Request) (*Response, error) {
	openaiReq := p.buildRequest(req, false)

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
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

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return p.parseResponse(&openaiResp), nil
}

func (p *OpenAIProvider) GenerateStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	openaiReq := p.buildRequest(req, true)

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
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
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if len(event.Choices) > 0 {
				if event.Choices[0].Delta.Content != "" {
					ch <- StreamChunk{Content: event.Choices[0].Delta.Content}
				}
				if event.Choices[0].FinishReason != nil {
					ch <- StreamChunk{Done: true}
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: err}
		}
	}()

	return ch, nil
}

func (p *OpenAIProvider) buildRequest(req *Request, stream bool) *openaiRequest {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := make([]openaiMessage, 0, len(req.Messages)+1)

	// Add system message if provided
	if req.System != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		messages = append(messages, openaiMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	return &openaiRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stop:        req.StopSeqs,
		Stream:      stream,
	}
}

func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

func (p *OpenAIProvider) parseResponse(resp *openaiResponse) *Response {
	if len(resp.Choices) == 0 {
		return &Response{}
	}

	return &Response{
		Content:      resp.Choices[0].Message.Content,
		FinishReason: resp.Choices[0].FinishReason,
		Usage: Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}
