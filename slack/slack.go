// Package slack draws A2UI surfaces as Slack Block Kit (https://docs.slack.dev/reference/block-kit/blocks): words as
// sections, buttons and choices in actions blocks (radio buttons, a menu, tick boxes), fields as a summary with an
// Edit… button that opens Slack's own form (a modal) to fill them. It builds what to send; the host sends it
// (chat.postMessage, chat.update, views.open) and hands what Slack posts back to Interaction.
package slack

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/unitz007/a2ui-chat/a2ui"
	"github.com/unitz007/a2ui-chat/chatui"
)

// Slack's limits (Block Kit reference).
const (
	idLimit       = 255 // action_id, block_id, callback_id
	maxButtonText = 75
	maxOptions    = 100 // a static_select's options
	maxRadio      = 10  // radio_buttons' and checkboxes' options
	maxActions    = 25  // elements in an actions block
	maxSection    = 3000
	maxFields     = 10
	maxTitle      = 24 // a modal's title
	maxOptionText = 75
)

// Renderer draws surfaces for Slack.
type Renderer struct {
	// Codec carries targets on buttons and menus (chatui.Inline fits Slack's 255 characters for most surfaces).
	Codec chatui.Codec
}

func (r Renderer) codec() chatui.Codec {
	if r.Codec == nil {
		return chatui.Inline{}
	}
	return r.Codec
}

func (r Renderer) id(t chatui.Target) (string, error) { return r.codec().Encode(t, idLimit) }

// Message is a surface as a message's blocks, and its words as plain text (the message's text, for notifications).
func (r Renderer) Message(s *a2ui.Surface) ([]any, string, error) {
	parts := chatui.Parts(s)
	var blocks []any
	var words []string
	editAdded := false
	for i, p := range parts {
		switch p.Kind {
		case chatui.Text:
			t := escape(p.Text)
			words = append(words, p.Text)
			switch {
			case p.Caption:
				blocks = append(blocks, map[string]any{"type": "context", "elements": []any{mrkdwn(t)}})
			case p.Title:
				blocks = append(blocks, section("*"+t+"*"))
			default:
				blocks = append(blocks, section(t))
			}
		case chatui.Divider:
			blocks = append(blocks, map[string]any{"type": "divider"})
		case chatui.Image:
			alt := p.Alt
			if alt == "" {
				alt = "Picture"
			}
			blocks = append(blocks, map[string]any{"type": "image", "image_url": p.URL, "alt_text": alt})
		case chatui.Buttons:
			els, err := r.buttons(p.Buttons)
			if err != nil {
				return nil, "", err
			}
			blocks = append(blocks, actions(els)...)
		case chatui.Choice:
			b, err := r.choice(p.Input)
			if err != nil {
				return nil, "", err
			}
			if p.Input.Label != "" {
				blocks = append(blocks, section("*"+escape(p.Input.Label)+"*"))
				words = append(words, p.Input.Label)
			}
			blocks = append(blocks, b, problems(p.Input))
		case chatui.Toggle:
			id, err := r.id(p.Input.Target)
			if err != nil {
				return nil, "", err
			}
			opt := map[string]any{"text": plain(clip(p.Input.Label, maxOptionText)), "value": "on"}
			el := map[string]any{"type": "checkboxes", "action_id": id, "options": []any{opt}}
			if p.Input.On {
				el["initial_options"] = []any{opt}
			}
			blocks = append(blocks, map[string]any{"type": "actions", "elements": []any{el}}, problems(p.Input))
		case chatui.Field:
			// Fields in a row are one summary, with one Edit… that opens the form.
			if i > 0 && parts[i-1].Kind == chatui.Field {
				continue
			}
			var fs []any
			for j := i; j < len(parts) && parts[j].Kind == chatui.Field && len(fs) < maxFields; j++ {
				in := parts[j].Input
				label := in.Label
				if label == "" {
					label = "Value"
				}
				fs = append(fs, mrkdwn("*"+escape(label)+"*\n"+escape(valueOf(in))))
				words = append(words, chatui.Summary(in))
			}
			blocks = append(blocks, map[string]any{"type": "section", "fields": fs})
			if !editAdded {
				t := p.Input.Target
				t.Op = chatui.Fill
				id, err := r.id(t)
				if err != nil {
					return nil, "", err
				}
				blocks = append(blocks, map[string]any{"type": "actions", "elements": []any{
					map[string]any{"type": "button", "text": plain("Edit…"), "action_id": id}}})
				editAdded = true
			}
		}
	}
	var out []any
	for _, b := range blocks {
		if m, ok := b.(map[string]any); ok && m != nil {
			out = append(out, b)
		}
	}
	return out, strings.Join(words, "\n"), nil
}

func (r Renderer) buttons(bs []chatui.Button) ([]any, error) {
	var els []any
	for _, b := range bs {
		t := b.Target
		id, err := r.id(t)
		if err != nil {
			return nil, err
		}
		el := map[string]any{"type": "button", "text": plain(clip(b.Label, maxButtonText)), "action_id": id}
		if b.URL != "" {
			el["url"] = b.URL
		} else if b.Primary {
			el["style"] = "primary"
		}
		els = append(els, el)
	}
	return els, nil
}

func (r Renderer) choice(in *chatui.Input) (map[string]any, error) {
	id, err := r.id(in.Target)
	if err != nil {
		return nil, err
	}
	var opts, chosen []any
	for i, o := range in.Options {
		if i == maxOptions {
			break
		}
		opt := map[string]any{"text": plain(clip(o.Label, maxOptionText)), "value": o.Value}
		opts = append(opts, opt)
		for _, c := range in.Chosen {
			if c == o.Value {
				chosen = append(chosen, opt)
			}
		}
	}
	var el map[string]any
	switch {
	case in.Multi && len(opts) <= maxRadio:
		el = map[string]any{"type": "checkboxes", "options": opts}
	case in.Multi:
		el = map[string]any{"type": "multi_static_select", "options": opts, "placeholder": plain("Choose…")}
	case len(opts) <= maxRadio:
		el = map[string]any{"type": "radio_buttons", "options": opts}
	default:
		el = map[string]any{"type": "static_select", "options": opts, "placeholder": plain("Choose…")}
	}
	el["action_id"] = id
	if len(chosen) > 0 {
		if in.Multi {
			el["initial_options"] = chosen
		} else {
			el["initial_option"] = chosen[0]
		}
	}
	return map[string]any{"type": "actions", "elements": []any{el}}, nil
}

// Modal is Slack's own form for a surface's fields, opened with views.open when someone presses Edit….
func (r Renderer) Modal(s *a2ui.Surface, title string) (map[string]any, error) {
	parts := chatui.Parts(s)
	cb, err := r.id(chatui.Target{Surface: s.ID, Op: chatui.Fill})
	if err != nil {
		return nil, err
	}
	var blocks []any
	for _, in := range chatui.Fields(parts) {
		bid, err := r.id(in.Target)
		if err != nil {
			return nil, err
		}
		label := in.Label
		if label == "" {
			label = "Value"
		}
		block := map[string]any{"type": "input", "block_id": bid, "label": plain(clip(label, 2000)), "element": element(in), "optional": true}
		if len(in.Problems) > 0 {
			block["hint"] = plain(strings.Join(in.Problems, " "))
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("slack: %q has nothing to fill in", s.ID)
	}
	if strings.TrimSpace(title) == "" {
		title = "Edit"
	}
	return map[string]any{"type": "modal", "callback_id": cb, "title": plain(clip(title, maxTitle)),
		"submit": plain("Save"), "close": plain("Cancel"), "blocks": blocks}, nil
}

func element(in *chatui.Input) map[string]any {
	switch in.Field {
	case chatui.Number, chatui.Slider:
		el := map[string]any{"type": "number_input", "action_id": "v", "is_decimal_allowed": true}
		if in.Value != "" {
			el["initial_value"] = in.Value
		}
		if in.Field == chatui.Slider {
			el["min_value"], el["max_value"] = a2ui.Text(in.Min), a2ui.Text(in.Max)
		}
		return el
	case chatui.Date:
		el := map[string]any{"type": "datepicker", "action_id": "v"}
		if len(in.Value) >= 10 {
			el["initial_date"] = in.Value[:10]
		}
		return el
	case chatui.Time:
		el := map[string]any{"type": "timepicker", "action_id": "v"}
		if len(in.Value) >= 5 {
			el["initial_time"] = in.Value[:5]
		}
		return el
	case chatui.DateTime:
		el := map[string]any{"type": "datetimepicker", "action_id": "v"}
		if t, err := time.Parse(time.RFC3339, in.Value); err == nil {
			el["initial_date_time"] = t.Unix()
		}
		return el
	}
	el := map[string]any{"type": "plain_text_input", "action_id": "v", "multiline": in.Field == chatui.LongText}
	if in.Value != "" && in.Field != chatui.Obscured {
		el["initial_value"] = in.Value
	}
	if in.Placeholder != "" {
		el["placeholder"] = plain(clip(in.Placeholder, 150))
	}
	return el
}

// Interaction is what Slack posted back (the payload's JSON: block_actions or view_submission): an event on a button,
// a choice or a tick, or a form's values. ok is false for a payload that isn't one of this renderer's.
func (r Renderer) Interaction(payload []byte) (ev *chatui.Event, form *Form, ok bool, err error) {
	var p struct {
		Type      string `json:"type"`
		TriggerID string `json:"trigger_id"`
		Actions   []struct {
			ActionID        string                   `json:"action_id"`
			SelectedOption  *struct{ Value string }  `json:"selected_option"`
			SelectedOptions []struct{ Value string } `json:"selected_options"`
		} `json:"actions"`
		View struct {
			CallbackID string `json:"callback_id"`
			State      struct {
				Values map[string]map[string]struct {
					Type             string `json:"type"`
					Value            string `json:"value"`
					SelectedDate     string `json:"selected_date"`
					SelectedTime     string `json:"selected_time"`
					SelectedDateTime int64  `json:"selected_date_time"`
				} `json:"values"`
			} `json:"state"`
		} `json:"view"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, nil, false, fmt.Errorf("slack: %w", err)
	}
	switch p.Type {
	case "block_actions":
		if len(p.Actions) == 0 {
			return nil, nil, false, nil
		}
		a := p.Actions[0]
		t, ok := r.codec().Decode(a.ActionID)
		if !ok {
			return nil, nil, false, nil
		}
		e := &chatui.Event{Target: t}
		switch {
		case a.SelectedOption != nil:
			e.Value = a.SelectedOption.Value
		case a.SelectedOptions != nil || t.Op == chatui.Tick:
			e.Set = true
			e.Values = []string{}
			for _, o := range a.SelectedOptions {
				e.Values = append(e.Values, o.Value)
			}
		}
		return e, nil, true, nil
	case "view_submission":
		t, ok := r.codec().Decode(p.View.CallbackID)
		if !ok {
			return nil, nil, false, nil
		}
		f := &Form{Surface: t.Surface, Values: map[string]string{}, Scopes: map[string]a2ui.Scope{}, blocks: map[string]string{}}
		for bid, byAction := range p.View.State.Values {
			ft, ok := r.codec().Decode(bid)
			if !ok {
				continue
			}
			v := byAction["v"]
			val := v.Value
			switch v.Type {
			case "datepicker":
				val = v.SelectedDate
			case "timepicker":
				val = v.SelectedTime
			case "datetimepicker":
				if v.SelectedDateTime > 0 {
					val = time.Unix(v.SelectedDateTime, 0).UTC().Format(time.RFC3339)
				}
			}
			f.Values[ft.Component], f.Scopes[ft.Component], f.blocks[ft.Component] = val, ft.Scope(), bid
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
	blocks  map[string]string
}

// Errors is the answer to Slack for a form whose fields still fail their checks (response_action errors), nil when
// none do: Slack shows each under its field and keeps the form open.
func (f *Form) Errors(bad map[string][]string) map[string]any {
	if len(bad) == 0 {
		return nil
	}
	errs := map[string]any{}
	for id, msgs := range bad {
		if b, ok := f.blocks[id]; ok {
			errs[b] = strings.Join(msgs, " ")
		}
	}
	return map[string]any{"response_action": "errors", "errors": errs}
}

func valueOf(in *chatui.Input) string {
	s := chatui.Summary(&chatui.Input{Value: in.Value, Field: in.Field})
	return s
}

func problems(in *chatui.Input) map[string]any {
	if len(in.Problems) == 0 {
		return nil
	}
	return map[string]any{"type": "context", "elements": []any{mrkdwn("⚠️ " + escape(strings.Join(in.Problems, " ")))}}
}

func actions(els []any) []any {
	var out []any
	for len(els) > 0 {
		n := min(len(els), maxActions)
		out = append(out, map[string]any{"type": "actions", "elements": els[:n]})
		els = els[n:]
	}
	return out
}

func section(t string) map[string]any {
	return map[string]any{"type": "section", "text": mrkdwn(clip(t, maxSection))}
}

func mrkdwn(t string) map[string]any { return map[string]any{"type": "mrkdwn", "text": t} }
func plain(t string) map[string]any {
	return map[string]any{"type": "plain_text", "text": t, "emoji": true}
}

// escape keeps words from being read as Slack's markup.
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
