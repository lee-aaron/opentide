package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/opentide/opentide/internal/adapters"
	"github.com/opentide/opentide/internal/approval"
	"github.com/opentide/opentide/internal/providers"
	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/internal/skills"
	"github.com/opentide/opentide/internal/state"
)

const maxMessageSize = 65536 // 64KB

// Gateway is the core message processing loop.
type Gateway struct {
	provider    providers.Provider
	adapter     adapters.Adapter
	store       state.Store
	approval    *approval.MemoryEngine
	skills      skills.Engine
	rateLimiter *security.RateLimiter
	logger      *slog.Logger
}

func (g *Gateway) Run(ctx context.Context) error {
	if err := g.adapter.Connect(ctx); err != nil {
		return fmt.Errorf("adapter connect: %w", err)
	}

	messages, err := g.adapter.ReceiveMessages(ctx)
	if err != nil {
		return fmt.Errorf("receive messages: %w", err)
	}

	g.logger.Info("gateway started", "provider", g.provider.Name(), "model", g.provider.ModelID())

	for {
		select {
		case <-ctx.Done():
			g.logger.Info("gateway shutting down")
			return g.adapter.Close()
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			g.handleMessage(ctx, msg)
		}
	}
}

func (g *Gateway) handleMessage(ctx context.Context, msg adapters.IncomingMessage) {
	// Rate limiting
	if g.rateLimiter != nil {
		key := security.RateLimitKey("user", msg.UserID)
		if !g.rateLimiter.Allow(key) {
			g.logger.Warn("rate limited", "user", msg.UserID)
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: "You're sending messages too quickly. Please wait a moment.",
			})
			return
		}
	}

	// Input validation
	if len(msg.Content) > maxMessageSize {
		g.logger.Warn("message too large, dropping",
			"user", msg.UserID, "size", len(msg.Content), "max", maxMessageSize)
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: fmt.Sprintf("Message too large (%d bytes, max %d). Please shorten your message.", len(msg.Content), maxMessageSize),
		})
		return
	}

	if !utf8.ValidString(msg.Content) {
		g.logger.Warn("invalid UTF-8 in message, dropping", "user", msg.UserID)
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: "Message contains invalid characters. Please use valid UTF-8 text.",
		})
		return
	}

	// Save incoming message
	g.store.SaveMessage(ctx, state.ConversationEntry{
		Timestamp: time.Now(),
		UserID:    msg.UserID,
		ChannelID: msg.ChannelID,
		Message: providers.ChatMessage{
			Role:    providers.RoleUser,
			Content: msg.Content,
		},
	})

	// Build conversation context
	history, err := g.store.GetHistory(ctx, msg.UserID, 20)
	if err != nil {
		g.logger.Error("failed to get history", "err", err)
	}

	chatMsgs := []providers.ChatMessage{
		{Role: providers.RoleSystem, Content: systemPrompt()},
	}
	for _, entry := range history {
		chatMsgs = append(chatMsgs, entry.Message)
	}

	// Build tool definitions from loaded skills
	tools := g.buildToolDefs(ctx)

	// Call LLM
	resp, err := g.provider.Chat(ctx, chatMsgs, tools)
	if err != nil {
		g.logger.Error("LLM request failed", "err", err, "provider", g.provider.Name())
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: "Sorry, I encountered an error processing your message. Please try again.",
		})
		return
	}

	// Handle tool calls (skill invocations)
	if len(resp.ToolCalls) > 0 {
		g.handleToolCalls(ctx, msg, chatMsgs, resp)
		return
	}

	// Save assistant response
	g.store.SaveMessage(ctx, state.ConversationEntry{
		Timestamp: time.Now(),
		UserID:    msg.UserID,
		ChannelID: msg.ChannelID,
		Message: providers.ChatMessage{
			Role:    providers.RoleAssistant,
			Content: resp.Content,
		},
	})

	// Send response
	if err := g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{Content: resp.Content}); err != nil {
		g.logger.Error("failed to send response", "err", err)
	}
}

// buildToolDefs converts loaded skills into LLM tool definitions.
func (g *Gateway) buildToolDefs(ctx context.Context) []providers.Tool {
	if g.skills == nil {
		return nil
	}

	infos, err := g.skills.ListSkills(ctx)
	if err != nil {
		g.logger.Error("failed to list skills", "err", err)
		return nil
	}

	tools := make([]providers.Tool, 0, len(infos))
	for _, info := range infos {
		tools = append(tools, providers.Tool{
			Name:        info.ToolName,
			Description: info.Description,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The input for the skill",
					},
				},
			},
		})
	}
	return tools
}

// handleToolCalls processes LLM tool call requests by invoking skills,
// then feeds results back to the LLM for a final response.
func (g *Gateway) handleToolCalls(ctx context.Context, msg adapters.IncomingMessage, chatMsgs []providers.ChatMessage, resp *providers.Response) {
	// Add assistant message with tool calls to context
	chatMsgs = append(chatMsgs, providers.ChatMessage{
		Role:    providers.RoleAssistant,
		Content: resp.Content,
	})

	for _, tc := range resp.ToolCalls {
		g.logger.Info("skill invocation", "tool", tc.Name, "id", tc.ID)

		// Parse arguments
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			g.logger.Error("invalid tool call arguments", "err", err)
			args = map[string]any{}
		}

		// Invoke the skill
		input := skills.Input{
			ToolName:  tc.Name,
			Arguments: args,
			UserID:    msg.UserID,
			ChannelID: msg.ChannelID,
		}

		output, err := g.skills.InvokeSkill(ctx, tc.Name, input)
		var result string
		if err != nil {
			g.logger.Error("skill invocation failed", "tool", tc.Name, "err", err)
			result = fmt.Sprintf("Error: %v", err)
		} else if output.Error != "" {
			result = fmt.Sprintf("Error: %s", output.Error)
		} else {
			result = output.Content
		}

		// Add tool result to conversation
		chatMsgs = append(chatMsgs, providers.ChatMessage{
			Role:       providers.RoleTool,
			Content:    result,
			ToolCallID: tc.ID,
		})
	}

	// Call LLM again with tool results for final response
	finalResp, err := g.provider.Chat(ctx, chatMsgs, nil)
	if err != nil {
		g.logger.Error("LLM follow-up failed", "err", err)
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: "Sorry, I encountered an error processing skill results. Please try again.",
		})
		return
	}

	g.store.SaveMessage(ctx, state.ConversationEntry{
		Timestamp: time.Now(),
		UserID:    msg.UserID,
		ChannelID: msg.ChannelID,
		Message: providers.ChatMessage{
			Role:    providers.RoleAssistant,
			Content: finalResp.Content,
		},
	})

	if err := g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{Content: finalResp.Content}); err != nil {
		g.logger.Error("failed to send response", "err", err)
	}
}

func systemPrompt() string {
	return `You are OpenTide, a secure AI assistant. You are helpful, direct, and concise.
You prioritize user safety and transparency. When you don't know something, say so.`
}
