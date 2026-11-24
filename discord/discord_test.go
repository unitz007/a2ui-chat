package discord

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/unitz007/a2ui-chat/chatui"
	"github.com/unitz007/a2ui-chat/internal/fixture"
)

func TestDiscord_CardSelectAndForm(t *testing.T) {
	ss := fixture.Surfaces()
	r := Renderer{Codec: &chatui.Memory{}, Accent: 0xe8b04a}
	s, _ := ss.Get("pick")
	cs, err := r.Message(s)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cs)
	if len(cs) != 1 || !strings.Contains(string(b), `"type":17`) || !strings.Contains(string(b), `"type":3`) || !strings.Contains(string(b), `"accent_color":15249482`) {
		t.Fatalf("one card with a select: %s", b)
	}
	sel := findType(cs, stringSelect)
	ev, _, ok, err := r.Interaction([]byte(`{"type":3,"data":{"custom_id":"` + sel["custom_id"].(string) + `","component_type":3,"values":["17"]}}`))
	if !ok || err != nil {
		t.Fatal(ok, err)
	}
	chatui.HandleEvent(ss, *ev)
	up, _ := r.Update(s)
	ub, _ := json.Marshal(up)
	if !strings.Contains(string(ub), `"default":true,"label":"#17 Health check"`) || !strings.Contains(string(ub), `"type":7`) {
		t.Errorf("drawn again with the pick: %s", ub)
	}
	take := findLabel(cs, "Take this one")
	ev, _, _, _ = r.Interaction([]byte(`{"type":3,"data":{"custom_id":"` + take["custom_id"].(string) + `","component_type":2}}`))
	res, err := chatui.HandleEvent(ss, *ev)
	if err != nil || res.Action.UserMessage != "Take #17" {
		t.Fatalf("%+v %v", res.Action, err)
	}

	tk, _ := ss.Get("ticket")
	m, err := r.Modal(tk, "Adjust the trade")
	if err != nil {
		t.Fatal(err)
	}
	rows := m["components"].([]any)
	stopID := rows[1].(map[string]any)["components"].([]any)[0].(map[string]any)["custom_id"].(string)
	_, form, ok, err := r.Interaction([]byte(`{"type":5,"data":{"custom_id":"` + m["custom_id"].(string) + `","components":[{"type":1,"components":[{"type":4,"custom_id":"` + stopID + `","value":"174.920"}]}]}}`))
	if !ok || err != nil || form.Values["stop"] != "174.920" {
		t.Fatalf("%+v %v", form, err)
	}
	chatui.Submit(ss, form.Surface, form.Values, form.Scopes)
	if tk.Get("/stop") != "174.920" {
		t.Error("written")
	}
}

func findType(cs []any, typ int) map[string]any {
	var found map[string]any
	var walk func(any)
	walk = func(x any) {
		m, ok := x.(map[string]any)
		if !ok || found != nil {
			return
		}
		if m["type"] == typ {
			found = m
			return
		}
		if l, ok := m["components"].([]any); ok {
			for _, c := range l {
				walk(c)
			}
		}
	}
	for _, c := range cs {
		walk(c)
	}
	return found
}

func findLabel(cs []any, label string) map[string]any {
	var found map[string]any
	var walk func(any)
	walk = func(x any) {
		m, ok := x.(map[string]any)
		if !ok || found != nil {
			return
		}
		if m["label"] == label {
			found = m
			return
		}
		if l, ok := m["components"].([]any); ok {
			for _, c := range l {
				walk(c)
			}
		}
	}
	for _, c := range cs {
		walk(c)
	}
	return found
}
