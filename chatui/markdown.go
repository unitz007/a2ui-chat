package chatui

import (
	"regexp"
	"strings"
)

// A Text's words may carry a little Markdown (A2UI's Text allows it): **bold**, *italic* or _italic_, `code`,
// [words](https://link), and lines starting "> " for quoted text (an email's body, an issue's description). Each
// renderer draws these in its app's own way; anything else is shown as written.

// Line is one line of a Text: quoted or not, and its pieces.
type Line struct {
	Quote bool
	Spans []Span
}

// Span is a run of words with one look.
type Span struct {
	Text         string
	Bold, Italic bool
	Code         bool
	Link         string // https or http only
}

var inline = regexp.MustCompile("\\*\\*([^*\\n]+?)\\*\\*|`([^`\\n]+)`|\\[([^\\]\\n]+)\\]\\((https?://[^)\\s]+)\\)|(?:^|[\\s(])\\*([^*\\s][^*\\n]*?)\\*(?:$|[\\s.,;:!?)])|(?:^|[\\s(])_([^_\\s][^_\\n]*?)_(?:$|[\\s.,;:!?)])")

// Lines reads a Text's words into lines of pieces.
func Lines(text string) []Line {
	var out []Line
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		l := Line{}
		if q, ok := strings.CutPrefix(raw, "> "); ok {
			l.Quote, raw = true, q
		} else if raw == ">" {
			l.Quote, raw = true, ""
		}
		l.Spans = spans(raw)
		out = append(out, l)
	}
	return out
}

func spans(s string) []Span {
	var out []Span
	plain := func(t string) {
		if t != "" {
			out = append(out, Span{Text: t})
		}
	}
	for s != "" {
		m := inline.FindStringSubmatchIndex(s)
		if m == nil {
			plain(s)
			break
		}
		start, end := m[0], m[1]
		group := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return s[m[2*i]:m[2*i+1]]
		}
		switch {
		case group(1) != "":
			plain(s[:start])
			out = append(out, Span{Text: group(1), Bold: true})
		case group(2) != "":
			plain(s[:start])
			out = append(out, Span{Text: group(2), Code: true})
		case group(3) != "":
			plain(s[:start])
			out = append(out, Span{Text: group(3), Link: group(4)})
		case group(5) != "" || group(6) != "":
			// "*word*" and "_word_" stand apart from the words around them: the space or mark before and after, which
			// the match took, stays as written.
			g := 5
			if group(5) == "" {
				g = 6
			}
			ws, we := m[2*g], m[2*g+1]
			plain(s[:ws-1])
			out = append(out, Span{Text: s[ws:we], Italic: true})
			end = we + 1
			if end > len(s) {
				end = len(s)
			}
			plain(s[we+1 : end])
		}
		s = s[end:]
	}
	return out
}

// Plain is a Text's words without their Markdown: for notifications and screen readers.
func Plain(text string) string {
	var b strings.Builder
	for i, l := range Lines(text) {
		if i > 0 {
			b.WriteByte('\n')
		}
		for _, sp := range l.Spans {
			b.WriteString(sp.Text)
		}
	}
	return b.String()
}
