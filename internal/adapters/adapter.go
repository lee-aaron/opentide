// Package adapters defines the channel adapter interface and shared types.
package adapters

import "context"

// Platform identifies a messaging platform.
type Platform string

const (
	PlatformDiscord  Platform = "discord"
	PlatformTelegram Platform = "telegram"
	PlatformSlack    Platform = "slack"
	PlatformStdio    Platform = "stdio"
)

// IncomingMessage represents a message received from a platform.
type IncomingMessage struct {
	Platform  Platform `json:"platform"`
	ChannelID string   `json:"channel_id"`
	UserID    string   `json:"user_id"`
	MessageID string   `json:"message_id"`
	Content   string   `json:"content"`
}

// Message is a message to send to a platform.
type Message struct {
	Content string            `json:"content"`
	Buttons []ApprovalButton  `json:"buttons,omitempty"`
}

// ApprovalButton is an inline button for approval UX.
type ApprovalButton struct {
	Label    string `json:"label"`
	ActionID string `json:"action_id"` // maps to an approval action hash
	Style    string `json:"style"`     // "approve", "deny"
}

// Adapter is the interface all messaging platform adapters implement.
type Adapter interface {
	Connect(ctx context.Context) error
	SendMessage(ctx context.Context, channelID string, msg Message) error
	ReceiveMessages(ctx context.Context) (<-chan IncomingMessage, error)
	Platform() Platform
	Close() error
}
