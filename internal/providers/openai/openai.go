// Package openai implements the OpenAI LLM provider.
// Also used by the Gradient provider (same API shape, different base URL).
package openai

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/opentide/opentide/internal/providers"
	oerr "github.com/opentide/opentide/pkg/errors"
)

const defaultModel = "gpt-4o"

type Provider struct {
	client *openai.Client
	model  string
	name   string
}

// New creates an OpenAI provider with the default base URL.
func New(apiKey, model string) (*Provider, error) {
	if apiKey == "" {
		return nil, oerr.New(oerr.CodeProviderAuth, "OpenAI API key is empty").
			WithFix("Set the OPENAI_API_KEY environment variable").
			WithDocs("https://platform.openai.com/api-keys")
	}
	if model == "" {
		model = defaultModel
	}
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &Provider{client: &client, model: model, name: "openai"}, nil
}

// NewWithBaseURL creates a provider with a custom base URL (for DO Gradient).
func NewWithBaseURL(apiKey, model, baseURL, name string) (*Provider, error) {
	if apiKey == "" {
		return nil, oerr.New(oerr.CodeProviderAuth, name+" API key is empty").
			WithFix("Set the appropriate API key environment variable")
	}
	if model == "" {
		model = defaultModel
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	return &Provider{client: &client, model: model, name: name}, nil
}

func (p *Provider) Chat(ctx context.Context, msgs []providers.ChatMessage, tools []providers.Tool) (*providers.Response, error) {
	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: toOpenAIMessages(msgs),
	}
	if len(tools) > 0 {
		params.Tools = toOpenAITools(tools)
	}

	completion, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, oerr.Wrap(oerr.CodeProviderError, p.name+" API request failed", err).
			WithFix("Check your API key and network connection.")
	}

	return fromOpenAIResponse(completion), nil
}

func (p *Provider) StreamChat(ctx context.Context, msgs []providers.ChatMessage, tools []providers.Tool) (<-chan providers.StreamEvent, error) {
	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: toOpenAIMessages(msgs),
	}
	if len(tools) > 0 {
		params.Tools = toOpenAITools(tools)
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	ch := make(chan providers.StreamEvent, 64)
	go func() {
		defer close(ch)
		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				ch <- providers.StreamEvent{Content: chunk.Choices[0].Delta.Content}
			}
		}
		if err := stream.Err(); err != nil {
			ch <- providers.StreamEvent{Err: err}
		}
		ch <- providers.StreamEvent{Done: true}
	}()

	return ch, nil
}

func (p *Provider) ModelID() string { return p.model }
func (p *Provider) Name() string    { return p.name }

func toOpenAIMessages(msgs []providers.ChatMessage) []openai.ChatCompletionMessageParamUnion {
	var result []openai.ChatCompletionMessageParamUnion
	for _, m := range msgs {
		switch m.Role {
		case providers.RoleSystem:
			result = append(result, openai.SystemMessage(m.Content))
		case providers.RoleUser:
			result = append(result, openai.UserMessage(m.Content))
		case providers.RoleAssistant:
			result = append(result, openai.AssistantMessage(m.Content))
		}
	}
	return result
}

func toOpenAITools(tools []providers.Tool) []openai.ChatCompletionToolParam {
	var result []openai.ChatCompletionToolParam
	for _, t := range tools {
		paramBytes, _ := json.Marshal(t.Parameters)
		var params openai.FunctionParameters
		json.Unmarshal(paramBytes, &params)
		result = append(result, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  params,
			},
		})
	}
	return result
}

func fromOpenAIResponse(c *openai.ChatCompletion) *providers.Response {
	resp := &providers.Response{
		Model: c.Model,
		Usage: providers.Usage{
			InputTokens:  int(c.Usage.PromptTokens),
			OutputTokens: int(c.Usage.CompletionTokens),
		},
	}

	if len(c.Choices) > 0 {
		choice := c.Choices[0]
		resp.Content = choice.Message.Content
		for _, tc := range choice.Message.ToolCalls {
			resp.ToolCalls = append(resp.ToolCalls, providers.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	return resp
}
