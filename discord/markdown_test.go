package discord

import (
	"fmt"
	"strings"
	"testing"

	"github.com/unitz007/a2ui-chat/chatui"
	"github.com/unitz007/a2ui-chat/internal/fixture"
)

func TestMessage_FormattedWords(t *testing.T) {
	cs, err := Renderer{Codec: chatui.Inline{}}.Message(fixture.JobSurface())
	if err != nil {
		t.Fatal(err)
	}
	s := fmt.Sprint(cs)
	for _, want := range []string{"**Senior Go Engineer**", `Fits your **Go** <experience\>: see [the post](https://lemon.io/x?a=1&b=2).`, "> Build the matching service.\n> Small team."} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
}
