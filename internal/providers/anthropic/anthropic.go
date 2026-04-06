// Package anthropic implements the Anthropic/Claude LLM provider.
package anthropic

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/opentide/opentide/internal/providers"
	oerr "github.com/opentide/opentide/pkg/errors"
)

const defaultModel = "claude-sonnet-4-20250514"

type Provider struct {
	client *anthropic.Client
	model  string
}

func New(apiKey, model string) (*Provider, error) {
	if apiKey == "" {
		return nil, oerr.New(oerr.CodeProviderAuth, "Anthropic API key is empty").
			WithFix("Set the ANTHROPIC_API_KEY environment variable").
			WithDocs("https://docs.anthropic.com/en/api/getting-started")
	}
	if model == "" {
		model = defaultModel
	}
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Provider{client: &client, model: model}, nil
}

func (p *Provider) Chat(ctx context.Context, msgs []providers.ChatMessage, tools []providers.Tool) (*providers.Response, error) {
	anthropicMsgs := toAnthropicMessages(msgs)
	systemPrompt := extractSystem(msgs)

	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 4096,
		Messages:  anthropicMsgs,
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}
	if len(tools) > 0 {
		params.Tools = toAnthropicTools(tools)
	}

	msg, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, oerr.Wrap(oerr.CodeProviderError, "Anthropic API request failed", err).
			WithFix("Check your API key and network connection. If rate limited, wait and retry.")
	}

	return fromAnthropicResponse(msg), nil
}

func (p *Provider) StreamChat(ctx context.Context, msgs []providers.ChatMessage, tools []providers.Tool) (<-chan providers.StreamEvent, error) {
	anthropicMsgs := toAnthropicMessages(msgs)
	systemPrompt := extractSystem(msgs)

	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 4096,
		Messages:  anthropicMsgs,
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}
	if len(tools) > 0 {
		params.Tools = toAnthropicTools(tools)
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	ch := make(chan providers.StreamEvent, 64)
	go func() {
		defer close(ch)
		for stream.Next() {
			evt := stream.Current()
			switch evt.Type {
			case "content_block_delta":
				if evt.Delta.Type == "text_delta" {
					ch <- providers.StreamEvent{Content: evt.Delta.Text}
				}
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
func (p *Provider) Name() string    { return "anthropic" }

func extractSystem(msgs []providers.ChatMessage) string {
	for _, m := range msgs {
		if m.Role == providers.RoleSystem {
			return m.Content
		}
	}
	return ""
}

func toAnthropicMessages(msgs []providers.ChatMessage) []anthropic.MessageParam {
	var result []anthropic.MessageParam
	for _, m := range msgs {
		if m.Role == providers.RoleSystem {
			continue
		}
		role := anthropic.MessageParamRoleUser
		if m.Role == providers.RoleAssistant {
			role = anthropic.MessageParamRoleAssistant
		}
		result = append(result, anthropic.MessageParam{
			Role: role,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(m.Content),
			},
		})
	}
	return result
}

func toAnthropicTools(tools []providers.Tool) []anthropic.ToolUnionParam {
	var result []anthropic.ToolUnionParam
	for _, t := range tools {
		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: t.Parameters,
				},
			},
		})
	}
	return result
}

func fromAnthropicResponse(msg *anthropic.Message) *providers.Response {
	resp := &providers.Response{
		Model: string(msg.Model),
		Usage: providers.Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
	}

	for _, block := range msg.Content {
		switch {
		case block.Type == "text":
			resp.Content += block.Text
		case block.Type == "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, providers.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: fmt.Sprintf("%v", block.Input),
			})
		}
	}

	return resp
}
