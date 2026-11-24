// Package discord draws A2UI surfaces with Discord's components (https://discord.com/developers/docs/components):
// a card as a container, words as text displays, buttons and a select menu in action rows, a tick box as a button
// that turns on and off, and fields as a summary with an Edit… button that opens Discord's own form (a modal). It
// uses Discord's newer message layout (components v2: send Flags with the message). It builds what to send; the host
// sends it and hands what Discord posts back to Interaction.
package discord

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unitz007/a2ui-chat/a2ui"
	"github.com/unitz007/a2ui-chat/chatui"
)

// Flags is what a message drawn by Message is sent with: Discord's IS_COMPONENTS_V2.
const Flags = 1 << 15

// Component types and button styles (Discord's numbers).
const (
	actionRow    = 1
	button       = 2
	stringSelect = 3
	textInput    = 4
	textDisplay  = 10
	mediaGallery = 12
	separator    = 14
	container    = 17

	stylePrimary   = 1
	styleSecondary = 2
	styleSuccess   = 3
	styleLink      = 5
)

// Discord's limits.
const (
	idLimit      = 100 // custom_id
	maxLabel     = 80
	maxRow       = 5 // buttons in a row
	maxOptions   = 25
	maxModalRows = 5
	maxTitle     = 45
	maxInputName = 45
	maxText      = 4000
)

// Renderer draws surfaces for Discord.
type Renderer struct {
	// Codec carries targets on buttons and menus; Discord's custom_id takes 100 characters, so a chatui.Memory (or a
	// host's own store) suits most surfaces.
	Codec chatui.Codec
	// Accent is the card's colour (0xRRGGBB), 0 for none.
	Accent int
}

func (r Renderer) codec() chatui.Codec {
	if r.Codec == nil {
		return chatui.Inline{}
	}
	return r.Codec
}

func (r Renderer) id(t chatui.Target) (string, error) { return r.codec().Encode(t, idLimit) }

// Message is a surface as a message's components (send it with Flags, and no content).
func (r Renderer) Message(s *a2ui.Surface) ([]any, error) {
	var top []any
	var card []any // a card's components, while one is open
	inCard := false
	add := func(c ...any) {
		if inCard {
			card = append(card, c...)
		} else {
			top = append(top, c...)
		}
	}
	parts := chatui.Parts(s)
	editAdded := false
	for i, p := range parts {
		switch p.Kind {
		case chatui.CardStart:
			inCard, card = true, nil
		case chatui.CardEnd:
			if len(card) > 0 {
				c := map[string]any{"type": container, "components": card}
				if r.Accent != 0 {
					c["accent_color"] = r.Accent
				}
				top = append(top, c)
			}
			inCard, card = false, nil
		case chatui.Text:
			t := markdownOf(p.Lines, p.Title)
			if p.Caption {
				t = "-# " + strings.ReplaceAll(t, "\n", "\n-# ")
			}
			add(text(t))
		case chatui.Divider:
			add(map[string]any{"type": separator})
		case chatui.Image:
			add(map[string]any{"type": mediaGallery, "items": []any{map[string]any{"media": map[string]any{"url": p.URL}, "description": clip(p.Alt, 1024)}}})
		case chatui.Buttons:
			rows, err := r.buttonRows(p.Buttons)
			if err != nil {
				return nil, err
			}
			add(rows...)
		case chatui.Choice:
			c, err := r.choice(p.Input)
			if err != nil {
				return nil, err
			}
			if p.Input.Label != "" {
				add(text("**" + escape(p.Input.Label) + "**"))
			}
			add(c)
			if len(p.Input.Problems) > 0 {
				add(text("-# ⚠️ " + escape(strings.Join(p.Input.Problems, " "))))
			}
		case chatui.Toggle:
			id, err := r.id(p.Input.Target)
			if err != nil {
				return nil, err
			}
			mark := "☐ "
			if p.Input.On {
				mark = "☑ "
			}
			add(row(map[string]any{"type": button, "style": styleSecondary, "label": clip(mark+p.Input.Label, maxLabel), "custom_id": id}))
		case chatui.Field:
			if i > 0 && parts[i-1].Kind == chatui.Field {
				continue
			}
			var lines []string
			for j := i; j < len(parts) && parts[j].Kind == chatui.Field; j++ {
				in := parts[j].Input
				lines = append(lines, "**"+escape(orValue(in.Label))+":** "+escape(shown(in)))
			}
			add(text(strings.Join(lines, "\n")))
			if !editAdded {
				t := p.Input.Target
				t.Op = chatui.Fill
				id, err := r.id(t)
				if err != nil {
					return nil, err
				}
				add(row(map[string]any{"type": button, "style": styleSecondary, "label": "Edit…", "custom_id": id}))
				editAdded = true
			}
		}
	}
	if inCard && len(card) > 0 { // a card left open
		top = append(top, map[string]any{"type": container, "components": card})
	}
	return top, nil
}

func (r Renderer) buttonRows(bs []chatui.Button) ([]any, error) {
	var rows []any
	var cur []any
	for _, b := range bs {
		el := map[string]any{"type": button, "label": clip(b.Label, maxLabel)}
		switch {
		case b.URL != "":
			el["style"], el["url"] = styleLink, b.URL
		default:
			id, err := r.id(b.Target)
			if err != nil {
				return nil, err
			}
			el["custom_id"] = id
			el["style"] = styleSecondary
			if b.Primary {
				el["style"] = stylePrimary
			}
		}
		cur = append(cur, el)
		if len(cur) == maxRow {
			rows, cur = append(rows, row(cur...)), nil
		}
	}
	if len(cur) > 0 {
		rows = append(rows, row(cur...))
	}
	return rows, nil
}

func (r Renderer) choice(in *chatui.Input) (map[string]any, error) {
	id, err := r.id(in.Target)
	if err != nil {
		return nil, err
	}
	var opts []any
	for i, o := range in.Options {
		if i == maxOptions {
			break
		}
		opt := map[string]any{"label": clip(o.Label, 100), "value": clip(o.Value, 100)}
		for _, c := range in.Chosen {
			if c == o.Value {
				opt["default"] = true
			}
		}
		opts = append(opts, opt)
	}
	sel := map[string]any{"type": stringSelect, "custom_id": id, "options": opts, "placeholder": clip(orValue2(in.Label, "Choose…"), 150)}
	if in.Multi {
		sel["min_values"], sel["max_values"] = 0, len(opts)
	}
	return row(sel), nil
}

// Modal is Discord's own form for a surface's fields (its first five), answered to Discord as interaction response
// type 9 when someone presses Edit….
func (r Renderer) Modal(s *a2ui.Surface, title string) (map[string]any, error) {
	cb, err := r.id(chatui.Target{Surface: s.ID, Op: chatui.Fill})
	if err != nil {
		return nil, err
	}
	var rows []any
	for _, in := range chatui.Fields(chatui.Parts(s)) {
		if len(rows) == maxModalRows {
			break
		}
		id, err := r.id(in.Target)
		if err != nil {
			return nil, err
		}
		style := 1
		if in.Field == chatui.LongText {
			style = 2
		}
		ti := map[string]any{"type": textInput, "custom_id": id, "style": style, "label": clip(orValue(in.Label), maxInputName), "required": false}
		if in.Value != "" && in.Field != chatui.Obscured {
			ti["value"] = clip(in.Value, 4000)
		}
		if ph := placeholder(in); ph != "" {
			ti["placeholder"] = clip(ph, 100)
		}
		rows = append(rows, row(ti))
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("discord: %q has nothing to fill in", s.ID)
	}
	if strings.TrimSpace(title) == "" {
		title = "Edit"
	}
	return map[string]any{"custom_id": cb, "title": clip(title, maxTitle), "components": rows}, nil
}

func placeholder(in *chatui.Input) string {
	switch in.Field {
	case chatui.Date:
		return "YYYY-MM-DD"
	case chatui.Time:
		return "HH:MM"
	case chatui.DateTime:
		return "YYYY-MM-DDTHH:MM:SSZ"
	case chatui.Slider:
		return fmt.Sprintf("A number from %s to %s", a2ui.Text(in.Min), a2ui.Text(in.Max))
	}
	return in.Placeholder
}

// Interaction is what Discord posted back (an interaction: a message component, type 3, or a modal submit, type 5):
// an event on a button, a menu or a tick, or a form's values. ok is false for one that isn't this renderer's.
func (r Renderer) Interaction(payload []byte) (ev *chatui.Event, form *Form, ok bool, err error) {
	var p struct {
		Type int `json:"type"`
		Data struct {
			CustomID      string   `json:"custom_id"`
			ComponentType int      `json:"component_type"`
			Values        []string `json:"values"`
			Components    []struct {
				Components []struct {
					CustomID string `json:"custom_id"`
					Value    string `json:"value"`
				} `json:"components"`
				Component *struct { // a modal's newer label layout
					CustomID string `json:"custom_id"`
					Value    string `json:"value"`
				} `json:"component"`
			} `json:"components"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, nil, false, fmt.Errorf("discord: %w", err)
	}
	t, ok := r.codec().Decode(p.Data.CustomID)
	if !ok {
		return nil, nil, false, nil
	}
	switch p.Type {
	case 3:
		e := &chatui.Event{Target: t}
		if p.Data.ComponentType == stringSelect {
			e.Set, e.Values = true, append([]string{}, p.Data.Values...)
		}
		return e, nil, true, nil
	case 5:
		f := &Form{Surface: t.Surface, Values: map[string]string{}, Scopes: map[string]a2ui.Scope{}}
		take := func(id, v string) {
			if ft, ok := r.codec().Decode(id); ok {
				f.Values[ft.Component], f.Scopes[ft.Component] = v, ft.Scope()
			}
		}
		for _, row := range p.Data.Components {
			for _, c := range row.Components {
				take(c.CustomID, c.Value)
			}
			if row.Component != nil {
				take(row.Component.CustomID, row.Component.Value)
			}
		}
		return nil, f, true, nil
	}
	return nil, nil, false, nil
}

// Form is a submitted modal's values, by component.
type Form struct {
	Surface string
	Values  map[string]string
	Scopes  map[string]a2ui.Scope
}

// Update is the interaction response that draws a surface again in the message acted on (type 7), after a choice or
// a tick.
func (r Renderer) Update(s *a2ui.Surface) (map[string]any, error) {
	cs, err := r.Message(s)
	if err != nil {
		return nil, err
	}
	return map[string]any{"type": 7, "data": map[string]any{"components": cs, "flags": Flags}}, nil
}

func text(t string) map[string]any {
	return map[string]any{"type": textDisplay, "content": clip(t, maxText)}
}
func row(c ...any) map[string]any { return map[string]any{"type": actionRow, "components": c} }

func shown(in *chatui.Input) string {
	v := in.Value
	if in.Field == chatui.Obscured && v != "" {
		return "••••••"
	}
	if v == "" {
		return "–"
	}
	return v
}

func orValue(label string) string { return orValue2(label, "Value") }
func orValue2(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

// escape keeps words from being read as Discord's markdown.
var escape = strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "~", `\~`, "`", "\\`", "|", `\|`, ">", `\>`, "#", `\#`).Replace

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// markdownOf draws a Text's lines in Discord's markdown: **bold**, *italic*, `code`, [words](link), "> " for quoted
// lines; all of it bold for a heading.
func markdownOf(lines []chatui.Line, heading bool) string {
	var b strings.Builder
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if l.Quote {
			b.WriteString("> ")
		}
		for _, sp := range l.Spans {
			var t string
			switch {
			case sp.Code:
				t = "`" + strings.ReplaceAll(sp.Text, "`", "'") + "`"
			case sp.Link != "":
				t = "[" + escape(sp.Text) + "](" + strings.NewReplacer("(", "%28", ")", "%29").Replace(sp.Link) + ")"
			default:
				t = escape(sp.Text)
			}
			if sp.Italic {
				t = "*" + t + "*"
			}
			if (sp.Bold || heading) && strings.TrimSpace(t) != "" {
				t = "**" + t + "**"
			}
			b.WriteString(t)
		}
	}
	return b.String()
}
