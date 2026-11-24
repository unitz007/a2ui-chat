package a2ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Surface is one surface as an agent built it: its components by id and its data model. Safe for use by several
// goroutines.
type Surface struct {
	mu         sync.RWMutex
	ID         string
	CatalogID  string
	components map[string]Component
	data       any
}

// Surfaces are the surfaces a renderer holds, by id: what the agent's messages build and an interaction acts on.
type Surfaces struct {
	mu sync.RWMutex
	m  map[string]*Surface
}

// NewSurfaces is an empty set of surfaces.
func NewSurfaces() *Surfaces { return &Surfaces{m: map[string]*Surface{}} }

// Get is a surface by id.
func (ss *Surfaces) Get(id string) (*Surface, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	s, ok := ss.m[id]
	return s, ok
}

// Put keeps a surface (one rebuilt from a host's own store, say).
func (ss *Surfaces) Put(s *Surface) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.m[s.ID] = s
}

// Apply applies one message, and says which surface it was about ("" for a function call, which is left to the host).
func (ss *Surfaces) Apply(m *Message) (string, error) {
	switch {
	case m.CreateSurface != nil:
		c := m.CreateSurface
		ss.mu.Lock()
		if _, dup := ss.m[c.SurfaceID]; dup {
			ss.mu.Unlock()
			return c.SurfaceID, fmt.Errorf("a2ui: INTEGRITY_ERROR: surface %q already exists", c.SurfaceID)
		}
		s := NewSurface(c.SurfaceID)
		s.CatalogID = c.CatalogID
		ss.m[c.SurfaceID] = s
		ss.mu.Unlock()
		if len(c.DataModel) > 0 {
			if err := s.SetData("/", c.DataModel); err != nil {
				return c.SurfaceID, err
			}
		}
		s.Put(c.Components...)
		return c.SurfaceID, nil
	case m.UpdateComponents != nil:
		s, ok := ss.Get(m.UpdateComponents.SurfaceID)
		if !ok {
			return m.UpdateComponents.SurfaceID, fmt.Errorf("a2ui: no surface %q", m.UpdateComponents.SurfaceID)
		}
		s.Put(m.UpdateComponents.Components...)
		return s.ID, nil
	case m.UpdateDataModel != nil:
		s, ok := ss.Get(m.UpdateDataModel.SurfaceID)
		if !ok {
			return m.UpdateDataModel.SurfaceID, fmt.Errorf("a2ui: no surface %q", m.UpdateDataModel.SurfaceID)
		}
		return s.ID, s.SetData(m.UpdateDataModel.Path, m.UpdateDataModel.Value)
	case m.DeleteSurface != nil:
		ss.mu.Lock()
		delete(ss.m, m.DeleteSurface.SurfaceID)
		ss.mu.Unlock()
		return m.DeleteSurface.SurfaceID, nil
	}
	return "", nil
}

// NewSurface is an empty surface.
func NewSurface(id string) *Surface {
	return &Surface{ID: id, components: map[string]Component{}, data: map[string]any{}}
}

// Put adds or replaces components.
func (s *Surface) Put(cs ...Component) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range cs {
		s.components[c.ID] = c
	}
}

// Component is a component by id.
func (s *Surface) Component(id string) (Component, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.components[id]
	return c, ok
}

// Components are every component, by id (a copy), for a host keeping a surface in its own store.
func (s *Surface) Components() []Component {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Component, 0, len(s.components))
	for _, c := range s.components {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Data is the whole data model, as JSON, for a host keeping a surface in its own store.
func (s *Surface) Data() json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.data)
	return b
}

// SetData sets a value at a JSON Pointer ("" or "/": the whole model); a JSON null deletes the key.
func (s *Surface) SetData(path string, value json.RawMessage) error {
	var v any
	if len(value) > 0 {
		if err := json.Unmarshal(value, &v); err != nil {
			return fmt.Errorf("a2ui: data at %q: %w", path, err)
		}
	}
	return s.Set(path, v)
}

// Set sets a value at a JSON Pointer; nil deletes it.
func (s *Surface) Set(path string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path == "" || path == "/" {
		if v == nil {
			v = map[string]any{}
		}
		s.data = v
		return nil
	}
	parts, err := pointer(path)
	if err != nil {
		return err
	}
	root, err := setAt(s.data, parts, v)
	if err != nil {
		return fmt.Errorf("a2ui: set %q: %w", path, err)
	}
	s.data = root
	return nil
}

// Get is the value at a JSON Pointer, nil when there is none.
func (s *Surface) Get(path string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return get(s.data, path)
}

func get(data any, path string) any {
	if path == "" || path == "/" {
		return data
	}
	parts, err := pointer(path)
	if err != nil {
		return nil
	}
	cur := data
	for _, p := range parts {
		switch x := cur.(type) {
		case map[string]any:
			cur = x[p]
		case []any:
			i, err := strconv.Atoi(p)
			if err != nil || i < 0 || i >= len(x) {
				return nil
			}
			cur = x[i]
		default:
			return nil
		}
	}
	return cur
}

// pointer splits an RFC 6901 JSON Pointer into its unescaped parts.
func pointer(path string) ([]string, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("a2ui: %q is not a JSON Pointer", path)
	}
	raw := strings.Split(path[1:], "/")
	for i, p := range raw {
		raw[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}
	return raw, nil
}

func setAt(cur any, parts []string, v any) (any, error) {
	if len(parts) == 0 {
		return v, nil
	}
	p := parts[0]
	switch x := cur.(type) {
	case []any:
		if p == "-" {
			if v == nil {
				return x, nil
			}
			next, err := setAt(nil, parts[1:], v)
			if err != nil {
				return nil, err
			}
			return append(x, next), nil
		}
		i, err := strconv.Atoi(p)
		if err != nil || i < 0 || i > len(x) {
			return nil, fmt.Errorf("index %q out of range", p)
		}
		if i == len(x) {
			x = append(x, nil)
		}
		if v == nil && len(parts) == 1 {
			return append(x[:i:i], x[i+1:]...), nil
		}
		next, err := setAt(x[i], parts[1:], v)
		if err != nil {
			return nil, err
		}
		x[i] = next
		return x, nil
	case map[string]any:
		if v == nil && len(parts) == 1 {
			delete(x, p)
			return x, nil
		}
		next, err := setAt(x[p], parts[1:], v)
		if err != nil {
			return nil, err
		}
		x[p] = next
		return x, nil
	case nil:
		m := map[string]any{}
		if v == nil && len(parts) == 1 {
			return m, nil
		}
		next, err := setAt(nil, parts[1:], v)
		if err != nil {
			return nil, err
		}
		m[p] = next
		return m, nil
	}
	return nil, fmt.Errorf("can't go into %T at %q", cur, p)
}
