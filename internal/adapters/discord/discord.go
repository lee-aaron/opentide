// Package discord implements the Discord channel adapter.
// Supports bot gateway messages, slash commands, and button interactions for approval UX.
package discord

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"

	"github.com/opentide/opentide/internal/adapters"
	oerr "github.com/opentide/opentide/pkg/errors"
)

type Adapter struct {
	session  *discordgo.Session
	guildID  string
	messages chan adapters.IncomingMessage
	logger   *slog.Logger

	mu       sync.Mutex
	ready    bool
	closeCh  chan struct{}
}

// New creates a Discord adapter. Token is a bot token (not OAuth).
func New(token, guildID string, logger *slog.Logger) (*Adapter, error) {
	if token == "" {
		return nil, oerr.New(oerr.CodeAdapterConnect, "Discord bot token is empty").
			WithFix("Set the DISCORD_TOKEN environment variable").
			WithDocs("https://discord.com/developers/docs/getting-started")
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, oerr.Wrap(oerr.CodeAdapterConnect, "failed to create Discord session", err)
	}

	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	return &Adapter{
		session:  session,
		guildID:  guildID,
		messages: make(chan adapters.IncomingMessage, 64),
		logger:   logger,
		closeCh:  make(chan struct{}),
	}, nil
}

func (a *Adapter) Connect(ctx context.Context) error {
	a.session.AddHandler(a.onMessageCreate)
	a.session.AddHandler(a.onInteractionCreate)
	a.session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		a.mu.Lock()
		a.ready = true
		a.mu.Unlock()
		a.logger.Info("discord bot connected", "user", r.User.Username, "guilds", len(r.Guilds))
	})

	if err := a.session.Open(); err != nil {
		return oerr.Wrap(oerr.CodeAdapterConnect, "failed to connect to Discord gateway", err).
			WithFix("Check your bot token and ensure the bot has been invited to your server")
	}

	// Register slash command
	_, err := a.session.ApplicationCommandCreate(a.session.State.User.ID, a.guildID, &discordgo.ApplicationCommand{
		Name:        "chat",
		Description: "Send a message to OpenTide",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "Your message",
				Required:    true,
			},
		},
	})
	if err != nil {
		a.logger.Warn("failed to register slash command, falling back to message-based interaction", "err", err)
	}

	return nil
}

func (a *Adapter) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Ignore messages that don't mention the bot (in guild channels)
	if m.GuildID != "" && !isBotMentioned(s, m.Message) {
		return
	}

	content := cleanMentions(s, m.Message)
	if content == "" {
		return
	}

	// Input validation: size limit
	if len(content) > 65536 {
		s.ChannelMessageSend(m.ChannelID, "Message too large (max 64KB). Please shorten your message.")
		return
	}

	a.messages <- adapters.IncomingMessage{
		Platform:  adapters.PlatformDiscord,
		ChannelID: m.ChannelID,
		UserID:    m.Author.ID,
		MessageID: m.ID,
		Content:   content,
	}
}

func (a *Adapter) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		a.handleSlashCommand(s, i)
	case discordgo.InteractionMessageComponent:
		a.handleButtonClick(s, i)
	}
}

func (a *Adapter) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "chat" {
		return
	}

	var content string
	for _, opt := range data.Options {
		if opt.Name == "message" {
			content = opt.StringValue()
		}
	}

	if content == "" {
		return
	}

	// Acknowledge the interaction
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	userID := ""
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	a.messages <- adapters.IncomingMessage{
		Platform:  adapters.PlatformDiscord,
		ChannelID: i.ChannelID,
		UserID:    userID,
		MessageID: i.ID,
		Content:   content,
	}
}

func (a *Adapter) handleButtonClick(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()

	userID := ""
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	// Button clicks are routed as messages with a special prefix
	a.messages <- adapters.IncomingMessage{
		Platform:  adapters.PlatformDiscord,
		ChannelID: i.ChannelID,
		UserID:    userID,
		MessageID: i.ID,
		Content:   fmt.Sprintf("__approval_response:%s", data.CustomID),
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
}

func (a *Adapter) SendMessage(_ context.Context, channelID string, msg adapters.Message) error {
	if len(msg.Buttons) == 0 {
		_, err := a.session.ChannelMessageSend(channelID, msg.Content)
		if err != nil {
			return oerr.Wrap(oerr.CodeAdapterSend, "failed to send Discord message", err)
		}
		return nil
	}

	// Build message with approval buttons
	components := []discordgo.MessageComponent{}
	var buttons []discordgo.MessageComponent
	for _, b := range msg.Buttons {
		style := discordgo.SecondaryButton
		if b.Style == "approve" {
			style = discordgo.SuccessButton
		} else if b.Style == "deny" {
			style = discordgo.DangerButton
		}
		buttons = append(buttons, discordgo.Button{
			Label:    b.Label,
			Style:    style,
			CustomID: b.ActionID,
		})
	}
	components = append(components, discordgo.ActionsRow{Components: buttons})

	_, err := a.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content:    msg.Content,
		Components: components,
	})
	if err != nil {
		return oerr.Wrap(oerr.CodeAdapterSend, "failed to send Discord message with buttons", err)
	}
	return nil
}

func (a *Adapter) ReceiveMessages(_ context.Context) (<-chan adapters.IncomingMessage, error) {
	return a.messages, nil
}

func (a *Adapter) Platform() adapters.Platform {
	return adapters.PlatformDiscord
}

func (a *Adapter) Close() error {
	close(a.closeCh)
	return a.session.Close()
}

func isBotMentioned(s *discordgo.Session, m *discordgo.Message) bool {
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			return true
		}
	}
	return false
}

func cleanMentions(s *discordgo.Session, m *discordgo.Message) string {
	content := m.Content
	// Remove bot mention from the start of the message
	botMention := fmt.Sprintf("<@%s>", s.State.User.ID)
	botMentionNick := fmt.Sprintf("<@!%s>", s.State.User.ID)
	if len(content) > len(botMention) && content[:len(botMention)] == botMention {
		content = content[len(botMention):]
	} else if len(content) > len(botMentionNick) && content[:len(botMentionNick)] == botMentionNick {
		content = content[len(botMentionNick):]
	}
	// Trim leading/trailing whitespace
	for len(content) > 0 && content[0] == ' ' {
		content = content[1:]
	}
	return content
}
