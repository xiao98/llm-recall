// OpenAI Chat Completions API client.
//
// Endpoint: POST {baseURL}/v1/chat/completions
// Headers:  Authorization: Bearer <key>
//
//	content-type: application/json
//
// Same wire-format-direct philosophy as anthropic.go (see header there).
// We construct a single user-message turn with the system prompt as a
// separate "system" role entry — Chat Completions' canonical shape.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type openaiClient struct {
	apiKey  string
	model   string
	baseURL string
}

func (c *openaiClient) Vendor() Vendor { return OpenAI }
func (c *openaiClient) Model() string  { return c.model }

type openaiRequestBody struct {
	Model     string             `json:"model"`
	Messages  []openaiRequestMsg `json:"messages"`
	MaxTokens int                `json:"max_tokens,omitempty"`
}

type openaiRequestMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponseBody struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (c *openaiClient) Complete(ctx context.Context, req Request) (Response, error) {
	msgs := make([]openaiRequestMsg, 0, 2)
	if req.System != "" {
		msgs = append(msgs, openaiRequestMsg{Role: "system", Content: req.System})
	}
	msgs = append(msgs, openaiRequestMsg{Role: "user", Content: req.Prompt})

	body := openaiRequestBody{
		Model:     c.model,
		Messages:  msgs,
		MaxTokens: req.MaxTokens,
	}
	return doRetried(ctx, func(ctx context.Context) (Response, error) {
		return c.callOnce(ctx, body)
	})
}

func (c *openaiClient) callOnce(ctx context.Context, body openaiRequestBody) (Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("content-type", "application/json")

	resp, err := httpClient().Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, &APIError{Vendor: OpenAI, Status: resp.StatusCode, Body: string(respBody)}
	}
	var out openaiResponseBody
	if err := json.Unmarshal(respBody, &out); err != nil {
		return Response{}, fmt.Errorf("openai: decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return Response{}, errors.New("openai: empty choices")
	}
	return Response{
		Text:       out.Choices[0].Message.Content,
		InputToks:  out.Usage.PromptTokens,
		OutputToks: out.Usage.CompletionTokens,
	}, nil
}
