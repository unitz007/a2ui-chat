package slack

import (
	"fmt"
	"strings"
	"testing"

	"github.com/unitz007/a2ui-chat/internal/fixture"
)

func TestMessage_FormattedWords(t *testing.T) {
	blocks, text, err := Renderer{}.Message(fixture.JobSurface())
	if err != nil {
		t.Fatal(err)
	}
	s := fmt.Sprint(blocks)
	for _, want := range []string{"*Senior Go Engineer*", "Fits your *Go* &lt;experience&gt;: see <https://lemon.io/x?a=1&b=2|the post>.", "> Build the matching service.\n> Small team."} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if !strings.Contains(text, "Fits your Go <experience>: see the post.") {
		t.Errorf("plain words: %q", text)
	}
}
