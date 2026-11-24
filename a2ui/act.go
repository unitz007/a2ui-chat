package a2ui

import (
	"errors"
	"fmt"
	"time"
)

// ErrLocal is what Act says for a component whose action runs on the renderer (a function call such as openUrl), not
// at the agent: the renderer does it itself (LocalURL), and nothing goes to the agent.
var ErrLocal = errors.New("a2ui: a local action, not one for the agent")

// Write writes what someone entered into an input's bound path (two-way binding): a TextField's or CheckBox's
// value, a ChoicePicker's chosen values, a Slider's number. A value that isn't bound to the data model is left alone
// (there is nowhere to keep it).
func (s *Surface) Write(componentID string, sc Scope, value any) error {
	c, ok := s.Component(componentID)
	if !ok {
		return fmt.Errorf("a2ui: no component %q on %q", componentID, s.ID)
	}
	p, ok := BindingPath(c.Props["value"])
	if !ok {
		return nil
	}
	switch c.Type {
	case "ChoicePicker":
		switch x := value.(type) {
		case string:
			value = []any{x}
		case []string:
			l := make([]any, len(x))
			for i, e := range x {
				l[i] = e
			}
			value = l
		}
	case "Slider":
		if f, ok := number(value); ok {
			value = f
		}
	case "CheckBox":
		if b, ok := value.(string); ok {
			value = b == "true"
		}
	}
	return s.Set(sc.abs(p), value)
}

// Act is the action message for someone acting on a component (a Button, or anything with an action), its context
// read from the data model now; ErrLocal for an action the renderer runs itself.
func (s *Surface) Act(componentID string, sc Scope) (*ActionMessage, error) {
	c, ok := s.Component(componentID)
	if !ok {
		return nil, fmt.Errorf("a2ui: no component %q on %q", componentID, s.ID)
	}
	action, _ := c.Props["action"].(map[string]any)
	if action == nil {
		return nil, fmt.Errorf("a2ui: %q has no action", componentID)
	}
	if _, local := action["functionCall"]; local {
		return nil, ErrLocal
	}
	ev, _ := action["event"].(map[string]any)
	name, _ := ev["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("a2ui: %q's action has no event name", componentID)
	}
	ctx := map[string]any{}
	if raw, ok := ev["context"].(map[string]any); ok {
		for k, v := range raw {
			ctx[k] = s.Resolve(v, sc)
		}
	}
	return &ActionMessage{Name: name, SurfaceID: s.ID, SourceComponentID: componentID, Timestamp: now().Format(time.RFC3339),
		Context: ctx, UserMessage: s.String(ev["userMessage"], sc)}, nil
}

// LocalURL is the address a component's local openUrl action opens, "" when it has none: a chat app shows it as a
// link button.
func (s *Surface) LocalURL(c Component, sc Scope) string {
	action, _ := c.Props["action"].(map[string]any)
	fc, _ := action["functionCall"].(map[string]any)
	if fc == nil || fc["@call"] != "openUrl" {
		return ""
	}
	args, _ := fc["args"].(map[string]any)
	return s.String(args["url"], sc)
}

// Checks are what a Checkable component's @checks say is wrong with it now (their messages), none when all hold.
func (s *Surface) Checks(c Component, sc Scope) []string {
	rules, _ := c.Props["@checks"].([]any)
	var bad []string
	for _, r := range rules {
		rule, _ := r.(map[string]any)
		if rule == nil {
			continue
		}
		ok := true
		switch v := s.Resolve(rule["condition"], sc).(type) {
		case bool:
			ok = v
		case map[string]any:
			ok, _ = v["valid"].(bool)
		}
		if !ok {
			msg, _ := rule["message"].(string)
			if msg == "" {
				msg = "This isn't valid."
			}
			bad = append(bad, msg)
		}
	}
	return bad
}
