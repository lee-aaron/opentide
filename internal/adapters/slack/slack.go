// Package slack implements the Slack messaging adapter using Socket Mode.
// Socket Mode connects via WebSocket, so no public HTTP endpoint is needed.
// This makes it suitable for local dev and DO App Platform deployments.
package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/opentide/opentide/internal/adapters"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Adapter implements adapters.Adapter for Slack.
type Adapter struct {
	botToken    string
	appToken    string
	client      *slack.Client
	socket      *socketmode.Client
	messages    chan adapters.IncomingMessage
	logger      *slog.Logger
	botUserID   string
}

// New creates a Slack adapter.
// botToken is the Bot User OAuth Token (xoxb-...).
// appToken is the App-Level Token (xapp-...) for Socket Mode.
func New(botToken, appToken string, logger *slog.Logger) (*Adapter, error) {
	if botToken == "" {
		return nil, fmt.Errorf("SLACK_BOT_TOKEN is required")
	}
	if appToken == "" {
		return nil, fmt.Errorf("SLACK_APP_TOKEN is required for Socket Mode")
	}

	client := slack.New(botToken,
		slack.OptionAppLevelToken(appToken),
	)

	socket := socketmode.New(client)

	return &Adapter{
		botToken: botToken,
		appToken: appToken,
		client:   client,
		socket:   socket,
		messages: make(chan adapters.IncomingMessage, 100),
		logger:   logger,
	}, nil
}

func (a *Adapter) Connect(ctx context.Context) error {
	// Get bot's own user ID for mention detection
	authResp, err := a.client.AuthTestContext(ctx)
	if err != nil {
		return fmt.Errorf("slack auth test failed: %w", err)
	}
	a.botUserID = authResp.UserID
	a.logger.Info("slack connected", "bot_user", a.botUserID, "team", authResp.Team)

	// Start Socket Mode event loop in background
	go a.eventLoop(ctx)
	go a.socket.RunContext(ctx)

	return nil
}

func (a *Adapter) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-a.socket.Events:
			if !ok {
				return
			}
			a.handleEvent(evt)
		}
	}
}

func (a *Adapter) handleEvent(evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		a.socket.Ack(*evt.Request)
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		a.handleEventsAPI(eventsAPIEvent)

	case socketmode.EventTypeSlashCommand:
		a.socket.Ack(*evt.Request)
		cmd, ok := evt.Data.(slack.SlashCommand)
		if !ok {
			return
		}
		a.messages <- adapters.IncomingMessage{
			Platform:  adapters.PlatformSlack,
			ChannelID: cmd.ChannelID,
			UserID:    cmd.UserID,
			MessageID: cmd.TriggerID,
			Content:   cmd.Text,
		}

	case socketmode.EventTypeInteractive:
		a.socket.Ack(*evt.Request)
		callback, ok := evt.Data.(slack.InteractionCallback)
		if !ok {
			return
		}
		a.handleInteraction(callback)
	}
}

func (a *Adapter) handleEventsAPI(evt slackevents.EventsAPIEvent) {
	switch evt.Type {
	case "event_callback":
		innerEvent := evt.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			// Ignore bot's own messages
			if ev.User == a.botUserID || ev.BotID != "" {
				return
			}
			// Only respond to DMs or mentions
			content := ev.Text
			isDM := strings.HasPrefix(ev.Channel, "D") // DM channels start with D
			isMention := strings.Contains(content, fmt.Sprintf("<@%s>", a.botUserID))

			if !isDM && !isMention {
				return
			}

			// Strip bot mention from content
			content = strings.ReplaceAll(content, fmt.Sprintf("<@%s>", a.botUserID), "")
			content = strings.TrimSpace(content)

			if content == "" {
				return
			}

			a.messages <- adapters.IncomingMessage{
				Platform:  adapters.PlatformSlack,
				ChannelID: ev.Channel,
				UserID:    ev.User,
				MessageID: ev.ClientMsgID,
				Content:   content,
			}

		case *slackevents.AppMentionEvent:
			if ev.User == a.botUserID {
				return
			}
			content := strings.ReplaceAll(ev.Text, fmt.Sprintf("<@%s>", a.botUserID), "")
			content = strings.TrimSpace(content)
			if content == "" {
				return
			}

			a.messages <- adapters.IncomingMessage{
				Platform:  adapters.PlatformSlack,
				ChannelID: ev.Channel,
				UserID:    ev.User,
				MessageID: ev.EventTimeStamp,
				Content:   content,
			}
		}
	}
}

func (a *Adapter) handleInteraction(callback slack.InteractionCallback) {
	for _, action := range callback.ActionCallback.BlockActions {
		a.messages <- adapters.IncomingMessage{
			Platform:  adapters.PlatformSlack,
			ChannelID: callback.Channel.ID,
			UserID:    callback.User.ID,
			MessageID: callback.TriggerID,
			Content:   fmt.Sprintf("__approval_response:%s", action.ActionID),
		}
	}
}

func (a *Adapter) SendMessage(ctx context.Context, channelID string, msg adapters.Message) error {
	if len(msg.Buttons) > 0 {
		return a.sendWithButtons(ctx, channelID, msg)
	}

	// Split long messages (Slack limit: 4000 chars per message)
	content := msg.Content
	for len(content) > 0 {
		chunk := content
		if len(chunk) > 3900 {
			// Find a good split point
			idx := strings.LastIndex(chunk[:3900], "\n")
			if idx < 100 {
				idx = 3900
			}
			chunk = content[:idx]
			content = content[idx:]
		} else {
			content = ""
		}

		_, _, err := a.client.PostMessageContext(ctx, channelID,
			slack.MsgOptionText(chunk, false),
		)
		if err != nil {
			return fmt.Errorf("slack send message: %w", err)
		}
	}
	return nil
}

func (a *Adapter) sendWithButtons(ctx context.Context, channelID string, msg adapters.Message) error {
	var buttons []slack.BlockElement
	for _, btn := range msg.Buttons {
		style := slack.StyleDefault
		if btn.Style == "approve" {
			style = slack.StylePrimary
		} else if btn.Style == "deny" {
			style = slack.StyleDanger
		}

		element := slack.NewButtonBlockElement(btn.ActionID, btn.ActionID,
			slack.NewTextBlockObject("plain_text", btn.Label, false, false),
		)
		element.Style = style
		buttons = append(buttons, element)
	}

	blocks := slack.MsgOptionBlocks(
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", msg.Content, false, false),
			nil, nil,
		),
		slack.NewActionBlock("approval_actions", buttons...),
	)

	_, _, err := a.client.PostMessageContext(ctx, channelID, blocks)
	if err != nil {
		return fmt.Errorf("slack send buttons: %w", err)
	}
	return nil
}

func (a *Adapter) ReceiveMessages(_ context.Context) (<-chan adapters.IncomingMessage, error) {
	return a.messages, nil
}

func (a *Adapter) Platform() adapters.Platform {
	return adapters.PlatformSlack
}

func (a *Adapter) Close() error {
	close(a.messages)
	return nil
}
