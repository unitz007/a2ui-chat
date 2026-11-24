// Package a2ui is the A2UI v1.0 protocol (https://a2ui.org/specification/v1.0-a2ui/) as a chat app's renderer needs
// it: the messages an agent sends, the state of each surface they build, the values bound to its data model, and the
// action message sent back when someone taps or submits. It draws nothing: the chatui, slack, discord and telegram
// packages draw a surface for their chat app.
package a2ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Version is the protocol version this package speaks.
const Version = "1.0"

// BasicCatalog is the id of A2UI's basic catalog, the components this package knows.
const BasicCatalog = "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"

// Message is one agent-to-renderer message: exactly one of its fields is set.
type Message struct {
	Version          string            `json:"version"`
	CreateSurface    *CreateSurface    `json:"createSurface,omitempty"`
	UpdateComponents *UpdateComponents `json:"updateComponents,omitempty"`
	UpdateDataModel  *UpdateDataModel  `json:"updateDataModel,omitempty"`
	DeleteSurface    *DeleteSurface    `json:"deleteSurface,omitempty"`
	// Function calls between agent and renderer are carried but not run: a chat app has no renderer functions to
	// offer, and answering them is left to the host.
	CallRendererFunction  json.RawMessage `json:"callRendererFunction,omitempty"`
	AgentFunctionResponse json.RawMessage `json:"agentFunctionResponse,omitempty"`
}

// CreateSurface starts a surface.
type CreateSurface struct {
	SurfaceID     string          `json:"surfaceId"`
	CatalogID     string          `json:"catalogId,omitempty"`
	SendDataModel bool            `json:"sendDataModel,omitempty"`
	Components    []Component     `json:"components,omitempty"`
	DataModel     json.RawMessage `json:"dataModel,omitempty"`
}

// UpdateComponents adds or replaces components of a surface, by id.
type UpdateComponents struct {
	SurfaceID  string      `json:"surfaceId"`
	Components []Component `json:"components"`
}

// UpdateDataModel sets a value in a surface's data model at a JSON Pointer ("/" or empty: the whole model); a null
// value deletes the key.
type UpdateDataModel struct {
	SurfaceID string          `json:"surfaceId"`
	Path      string          `json:"path,omitempty"`
	Value     json.RawMessage `json:"value"`
}

// DeleteSurface removes a surface.
type DeleteSurface struct {
	SurfaceID string `json:"surfaceId"`
}

// Component is one component of a surface: its id, its type ("Text", "Button"…) and its own properties, kept as
// they came (each type reads its own: Prop, String, Children…).
type Component struct {
	ID    string         `json:"id"`
	Type  string         `json:"component"`
	Props map[string]any `json:"-"`
}

// UnmarshalJSON reads a component, keeping every property but id and component in Props.
func (c *Component) UnmarshalJSON(b []byte) error {
	var all map[string]any
	if err := json.Unmarshal(b, &all); err != nil {
		return err
	}
	c.ID, _ = all["id"].(string)
	c.Type, _ = all["component"].(string)
	delete(all, "id")
	delete(all, "component")
	c.Props = all
	if c.ID == "" {
		return fmt.Errorf("a2ui: a component without an id")
	}
	if c.Type == "" {
		return fmt.Errorf("a2ui: component %q has no type", c.ID)
	}
	return nil
}

// MarshalJSON writes a component back as A2UI has it.
func (c Component) MarshalJSON() ([]byte, error) {
	all := make(map[string]any, len(c.Props)+2)
	for k, v := range c.Props {
		all[k] = v
	}
	all["id"], all["component"] = c.ID, c.Type
	return json.Marshal(all)
}

// Prop is one of a component's properties, nil when it has none of that name.
func (c Component) Prop(name string) any { return c.Props[name] }

// ActionMessage is what the renderer sends the agent when someone acts on a surface.
type ActionMessage struct {
	Name              string         `json:"name"`
	SurfaceID         string         `json:"surfaceId"`
	SourceComponentID string         `json:"sourceComponentId"`
	Timestamp         string         `json:"timestamp"`
	Context           map[string]any `json:"context"`
	UserMessage       string         `json:"userMessage,omitempty"`
}

// ErrorMessage is a renderer's error to the agent (a surface it couldn't take).
type ErrorMessage struct {
	Code      string `json:"code"`
	Path      string `json:"path,omitempty"`
	Message   string `json:"message"`
	SurfaceID string `json:"surfaceId,omitempty"`
}

// Outbound is a renderer-to-agent message.
type Outbound struct {
	Version string         `json:"version"`
	Action  *ActionMessage `json:"action,omitempty"`
	Error   *ErrorMessage  `json:"error,omitempty"`
}

// NewAction wraps an action message to send.
func NewAction(a *ActionMessage) Outbound { return Outbound{Version: Version, Action: a} }

// ParseMessage reads one message and checks it is a v1.0 message with exactly one kind.
func ParseMessage(b []byte) (*Message, error) {
	var m Message
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("a2ui: %w", err)
	}
	if m.Version != Version {
		return nil, fmt.Errorf("a2ui: version %q, want %q", m.Version, Version)
	}
	n := 0
	for _, set := range []bool{m.CreateSurface != nil, m.UpdateComponents != nil, m.UpdateDataModel != nil, m.DeleteSurface != nil,
		len(m.CallRendererFunction) > 0, len(m.AgentFunctionResponse) > 0} {
		if set {
			n++
		}
	}
	if n != 1 {
		return nil, fmt.Errorf("a2ui: a message has exactly one kind, this one has %d", n)
	}
	return &m, nil
}

// ReadStream reads JSONL, one message a line, calling each in turn; an empty line is skipped.
func ReadStream(r io.Reader, each func(*Message) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		m, err := ParseMessage([]byte(line))
		if err != nil {
			return err
		}
		if err := each(m); err != nil {
			return err
		}
	}
	return sc.Err()
}

// now is the clock, for tests.
var now = func() time.Time { return time.Now().UTC() }
