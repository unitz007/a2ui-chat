// Package fixture holds the surfaces the renderers' tests share: a task to pick and a trade ticket to adjust.
package fixture

import (
	"strings"

	"github.com/unitz007/a2ui-chat/a2ui"
)

// Stream is a task picker and a trade ticket, as an agent would send them (JSONL).
const Stream = `{"version":"1.0","createSurface":{"surfaceId":"pick","catalogId":"https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"}}
{"version":"1.0","updateComponents":{"surfaceId":"pick","components":[{"id":"root","component":"Card","child":"col"},{"id":"col","component":"Column","children":["title","tasks","take"]},{"id":"title","component":"Text","text":"Beginner backend tasks"},{"id":"tasks","component":"ChoicePicker","label":"Pick one","options":[{"label":"#14 Add input validation","value":"14"},{"label":"#15 Standardise errors","value":"15"},{"label":"#17 Health check","value":"17"}],"value":{"@path":"/chosen"}},{"id":"take","component":"Button","child":"takeLabel","variant":"primary","action":{"event":{"name":"take_task","userMessage":{"@call":"formatString","args":{"value":"Take #${/chosen/0}"}},"context":{"issue":{"@path":"/chosen"}}}}},{"id":"takeLabel","component":"Text","text":"Take this one"}]}}
{"version":"1.0","updateDataModel":{"surfaceId":"pick","value":{"chosen":[]}}}
{"version":"1.0","createSurface":{"surfaceId":"ticket","catalogId":"https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json","dataModel":{"size":"20229","stop":"174.900","confirm":false}}}
{"version":"1.0","updateComponents":{"surfaceId":"ticket","components":[{"id":"root","component":"Card","child":"col"},{"id":"col","component":"Column","children":["head","size","stop","confirm","row"]},{"id":"head","component":"Text","text":"SELL EUR/JPY · limit at 174.814"},{"id":"size","component":"TextField","label":"Size (units)","variant":"number","value":{"@path":"/size"}},{"id":"stop","component":"TextField","label":"Stop","variant":"number","value":{"@path":"/stop"}},{"id":"confirm","component":"CheckBox","label":"Alert me when it fills","value":{"@path":"/confirm"}},{"id":"row","component":"Row","children":["place","later"]},{"id":"place","component":"Button","child":"placeLabel","variant":"primary","action":{"event":{"name":"place_trade","context":{"size":{"@path":"/size"},"stop":{"@path":"/stop"},"alert":{"@path":"/confirm"}}}}},{"id":"placeLabel","component":"Text","text":"Place it"},{"id":"later","component":"Button","child":"laterLabel","action":{"event":{"name":"not_now"}}},{"id":"laterLabel","component":"Text","text":"Not now"}]}}`

// Job is a card about one job with formatted words: a heading, bold, a link, and a quoted description.
const Job = `{"version":"1.0","createSurface":{"surfaceId":"job","catalogId":"https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"}}
{"version":"1.0","updateComponents":{"surfaceId":"job","components":[{"id":"root","component":"Card","child":"col"},{"id":"col","component":"Column","children":["title","line","desc"]},{"id":"title","component":"Text","variant":"h3","text":"Senior Go Engineer"},{"id":"line","component":"Text","text":"Fits your **Go** <experience>: see [the post](https://lemon.io/x?a=1&b=2)."},{"id":"desc","component":"Text","text":"> Build the matching service.\n> Small team."}]}}`

// JobSurface is the Job card's surface.
func JobSurface() *a2ui.Surface {
	ss := a2ui.NewSurfaces()
	if err := a2ui.ReadStream(strings.NewReader(Job), func(m *a2ui.Message) error { _, err := ss.Apply(m); return err }); err != nil {
		panic(err)
	}
	s, _ := ss.Get("job")
	return s
}

// Surfaces are the fixture's surfaces, built from Stream.
func Surfaces() *a2ui.Surfaces {
	ss := a2ui.NewSurfaces()
	if err := a2ui.ReadStream(strings.NewReader(Stream), func(m *a2ui.Message) error { _, err := ss.Apply(m); return err }); err != nil {
		panic(err)
	}
	return ss
}
