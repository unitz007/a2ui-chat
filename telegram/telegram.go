// Package telegram draws A2UI surfaces as a Telegram message (https://core.telegram.org/bots/api): words in its HTML,
// and inline buttons under it: a button for each of a choice's options (the chosen one marked, the message drawn
// again when one is tapped), a tick box that turns on and off, buttons as they are. Telegram has no menus or forms
// in a message, so fields are shown with their values and an Edit… button that opens the card in a window inside
// Telegram (a Mini App, when the host serves one: MiniApp). It builds what to send; the host sends it and hands a
// callback query's data to Interaction.
package telegram

import (
	"html"
	"strings"

	"github.com/unitz007/a2ui-chat/a2ui"
	"github.com/unitz007/a2ui-chat/chatui"
)

// idLimit is callback_data's limit: 64 bytes, too few for most targets carried inline, so a chatui.Memory (or a
// host's own store) is the codec here.
const idLimit = 64

// Message is a surface as Telegram has it: HTML text (parse_mode HTML) and an inline keyboard.
type Message struct {
	Text     string
	Keyboard [][]Button
}

// Button is an inline keyboard button: exactly one of CallbackData, URL or WebApp.
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
	WebApp       *struct {
		URL string `json:"url"`
	} `json:"web_app,omitempty"`
}

// Markup is the message's reply_markup.
func (m Message) Markup() map[string]any {
	if len(m.Keyboard) == 0 {
		return nil
	}
	return map[string]any{"inline_keyboard": m.Keyboard}
}

// Renderer draws surfaces for Telegram.
type Renderer struct {
	Codec chatui.Codec
	// MiniApp is the address of a page that draws a surface for its fields to be filled (a Telegram Mini App), ""
	// when the host has none: then the fields are shown and the agent asks for them in words. Telegram opens such
	// buttons only in one-to-one chats; Group says the chat is a group, where they are left out.
	MiniApp func(surfaceID string) string
	Group   bool
}

func (r Renderer) codec() chatui.Codec {
	if r.Codec == nil {
		return &chatui.Memory{}
	}
	return r.Codec
}

// Message draws a surface.
func (r Renderer) Message(s *a2ui.Surface) (Message, error) {
	var lines []string
	var kb [][]Button
	parts := chatui.Parts(s)
	editAdded := false
	cb := func(t chatui.Target, label string) (Button, error) {
		id, err := r.codec().Encode(t, idLimit)
		return Button{Text: label, CallbackData: id}, err
	}
	for _, p := range parts {
		switch p.Kind {
		case chatui.Text:
			t := html.EscapeString(p.Text)
			switch {
			case p.Title:
				t = "<b>" + t + "</b>"
			case p.Caption:
				t = "<i>" + t + "</i>"
			}
			lines = append(lines, t)
		case chatui.Divider:
			lines = append(lines, "")
		case chatui.Image:
			label := p.Alt
			if label == "" {
				label = "Picture"
			}
			lines = append(lines, `<a href="`+html.EscapeString(p.URL)+`">`+html.EscapeString(label)+"</a>")
		case chatui.Buttons:
			var row []Button
			for _, b := range p.Buttons {
				var btn Button
				if b.URL != "" {
					btn = Button{Text: b.Label, URL: b.URL}
				} else {
					var err error
					if btn, err = cb(b.Target, b.Label); err != nil {
						return Message{}, err
					}
					if b.Primary {
						btn.Text = "✅ " + btn.Text
					}
				}
				row = append(row, btn)
			}
			kb = append(kb, rows(row)...)
		case chatui.Choice:
			if p.Input.Label != "" {
				lines = append(lines, "<b>"+html.EscapeString(p.Input.Label)+"</b>")
			}
			var row []Button
			for _, o := range p.Input.Options {
				t := p.Input.Target
				t.Value = o.Value
				mark := ""
				chosen := contains(p.Input.Chosen, o.Value)
				switch {
				case p.Input.Multi && chosen:
					mark = "☑ "
				case p.Input.Multi:
					mark = "☐ "
				case chosen:
					mark = "● "
				}
				btn, err := cb(t, mark+o.Label)
				if err != nil {
					return Message{}, err
				}
				row = append(row, btn)
			}
			kb = append(kb, rows(row)...)
			if len(p.Input.Problems) > 0 {
				lines = append(lines, "⚠️ "+html.EscapeString(strings.Join(p.Input.Problems, " ")))
			}
		case chatui.Toggle:
			mark := "☐ "
			if p.Input.On {
				mark = "☑ "
			}
			btn, err := cb(p.Input.Target, mark+p.Input.Label)
			if err != nil {
				return Message{}, err
			}
			kb = append(kb, []Button{btn})
		case chatui.Field:
			in := p.Input
			lines = append(lines, "<b>"+html.EscapeString(orValue(in.Label))+":</b> "+html.EscapeString(shown(in)))
			if !editAdded && r.MiniApp != nil && !r.Group {
				if u := r.MiniApp(s.ID); u != "" {
					kb = append(kb, []Button{{Text: "Edit…", WebApp: &struct {
						URL string `json:"url"`
					}{URL: u}}})
					editAdded = true
				}
			}
		}
	}
	return Message{Text: strings.TrimSpace(strings.Join(lines, "\n")), Keyboard: kb}, nil
}

// Interaction is the target of a callback query's data, ok false for one that isn't this renderer's.
func (r Renderer) Interaction(data string) (*chatui.Event, bool) {
	t, ok := r.codec().Decode(data)
	if !ok {
		return nil, false
	}
	return &chatui.Event{Target: t}, true
}

// rows lays buttons out: short labels two to a row, longer ones one to a row.
func rows(bs []Button) [][]Button {
	short := true
	for _, b := range bs {
		if len([]rune(b.Text)) > 22 {
			short = false
		}
	}
	var out [][]Button
	per := 1
	if short {
		per = 2
	}
	for len(bs) > 0 {
		n := min(per, len(bs))
		out = append(out, bs[:n])
		bs = bs[n:]
	}
	return out
}

func contains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func shown(in *chatui.Input) string {
	if in.Field == chatui.Obscured && in.Value != "" {
		return "••••••"
	}
	if in.Value == "" {
		return "–"
	}
	return in.Value
}

func orValue(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Value"
	}
	return s
}
