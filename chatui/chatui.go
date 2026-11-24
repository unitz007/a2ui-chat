// Package chatui turns an A2UI surface into what a chat app can show: text, pictures, a row of buttons, a choice to
// pick from, a box to tick, and fields to fill in a form. Slack, Discord and Telegram each draw these with their own
// widgets (the slack, discord and telegram packages), and what someone does comes back as a Target, applied with
// Handle: a choice or a tick is written into the surface's data model, a button press becomes the action message
// for the agent.
package chatui

import (
	"strings"

	"github.com/unitz007/a2ui-chat/a2ui"
)

// Kind is what a part is.
type Kind string

const (
	Text      Kind = "text"
	Divider   Kind = "divider"
	Image     Kind = "image"
	Buttons   Kind = "buttons"
	Choice    Kind = "choice"
	Toggle    Kind = "toggle"
	Field     Kind = "field"
	CardStart Kind = "card_start"
	CardEnd   Kind = "card_end"
)

// Part is one thing to show, in order.
type Part struct {
	Kind Kind
	// Text: its words as written (Markdown and all); Lines are them read (markdown.go) for a renderer to draw: bold,
	// italic, code, links, quoted lines. Caption is smaller print (a Text's caption variant), Title a heading (h1 to h5)
	// or a tab's title.
	Text    string
	Lines   []Line
	Caption bool
	Title   bool
	// Image: its address and what it shows.
	URL, Alt string
	// Buttons: a row of them.
	Buttons []Button
	// Choice, Toggle and Field: the input.
	Input *Input
}

// Button is a button: what it says, whether it is the main one, and what pressing it does (an action for the agent,
// or opening an address: URL).
type Button struct {
	Target  Target
	Label   string
	Primary bool
	URL     string
}

// Input is a choice, a tick box or a field: its component, label and value now, and, for a choice, its options.
type Input struct {
	Target      Target
	Label       string
	Options     []Option
	Chosen      []string // a choice's chosen values
	Multi       bool     // a choice of several
	On          bool     // a tick box
	Value       string   // a field's value now
	Field       FieldKind
	Placeholder string
	Min, Max    float64 // a slider's range
	Problems    []string
}

// Option is one of a choice's options.
type Option struct{ Label, Value string }

// FieldKind is what a field takes.
type FieldKind string

const (
	ShortText FieldKind = "shortText"
	LongText  FieldKind = "longText"
	Number    FieldKind = "number"
	Obscured  FieldKind = "obscured"
	Date      FieldKind = "date"
	Time      FieldKind = "time"
	DateTime  FieldKind = "dateTime"
	Slider    FieldKind = "slider"
)

// Parts are a surface as a chat app shows it, in order; none while its root hasn't come.
func Parts(s *a2ui.Surface) []Part {
	var out []Part
	walk(s, s.Tree(), &out)
	return merge(out)
}

func target(s *a2ui.Surface, n *a2ui.Node, op Op) Target {
	return Target{Surface: s.ID, Component: n.ID, Base: n.Scope.Base, Index: n.Scope.Index, Op: op}
}

func walk(s *a2ui.Surface, n *a2ui.Node, out *[]Part) {
	if n == nil {
		return
	}
	sc := n.Scope
	p := n.Props
	switch n.Type {
	case "Text":
		if t := strings.TrimSpace(s.String(p["text"], sc)); t != "" {
			v, _ := p["variant"].(string)
			title := len(v) == 2 && v[0] == 'h' && v[1] >= '1' && v[1] <= '5'
			*out = append(*out, Part{Kind: Text, Text: t, Lines: Lines(t), Caption: v == "caption", Title: title})
		}
		return
	case "Image":
		if u := s.String(p["url"], sc); u != "" {
			*out = append(*out, Part{Kind: Image, URL: u, Alt: s.String(p["description"], sc)})
		}
		return
	case "Video", "AudioPlayer":
		if u := s.String(p["url"], sc); u != "" {
			label := s.String(p["description"], sc)
			if label == "" {
				label = map[string]string{"Video": "Watch the video", "AudioPlayer": "Listen"}[n.Type]
			}
			*out = append(*out, Part{Kind: Buttons, Buttons: []Button{{Label: label, URL: u}}})
		}
		return
	case "Icon":
		return
	case "Divider":
		*out = append(*out, Part{Kind: Divider})
		return
	case "Button":
		b := Button{Target: target(s, n, Press), Label: label(s, n), Primary: p["variant"] == "primary", URL: s.LocalURL(n.Component, sc)}
		*out = append(*out, Part{Kind: Buttons, Buttons: []Button{b}})
		return
	case "ChoicePicker":
		in := &Input{Target: target(s, n, Pick), Label: s.String(p["label"], sc), Chosen: s.Strings(p["value"], sc),
			Multi: p["variant"] == "multipleSelection", Problems: s.Checks(n.Component, sc)}
		opts, _ := p["options"].([]any)
		for _, o := range opts {
			m, _ := o.(map[string]any)
			v, _ := m["value"].(string)
			in.Options = append(in.Options, Option{Label: s.String(m["label"], sc), Value: v})
		}
		*out = append(*out, Part{Kind: Choice, Input: in})
		return
	case "CheckBox":
		*out = append(*out, Part{Kind: Toggle, Input: &Input{Target: target(s, n, Tick), Label: s.String(p["label"], sc),
			On: s.Bool(p["value"], sc), Problems: s.Checks(n.Component, sc)}})
		return
	case "TextField", "Slider", "DateTimeInput":
		*out = append(*out, Part{Kind: Field, Input: field(s, n)})
		return
	case "Card":
		*out = append(*out, Part{Kind: CardStart})
		for _, c := range n.Children {
			walk(s, c, out)
		}
		*out = append(*out, Part{Kind: CardEnd})
		return
	case "Tabs":
		for i, c := range n.Children {
			if i < len(n.Tabs) && n.Tabs[i] != "" {
				*out = append(*out, Part{Kind: Text, Text: n.Tabs[i], Lines: Lines(n.Tabs[i]), Title: true})
			}
			walk(s, c, out)
		}
		return
	}
	for _, c := range n.Children { // Row, Column, List, Modal (its trigger, then its content), and anything else
		walk(s, c, out)
	}
}

// label is a button's words: the text of its child.
func label(s *a2ui.Surface, n *a2ui.Node) string {
	var words []string
	for _, c := range n.Children {
		c.Walk(func(x *a2ui.Node) bool {
			if x.Type == "Text" {
				words = append(words, s.String(x.Props["text"], x.Scope))
			}
			return true
		})
	}
	if l := strings.TrimSpace(strings.Join(words, " ")); l != "" {
		return l
	}
	return "OK"
}

func field(s *a2ui.Surface, n *a2ui.Node) *Input {
	sc, p := n.Scope, n.Props
	in := &Input{Target: target(s, n, Fill), Label: s.String(p["label"], sc), Value: s.String(p["value"], sc),
		Placeholder: s.String(p["placeholder"], sc), Problems: s.Checks(n.Component, sc)}
	switch n.Type {
	case "TextField":
		in.Field = ShortText
		if v, ok := p["variant"].(string); ok && v != "" {
			in.Field = FieldKind(v)
		}
	case "Slider":
		in.Field = Slider
		in.Min, _ = s.Number(p["min"], sc)
		in.Max, _ = s.Number(p["max"], sc)
	case "DateTimeInput":
		d, t := s.Bool(p["enableDate"], sc), s.Bool(p["enableTime"], sc)
		switch {
		case d && t:
			in.Field = DateTime
		case t:
			in.Field = Time
		default:
			in.Field = Date
		}
	}
	return in
}

// merge joins buttons next to each other into one row.
func merge(ps []Part) []Part {
	var out []Part
	for _, p := range ps {
		if p.Kind == Buttons && len(out) > 0 && out[len(out)-1].Kind == Buttons {
			out[len(out)-1].Buttons = append(out[len(out)-1].Buttons, p.Buttons...)
			continue
		}
		out = append(out, p)
	}
	return out
}

// Fields are a surface's fields: what a chat app's own form asks for (Slack's and Discord's modals).
func Fields(ps []Part) []*Input {
	var out []*Input
	for _, p := range ps {
		if p.Kind == Field {
			out = append(out, p.Input)
		}
	}
	return out
}

// Summary is a field as one line: its label and value now.
func Summary(in *Input) string {
	v := in.Value
	if in.Field == Obscured && v != "" {
		v = "••••••"
	}
	if v == "" {
		v = "–"
	}
	if in.Label == "" {
		return v
	}
	return in.Label + ": " + v
}
