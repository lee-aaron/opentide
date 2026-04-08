package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/opentide/opentide/internal/adapters"
	"github.com/opentide/opentide/internal/approval"
	"github.com/opentide/opentide/internal/memory"
	"github.com/opentide/opentide/internal/providers"
	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/internal/skills"
	"github.com/opentide/opentide/internal/state"
)

const maxMessageSize = 65536 // 64KB

// Gateway is the core message processing loop.
type Gateway struct {
	registry    *providers.Registry
	adapter     adapters.Adapter
	store       state.Store
	memory      memory.Store
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

	defaultProvider := g.registry.Default()
	if defaultProvider != nil {
		g.logger.Info("gateway started", "provider", defaultProvider.Name(), "model", defaultProvider.ModelID())
	} else {
		g.logger.Info("gateway started", "provider", "none")
	}

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

	// Handle slash commands before LLM call
	if handled := g.handleModelCommand(ctx, msg); handled {
		return
	}
	if handled := g.handleMemoryCommand(ctx, msg); handled {
		return
	}

	// Resolve provider for this message (user override > channel route > default)
	provider := g.registry.Resolve(msg.UserID, msg.ChannelID)
	if provider == nil {
		g.logger.Error("no provider available")
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: "No LLM provider is configured. Please set up a provider.",
		})
		return
	}
	g.logger.Debug("provider resolved", "provider", provider.Name(), "model", provider.ModelID(),
		"user", msg.UserID, "channel", msg.ChannelID)

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

	// Build conversation context (scoped per channel for privacy)
	history, err := g.store.GetHistory(ctx, msg.ChannelID, 20)
	if err != nil {
		g.logger.Error("failed to get history", "err", err)
	}

	chatMsgs := []providers.ChatMessage{
		{Role: providers.RoleSystem, Content: g.buildSystemPrompt(ctx, msg.UserID)},
	}
	for _, entry := range history {
		chatMsgs = append(chatMsgs, entry.Message)
	}

	// Build tool definitions from loaded skills
	tools := g.buildToolDefs(ctx)

	// Call LLM (pinned provider for entire message handling)
	resp, err := provider.Chat(ctx, chatMsgs, tools)
	if err != nil {
		g.logger.Error("LLM request failed", "err", err, "provider", provider.Name())
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: "Sorry, I encountered an error processing your message. Please try again.",
		})
		return
	}

	// Handle tool calls (skill invocations)
	if len(resp.ToolCalls) > 0 {
		g.handleToolCalls(ctx, msg, chatMsgs, resp, provider)
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

// handleModelCommand processes /model commands for runtime provider switching.
// Returns true if the message was a /model command and was handled.
func (g *Gateway) handleModelCommand(ctx context.Context, msg adapters.IncomingMessage) bool {
	if len(msg.Content) < 6 || msg.Content[:6] != "/model" {
		return false
	}

	parts := strings.Fields(msg.Content)
	if len(parts) == 1 {
		// /model — show current provider and available providers
		current := g.registry.Resolve(msg.UserID, msg.ChannelID)
		var currentName string
		if current != nil {
			currentName = fmt.Sprintf("%s (%s)", current.Name(), current.ModelID())
		} else {
			currentName = "none"
		}

		overrideName, overrideModel, hasOverride := g.registry.GetUserOverride(msg.UserID)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**Current provider:** %s\n", currentName))
		if hasOverride {
			sb.WriteString(fmt.Sprintf("**User override:** %s", overrideName))
			if overrideModel != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", overrideModel))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n**Available providers:**\n")
		for _, info := range g.registry.List() {
			marker := " "
			if current != nil && info.Name == current.Name() {
				marker = "→"
			}
			sb.WriteString(fmt.Sprintf("%s %s (%s)\n", marker, info.Name, info.Model))
		}
		sb.WriteString("\nUsage: `/model <provider> [model]` or `/model reset`")

		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{Content: sb.String()})
		return true
	}

	if parts[1] == "reset" {
		g.registry.ClearUserOverride(msg.UserID)
		defaultProvider := g.registry.Resolve(msg.UserID, msg.ChannelID)
		var name string
		if defaultProvider != nil {
			name = defaultProvider.Name()
		}
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: fmt.Sprintf("Model override cleared. Using default: **%s**", name),
		})
		return true
	}

	// /model <provider> [model]
	providerName := parts[1]
	var model string
	if len(parts) >= 3 {
		model = parts[2]
	}

	if ok := g.registry.SetUserOverride(msg.UserID, providerName, model); !ok {
		available := make([]string, 0)
		for _, info := range g.registry.List() {
			available = append(available, info.Name)
		}
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: fmt.Sprintf("Unknown provider **%s**. Available: %s", providerName, strings.Join(available, ", ")),
		})
		return true
	}

	resp := fmt.Sprintf("Switched to **%s**", providerName)
	if model != "" {
		resp += fmt.Sprintf(" (%s)", model)
	}
	resp += ". Override lasts 24h or until `/model reset`."
	g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{Content: resp})
	return true
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
		if !info.Enabled {
			continue
		}
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
func (g *Gateway) handleToolCalls(ctx context.Context, msg adapters.IncomingMessage, chatMsgs []providers.ChatMessage, resp *providers.Response, provider providers.Provider) {
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
	finalResp, err := provider.Chat(ctx, chatMsgs, nil)
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

// handleMemoryCommand processes /remember, /memories, /forget, /forget-all commands.
// Returns true if the message was a memory command and was handled.
func (g *Gateway) handleMemoryCommand(ctx context.Context, msg adapters.IncomingMessage) bool {
	if g.memory == nil {
		return false
	}

	content := strings.TrimSpace(msg.Content)

	switch {
	case strings.HasPrefix(content, "/remember "):
		note := strings.TrimSpace(content[10:])
		if note == "" {
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: "Usage: `/remember <something to remember>`",
			})
			return true
		}
		if _, err := g.memory.Add(ctx, msg.UserID, note); err != nil {
			g.logger.Error("failed to save memory", "err", err)
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: "Failed to save memory. Please try again.",
			})
			return true
		}
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: fmt.Sprintf("Remembered: *%s*", note),
		})
		return true

	case content == "/memories":
		notes, err := g.memory.List(ctx, msg.UserID)
		if err != nil {
			g.logger.Error("failed to list memories", "err", err)
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: "Failed to retrieve memories. Please try again.",
			})
			return true
		}
		if len(notes) == 0 {
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: "No memories saved. Use `/remember <note>` to add one.",
			})
			return true
		}
		var sb strings.Builder
		sb.WriteString("**Your memories:**\n")
		for _, n := range notes {
			fmt.Fprintf(&sb, "• `#%d` %s\n", n.ID, n.Text)
		}
		sb.WriteString("\nUse `/forget <id>` to remove one, or `/forget-all` to clear all.")
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{Content: sb.String()})
		return true

	case strings.HasPrefix(content, "/forget-all"):
		count, err := g.memory.DeleteAll(ctx, msg.UserID)
		if err != nil {
			g.logger.Error("failed to delete all memories", "err", err)
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: "Failed to clear memories. Please try again.",
			})
			return true
		}
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: fmt.Sprintf("Cleared %d memories.", count),
		})
		return true

	case strings.HasPrefix(content, "/forget "):
		idStr := strings.TrimSpace(content[8:])
		idStr = strings.TrimPrefix(idStr, "#")
		var noteID int64
		if _, err := fmt.Sscanf(idStr, "%d", &noteID); err != nil {
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: "Usage: `/forget <id>` (use `/memories` to see IDs)",
			})
			return true
		}
		if err := g.memory.Delete(ctx, msg.UserID, noteID); err != nil {
			g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
				Content: fmt.Sprintf("Note #%d not found. Use `/memories` to see your notes.", noteID),
			})
			return true
		}
		g.adapter.SendMessage(ctx, msg.ChannelID, adapters.Message{
			Content: fmt.Sprintf("Forgot note #%d.", noteID),
		})
		return true
	}

	return false
}

// buildSystemPrompt creates the system prompt, optionally including user memories.
func (g *Gateway) buildSystemPrompt(ctx context.Context, userID string) string {
	base := `You are OpenTide, a secure AI assistant. You are helpful, direct, and concise.
You prioritize user safety and transparency. When you don't know something, say so.`

	if g.memory == nil {
		return base
	}

	notes, err := g.memory.List(ctx, userID)
	if err != nil || len(notes) == 0 {
		return base
	}

	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n\nThe user has saved the following notes for context:\n")
	for _, n := range notes {
		fmt.Fprintf(&sb, "- %s\n", n.Text)
	}
	return sb.String()
}
