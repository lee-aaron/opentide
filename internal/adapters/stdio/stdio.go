// Package stdio implements a CLI/stdio adapter for demo mode and local development.
// Messages are read from stdin and responses written to stdout.
package stdio

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/opentide/opentide/internal/adapters"
)

type Adapter struct {
	messages chan adapters.IncomingMessage
	msgID    atomic.Int64
}

func New() *Adapter {
	return &Adapter{
		messages: make(chan adapters.IncomingMessage, 16),
	}
}

func (a *Adapter) Connect(ctx context.Context) error {
	go a.readLoop(ctx)
	return nil
}

func (a *Adapter) readLoop(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("OpenTide Demo Mode (type a message, press Enter)")
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Print("> ")

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		text := scanner.Text()
		if text == "" {
			fmt.Print("> ")
			continue
		}

		id := a.msgID.Add(1)
		a.messages <- adapters.IncomingMessage{
			Platform:  adapters.PlatformStdio,
			ChannelID: "stdio",
			UserID:    "local",
			MessageID: fmt.Sprintf("stdio-%d", id),
			Content:   text,
		}
	}
}

func (a *Adapter) SendMessage(_ context.Context, _ string, msg adapters.Message) error {
	fmt.Printf("\n%s\n", msg.Content)
	if len(msg.Buttons) > 0 {
		for i, b := range msg.Buttons {
			fmt.Printf("  [%d] %s\n", i+1, b.Label)
		}
	}
	fmt.Print("> ")
	return nil
}

func (a *Adapter) ReceiveMessages(_ context.Context) (<-chan adapters.IncomingMessage, error) {
	return a.messages, nil
}

func (a *Adapter) Platform() adapters.Platform {
	return adapters.PlatformStdio
}

func (a *Adapter) Close() error {
	close(a.messages)
	return nil
}
