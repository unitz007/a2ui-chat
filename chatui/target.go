package chatui

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/unitz007/a2ui-chat/a2ui"
)

// Op is what someone did.
type Op string

const (
	Press Op = "press" // a button
	Pick  Op = "pick"  // one of a choice's options (Value)
	Tick  Op = "tick"  // a tick box
	Fill  Op = "fill"  // open the form with this surface's fields
)

// Target is what a button or menu in a chat app stands for: which surface and component, the scope it was drawn in
// (an item of a template), what was done, and an option's value.
type Target struct {
	Surface   string `json:"s"`
	Component string `json:"c"`
	Base      string `json:"b,omitempty"`
	Index     int    `json:"i,omitempty"`
	Op        Op     `json:"o"`
	Value     string `json:"v,omitempty"`
}

// Scope is the scope the target was drawn in.
func (t Target) Scope() a2ui.Scope { return a2ui.Scope{Base: t.Base, Index: t.Index} }

// Codec turns a target into the id a chat app carries back on a button (Slack's action_id and value, Discord's
// custom_id, Telegram's callback_data) and back. Each chat app has its own limit, so a codec is told it.
type Codec interface {
	Encode(t Target, limit int) (string, error)
	Decode(id string) (Target, bool)
}

// Prefix starts every id this package makes, so a host can tell its own buttons from others.
const Prefix = "a2:"

// Inline carries the target in the id itself (base64 of its JSON): nothing to keep, but ids get long, so it fails
// past a chat app's limit (Telegram's 64 bytes is often too few).
type Inline struct{}

// Encode puts the target in the id.
func (Inline) Encode(t Target, limit int) (string, error) {
	b, _ := json.Marshal(t)
	id := Prefix + base64.RawURLEncoding.EncodeToString(b)
	if limit > 0 && len(id) > limit {
		return "", fmt.Errorf("chatui: %d bytes for %s/%s, the chat app takes %d: use a Store", len(id), t.Surface, t.Component, limit)
	}
	return id, nil
}

// Decode reads the target back.
func (Inline) Decode(id string) (Target, bool) {
	raw, ok := strings.CutPrefix(id, Prefix)
	if !ok {
		return Target{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return Target{}, false
	}
	var t Target
	return t, json.Unmarshal(b, &t) == nil && t.Surface != ""
}

// Memory keeps targets in memory under short random ids, forgetting them after TTL (a day if zero): fine for one
// server; a host with several keeps them in its own store behind Codec.
type Memory struct {
	TTL time.Duration
	mu  sync.Mutex
	m   map[string]memEntry
}

type memEntry struct {
	t  Target
	at time.Time
}

// Encode keeps the target and gives a short id for it (the same id for the same target).
func (m *Memory) Encode(t Target, limit int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = map[string]memEntry{}
	}
	ttl := m.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}
	for id, e := range m.m {
		if time.Since(e.at) > ttl {
			delete(m.m, id)
		} else if e.t == t {
			e.at = time.Now()
			m.m[id] = e
			return id, nil
		}
	}
	var b [9]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	id := Prefix + base64.RawURLEncoding.EncodeToString(b[:])
	if limit > 0 && len(id) > limit {
		return "", fmt.Errorf("chatui: a %d-byte id won't fit %d", len(id), limit)
	}
	m.m[id] = memEntry{t, time.Now()}
	return id, nil
}

// Decode is the target kept under an id.
func (m *Memory) Decode(id string) (Target, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.m[id]
	return e.t, ok
}

// Result is what handling something someone did came to: an action message for the agent (a button), the surface
// changed and to be drawn again (a choice or a tick), or a form to open (Fill).
type Result struct {
	Action   *a2ui.ActionMessage
	Redraw   bool
	OpenForm bool
	// URL is an address to open, for a button whose action is local (openUrl): chat apps open links themselves, so it
	// is only here for one drawn without its address.
	URL string
}

// ErrNoSurface is a target whose surface the renderer no longer holds.
var ErrNoSurface = errors.New("chatui: that surface is gone")

// Handle applies what someone did to its surface.
func Handle(ss *a2ui.Surfaces, t Target) (Result, error) {
	s, ok := ss.Get(t.Surface)
	if !ok {
		return Result{}, ErrNoSurface
	}
	sc := t.Scope()
	c, ok := s.Component(t.Component)
	if !ok {
		return Result{}, fmt.Errorf("chatui: no component %q on %q", t.Component, t.Surface)
	}
	switch t.Op {
	case Pick:
		chosen := s.Strings(c.Props["value"], sc)
		next := []string{t.Value}
		if c.Props["variant"] == "multipleSelection" {
			next = nil
			had := false
			for _, v := range chosen {
				if v == t.Value {
					had = true
					continue
				}
				next = append(next, v)
			}
			if !had {
				next = append(next, t.Value)
			}
		}
		return Result{Redraw: true}, s.Write(t.Component, sc, next)
	case Tick:
		return Result{Redraw: true}, s.Write(t.Component, sc, !s.Bool(c.Props["value"], sc))
	case Fill:
		return Result{OpenForm: true}, nil
	case Press:
		a, err := s.Act(t.Component, sc)
		if errors.Is(err, a2ui.ErrLocal) {
			return Result{URL: s.LocalURL(c, sc)}, nil
		}
		return Result{Action: a}, err
	}
	return Result{}, fmt.Errorf("chatui: unknown op %q", t.Op)
}

// Submit writes what was entered in a chat app's form, by component id, into the surface; it says what is still wrong
// with any field (its checks), by component id.
func Submit(ss *a2ui.Surfaces, surface string, values map[string]string, scopes map[string]a2ui.Scope) (map[string][]string, error) {
	s, ok := ss.Get(surface)
	if !ok {
		return nil, ErrNoSurface
	}
	for id, v := range values {
		var val any = v
		if c, ok := s.Component(id); ok && c.Type == "Slider" {
			val = v // a2ui.Write reads the number
		}
		if err := s.Write(id, scopes[id], val); err != nil {
			return nil, err
		}
	}
	bad := map[string][]string{}
	for id := range values {
		if c, ok := s.Component(id); ok {
			if p := s.Checks(c, scopes[id]); len(p) > 0 {
				bad[id] = p
			}
		}
	}
	return bad, nil
}

// Event is something someone did in a chat app: its target and, from a menu or tick boxes that say what is chosen
// now rather than what was tapped (Slack's checkboxes and multi-select, Discord's select menu), the whole choice.
type Event struct {
	Target
	Values []string
	Set    bool // Values is the whole choice now
}

// HandleEvent applies an event to its surface: a whole choice written as it is, anything else as Handle does.
func HandleEvent(ss *a2ui.Surfaces, e Event) (Result, error) {
	if !e.Set {
		return Handle(ss, e.Target)
	}
	s, ok := ss.Get(e.Surface)
	if !ok {
		return Result{}, ErrNoSurface
	}
	c, ok := s.Component(e.Component)
	if !ok {
		return Result{}, fmt.Errorf("chatui: no component %q on %q", e.Component, e.Surface)
	}
	var v any = e.Values
	if c.Type == "CheckBox" {
		v = len(e.Values) > 0
	}
	return Result{Redraw: true}, s.Write(e.Component, e.Scope(), v)
}
