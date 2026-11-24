package chatui

import (
	"testing"

	"github.com/unitz007/a2ui-chat/internal/fixture"
)

func TestParts(t *testing.T) {
	ss := fixture.Surfaces()
	s, _ := ss.Get("ticket")
	var kinds []Kind
	for _, p := range Parts(s) {
		kinds = append(kinds, p.Kind)
	}
	want := []Kind{CardStart, Text, Field, Field, Toggle, Buttons, CardEnd}
	if len(kinds) != len(want) {
		t.Fatalf("%v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("%v", kinds)
		}
	}
	if got := Parts(s)[5].Buttons; len(got) != 2 || !got[0].Primary || got[1].Label != "Not now" {
		t.Errorf("one row of two: %+v", got)
	}
}

func TestCodecs(t *testing.T) {
	tg := Target{Surface: "a-fairly-long-surface-id-for-a-ticket", Component: "place", Op: Press}
	if _, err := (Inline{}).Encode(tg, 64); err == nil {
		t.Error("inline is too long for 64 bytes")
	}
	m := &Memory{}
	id, err := m.Encode(tg, 64)
	if err != nil || len(id) > 64 {
		t.Fatal(id, err)
	}
	if again, _ := m.Encode(tg, 64); again != id {
		t.Error("the same target, the same id")
	}
	if got, ok := m.Decode(id); !ok || got != tg {
		t.Error("decoded")
	}
	in, _ := (Inline{}).Encode(tg, 255)
	if got, ok := (Inline{}).Decode(in); !ok || got != tg {
		t.Error("inline round trip")
	}
}
