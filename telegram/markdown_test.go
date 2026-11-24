package telegram

import (
	"strings"
	"testing"

	"github.com/unitz007/a2ui-chat/chatui"
	"github.com/unitz007/a2ui-chat/internal/fixture"
)

func TestMessage_FormattedWords(t *testing.T) {
	m, err := Renderer{Codec: &chatui.Memory{}}.Message(fixture.JobSurface())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<b>Senior Go Engineer</b>", `Fits your <b>Go</b> &lt;experience&gt;: see <a href="https://lemon.io/x?a=1&amp;b=2">the post</a>.`, "<blockquote>Build the matching service.\nSmall team.</blockquote>"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("missing %s in %s", want, m.Text)
		}
	}
}
