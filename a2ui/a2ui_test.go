package a2ui

import (
	"strings"
	"testing"
	"time"
)

const taskStream = `{"version":"1.0","createSurface":{"surfaceId":"pick","catalogId":"https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"}}
{"version":"1.0","updateComponents":{"surfaceId":"pick","components":[{"id":"root","component":"Card","child":"col"},{"id":"col","component":"Column","children":["title","tasks","list","take"]},{"id":"title","component":"Text","text":"Beginner backend tasks"},{"id":"tasks","component":"ChoicePicker","label":"Pick one","options":[{"label":"#14 Add input validation","value":"14"},{"label":"#15 Standardise errors","value":"15"}],"value":{"@path":"/chosen"}},{"id":"list","component":"List","children":{"componentId":"row","path":"/tasks"}},{"id":"row","component":"Text","text":{"@call":"formatString","args":{"value":"${@index(offset: 1)}. ${title} (${/repo})"}}},{"id":"take","component":"Button","child":"takeLabel","variant":"primary","action":{"event":{"name":"take_task","userMessage":{"@call":"formatString","args":{"value":"Take #${/chosen/0}"}},"context":{"issue":{"@path":"/chosen"},"repo":"Open-IN/OIN"}}}},{"id":"takeLabel","component":"Text","text":"Take this one"}]}}
{"version":"1.0","updateDataModel":{"surfaceId":"pick","value":{"repo":"Open-IN/OIN","chosen":[],"tasks":[{"title":"Validation"},{"title":"Errors"}]}}}`

func TestStream_TreeTemplatesAndAct(t *testing.T) {
	ss := NewSurfaces()
	err := ReadStream(strings.NewReader(taskStream), func(m *Message) error { _, err := ss.Apply(m); return err })
	if err != nil {
		t.Fatal(err)
	}
	s, _ := ss.Get("pick")
	tree := s.Tree()
	var texts []string
	tree.Walk(func(n *Node) bool {
		if n.Type == "Text" {
			texts = append(texts, s.String(n.Props["text"], n.Scope))
		}
		return true
	})
	want := "Beginner backend tasks|1. Validation (Open-IN/OIN)|2. Errors (Open-IN/OIN)|Take this one"
	if got := strings.Join(texts, "|"); got != want {
		t.Fatalf("texts %q", got)
	}
	if err := s.Write("tasks", Scope{}, "15"); err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return time.Date(2026, 11, 24, 16, 0, 0, 0, time.UTC) }
	a, err := s.Act("take", Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "take_task" || a.UserMessage != "Take #15" || a.Context["repo"] != "Open-IN/OIN" || a.Timestamp != "2026-11-24T16:00:00Z" {
		t.Errorf("action %+v", a)
	}
	if l, _ := a.Context["issue"].([]any); len(l) != 1 || l[0] != "15" {
		t.Errorf("context read now: %+v", a.Context)
	}
	// Creating it twice is an integrity error; deleting it lets it go.
	if _, err := ss.Apply(&Message{Version: Version, CreateSurface: &CreateSurface{SurfaceID: "pick"}}); err == nil {
		t.Error("twice")
	}
	ss.Apply(&Message{Version: Version, DeleteSurface: &DeleteSurface{SurfaceID: "pick"}})
	if _, ok := ss.Get("pick"); ok {
		t.Error("deleted")
	}
}

func TestPointerAndFormats(t *testing.T) {
	s := NewSurface("x")
	s.Set("/a/b~1c/0", "v")
	if s.Get("/a/b~1c/0") != nil { // a map, not a list: "0" is a key here
		if got := s.Get("/a/b~1c/0"); got != "v" {
			t.Errorf("%v", got)
		}
	}
	s.Set("/amount", 1234.5)
	s.Set("/when", "2026-11-24T09:30:00Z")
	sc := Scope{}
	for in, want := range map[string]string{
		`{"@call":"formatCurrency","args":{"value":{"@path":"/amount"},"currency":"EUR"}}`:                                  "€1,234.50",
		`{"@call":"formatNumber","args":{"value":1234567,"grouping":true}}`:                                                 "1,234,567",
		`{"@call":"formatDate","args":{"value":{"@path":"/when"},"format":"EEE d MMM, HH:mm"}}`:                             "Tue 24 Nov, 09:30",
		`{"@call":"formatString","args":{"value":"Pay ${formatCurrency(value:${/amount}, currency:'GBP')} by \\${today}"}}`: "Pay £1,234.50 by ${today}",
	} {
		var v any
		if err := jsonUnmarshal(in, &v); err != nil {
			t.Fatal(err)
		}
		if got := s.String(v, sc); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
	s.Set("/amount", nil)
	if s.Get("/amount") != nil {
		t.Error("null deletes")
	}
}

func TestChecks(t *testing.T) {
	s := NewSurface("f")
	c := Component{ID: "email", Type: "TextField", Props: map[string]any{"value": map[string]any{"@path": "/email"},
		"@checks": []any{map[string]any{"condition": map[string]any{"@call": "email", "args": map[string]any{"value": map[string]any{"@path": "/email"}}}, "message": "Give a valid email."}}}}
	s.Put(c)
	s.Set("/email", "nope")
	if got := s.Checks(c, Scope{}); len(got) != 1 || got[0] != "Give a valid email." {
		t.Errorf("%v", got)
	}
	s.Write("email", Scope{}, "ada@example.com")
	if got := s.Checks(c, Scope{}); len(got) != 0 {
		t.Errorf("%v", got)
	}
}
