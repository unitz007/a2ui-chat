// Command basic draws an A2UI stream for Slack, Discord and Telegram and prints what each would be sent.
//
//	go run ./examples/basic < stream.jsonl
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/unitz007/a2ui-chat/a2ui"
	"github.com/unitz007/a2ui-chat/chatui"
	"github.com/unitz007/a2ui-chat/discord"
	"github.com/unitz007/a2ui-chat/slack"
	"github.com/unitz007/a2ui-chat/telegram"
)

func main() {
	ss := a2ui.NewSurfaces()
	var ids []string
	err := a2ui.ReadStream(os.Stdin, func(m *a2ui.Message) error {
		id, err := ss.Apply(m)
		if m.CreateSurface != nil {
			ids = append(ids, id)
		}
		return err
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	codec := &chatui.Memory{}
	for _, id := range ids {
		s, ok := ss.Get(id)
		if !ok {
			continue
		}
		blocks, text, _ := slack.Renderer{Codec: codec}.Message(s)
		show("Slack · "+id, map[string]any{"text": text, "blocks": blocks})
		cs, _ := discord.Renderer{Codec: codec}.Message(s)
		show("Discord · "+id, map[string]any{"flags": discord.Flags, "components": cs})
		tg, _ := telegram.Renderer{Codec: codec}.Message(s)
		show("Telegram · "+id, map[string]any{"text": tg.Text, "parse_mode": "HTML", "reply_markup": tg.Markup()})
	}
}

func show(title string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("── %s ──\n%s\n\n", title, b)
}
