package telegram

import (
	"strings"
	"testing"

	"github.com/unitz007/a2ui-chat/chatui"
	"github.com/unitz007/a2ui-chat/internal/fixture"
)

func TestTelegram_ButtonsMarksAndMiniApp(t *testing.T) {
	ss := fixture.Surfaces()
	r := Renderer{Codec: &chatui.Memory{}, MiniApp: func(id string) string { return "https://example.test/card/" + id }}
	s, _ := ss.Get("pick")
	m, err := r.Message(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Text, "<b>Pick one</b>") || len(m.Keyboard) < 3 {
		t.Fatalf("%+v", m)
	}
	for _, row := range m.Keyboard {
		for _, b := range row {
			if len(b.CallbackData) > idLimit {
				t.Errorf("callback data over 64 bytes: %q", b.CallbackData)
			}
		}
	}
	ev, ok := r.Interaction(m.Keyboard[1][0].CallbackData) // #15
	if !ok {
		t.Fatal("not ours")
	}
	chatui.HandleEvent(ss, *ev)
	m, _ = r.Message(s)
	if m.Keyboard[1][0].Text != "● #15 Standardise errors" {
		t.Errorf("the pick is marked: %+v", m.Keyboard)
	}
	last := m.Keyboard[len(m.Keyboard)-1][0]
	ev, _ = r.Interaction(last.CallbackData)
	res, err := chatui.HandleEvent(ss, *ev)
	if err != nil || res.Action.UserMessage != "Take #15" || last.Text != "✅ Take this one" {
		t.Fatalf("%+v %v %q", res.Action, err, last.Text)
	}
	tk, _ := ss.Get("ticket")
	m, _ = r.Message(tk)
	if !strings.Contains(m.Text, "<b>Size (units):</b> 20229") {
		t.Errorf("fields shown: %q", m.Text)
	}
	var web bool
	for _, row := range m.Keyboard {
		for _, b := range row {
			web = web || (b.WebApp != nil && b.WebApp.URL == "https://example.test/card/ticket")
		}
	}
	if !web {
		t.Error("Edit… opens the card inside Telegram")
	}
	r.Group = true
	m, _ = r.Message(tk)
	for _, row := range m.Keyboard {
		for _, b := range row {
			if b.WebApp != nil {
				t.Error("no window buttons in a group")
			}
		}
	}
}
