package chatui

import (
	"reflect"
	"testing"
)

func TestLines(t *testing.T) {
	got := Lines("Fits your **Go** and *Postgres* experience: see [the post](https://lemon.io/x), run `make dev`.\n> Hi Ada,\n> see you soon")
	want := []Line{
		{Spans: []Span{{Text: "Fits your "}, {Text: "Go", Bold: true}, {Text: " and "}, {Text: "Postgres", Italic: true}, {Text: " experience: see "}, {Text: "the post", Link: "https://lemon.io/x"}, {Text: ", run "}, {Text: "make dev", Code: true}, {Text: "."}}},
		{Quote: true, Spans: []Span{{Text: "Hi Ada,"}}},
		{Quote: true, Spans: []Span{{Text: "see you soon"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	if l := Lines("a snake_case_name and 2*3*4"); len(l[0].Spans) != 1 {
		t.Errorf("underscores in a word and lone stars stay as written: %+v", l[0].Spans)
	}
	if l := Lines("see _this_ now"); len(l[0].Spans) != 3 || !l[0].Spans[1].Italic || l[0].Spans[1].Text != "this" || l[0].Spans[2].Text != " now" {
		t.Errorf("_italic_: %+v", l[0].Spans)
	}
	if l := Lines("[bad](javascript:alert(1))"); len(l[0].Spans) != 1 || l[0].Spans[0].Link != "" {
		t.Errorf("only web links: %+v", l[0].Spans)
	}
	if l := Lines(`branch fix\_login\_bug, 2\*3, \[draft\]`); len(l[0].Spans) != 1 || l[0].Spans[0].Text != "branch fix_login_bug, 2*3, [draft]" {
		t.Errorf("backslash escapes: %+v", l[0].Spans)
	}
	if Plain("**Go** [post](https://x.y)\n> quoted") != "Go post\nquoted" {
		t.Errorf("plain: %q", Plain("**Go** [post](https://x.y)\n> quoted"))
	}
}
