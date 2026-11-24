package slack

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/unitz007/a2ui-chat/chatui"
	"github.com/unitz007/a2ui-chat/internal/fixture"
)

func TestSlack_PickThenTake(t *testing.T) {
	ss := fixture.Surfaces()
	s, _ := ss.Get("pick")
	r := Renderer{}
	blocks, words, err := r.Message(s)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(blocks)
	for _, want := range []string{`"radio_buttons"`, `"style":"primary"`, "Beginner backend tasks", "#15 Standardise errors"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %s in %s", want, b)
		}
	}
	if !strings.Contains(words, "Beginner backend tasks") {
		t.Errorf("words for notifications: %q", words)
	}
	radio, take := find(blocks, "radio_buttons"), find(blocks, "button")
	ev, _, ok, err := r.Interaction([]byte(`{"type":"block_actions","actions":[{"action_id":"` + radio["action_id"].(string) + `","selected_option":{"value":"15"}}]}`))
	if !ok || err != nil {
		t.Fatal(ok, err)
	}
	if res, err := chatui.HandleEvent(ss, *ev); err != nil || !res.Redraw {
		t.Fatal(res, err)
	}
	ev, _, _, _ = r.Interaction([]byte(`{"type":"block_actions","actions":[{"action_id":"` + take["action_id"].(string) + `"}]}`))
	res, err := chatui.HandleEvent(ss, *ev)
	if err != nil || res.Action == nil || res.Action.Name != "take_task" || res.Action.UserMessage != "Take #15" {
		t.Fatalf("pressed: %+v %v", res.Action, err)
	}
	// Drawn again, the choice shows what was picked.
	blocks, _, _ = r.Message(s)
	if sel := find(blocks, "radio_buttons")["initial_option"].(map[string]any); sel["value"] != "15" {
		t.Errorf("picked: %v", sel)
	}
}

func TestSlack_TicketFormAndTick(t *testing.T) {
	ss := fixture.Surfaces()
	s, _ := ss.Get("ticket")
	r := Renderer{}
	blocks, _, err := r.Message(s)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(blocks)
	if !strings.Contains(string(b), `Size (units)*\n20229`) || !strings.Contains(string(b), `"Edit…"`) || !strings.Contains(string(b), `"checkboxes"`) {
		t.Fatalf("summary, edit and tick: %s", b)
	}
	modal, err := r.Modal(s, "Adjust the trade")
	if err != nil {
		t.Fatal(err)
	}
	mb, _ := json.Marshal(modal)
	if !strings.Contains(string(mb), `"number_input"`) || !strings.Contains(string(mb), `"initial_value":"174.900"`) {
		t.Fatalf("the form: %s", mb)
	}
	in := modal["blocks"].([]any)
	sizeBlock := in[0].(map[string]any)["block_id"].(string)
	payload := `{"type":"view_submission","view":{"callback_id":"` + modal["callback_id"].(string) + `","state":{"values":{"` + sizeBlock + `":{"v":{"type":"number_input","value":"15000"}}}}}}`
	_, form, ok, err := r.Interaction([]byte(payload))
	if !ok || err != nil || form.Values["size"] != "15000" {
		t.Fatalf("submitted: %+v %v", form, err)
	}
	if bad, err := chatui.Submit(ss, form.Surface, form.Values, form.Scopes); err != nil || len(bad) != 0 || form.Errors(bad) != nil {
		t.Fatal(bad, err)
	}
	tick := find(blocks, "checkboxes")
	ev, _, _, _ := r.Interaction([]byte(`{"type":"block_actions","actions":[{"action_id":"` + tick["action_id"].(string) + `","selected_options":[{"value":"on"}]}]}`))
	chatui.HandleEvent(ss, *ev)
	place := findLabel(blocks, "Place it")
	ev, _, _, _ = r.Interaction([]byte(`{"type":"block_actions","actions":[{"action_id":"` + place["action_id"].(string) + `"}]}`))
	res, err := chatui.HandleEvent(ss, *ev)
	if err != nil || res.Action.Context["size"] != "15000" || res.Action.Context["alert"] != true {
		t.Fatalf("placed with what was entered: %+v %v", res.Action, err)
	}
}

func find(blocks []any, typ string) map[string]any {
	for _, b := range blocks {
		m := b.(map[string]any)
		els, _ := m["elements"].([]any)
		for _, e := range els {
			if em, ok := e.(map[string]any); ok && em["type"] == typ {
				return em
			}
		}
	}
	return nil
}

func findLabel(blocks []any, label string) map[string]any {
	for _, b := range blocks {
		els, _ := b.(map[string]any)["elements"].([]any)
		for _, e := range els {
			em, _ := e.(map[string]any)
			if txt, ok := em["text"].(map[string]any); ok && txt["text"] == label {
				return em
			}
		}
	}
	return nil
}
