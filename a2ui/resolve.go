package a2ui

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Scope is where a value is read: the data a relative path starts from (an item of a List's template, "" for the
// whole model) and that item's index (for @index).
type Scope struct {
	Base  string
	Index int
}

// Item is the scope of a template's item: a relative path reads from it.
func (sc Scope) Item(listPath string, i int) Scope {
	return Scope{Base: strings.TrimSuffix(sc.abs(listPath), "/") + "/" + strconv.Itoa(i), Index: i}
}

// abs is a path as it is read in this scope: absolute as it is, relative from the scope's item.
func (sc Scope) abs(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	if p == "" {
		if sc.Base == "" {
			return "/"
		}
		return sc.Base
	}
	return strings.TrimSuffix(sc.Base, "/") + "/" + p
}

// Abs is a binding's path made absolute in this scope.
func (sc Scope) Abs(p string) string { return sc.abs(p) }

// BindingPath is the path a value is bound to ({"@path": …}), "" when it is a literal or a function call.
func BindingPath(v any) (string, bool) {
	m, ok := v.(map[string]any)
	if !ok || len(m) != 1 {
		return "", false
	}
	p, ok := m["@path"].(string)
	return p, ok
}

// Resolve is a dynamic value's value now: a literal as it is, a path read from the data model, a function called.
func (s *Surface) Resolve(v any, sc Scope) any {
	switch x := v.(type) {
	case map[string]any:
		if p, ok := x["@path"].(string); ok && len(x) == 1 {
			return s.Get(sc.abs(p))
		}
		if name, ok := x["@call"].(string); ok {
			args, _ := x["args"].(map[string]any)
			return s.call(name, args, sc)
		}
		out := make(map[string]any, len(x))
		for k, val := range x {
			if strings.HasPrefix(k, "@@") {
				k = k[1:] // "@@path" is the key "@path"
			}
			out[k] = val
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = s.Resolve(e, sc)
		}
		return out
	}
	return v
}

// String is a dynamic value as words.
func (s *Surface) String(v any, sc Scope) string { return Text(s.Resolve(v, sc)) }

// Text is a value as words: a whole number without a point, nothing for nothing.
func Text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return fmt.Sprint(v)
}

// Bool is a dynamic value as true or false.
func (s *Surface) Bool(v any, sc Scope) bool {
	switch x := s.Resolve(v, sc).(type) {
	case bool:
		return x
	case string:
		return x == "true"
	case float64:
		return x != 0
	}
	return false
}

// Number is a dynamic value as a number.
func (s *Surface) Number(v any, sc Scope) (float64, bool) { return number(s.Resolve(v, sc)) }

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(x), ",", ""), 64)
		return f, err == nil
	}
	return 0, false
}

// Strings is a dynamic string list (a ChoicePicker's value).
func (s *Surface) Strings(v any, sc Scope) []string {
	switch x := s.Resolve(v, sc).(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			out = append(out, Text(e))
		}
		return out
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}

func (s *Surface) call(name string, raw map[string]any, sc Scope) any {
	arg := func(k string) any { return s.Resolve(raw[k], sc) }
	str := func(k string) string { return Text(arg(k)) }
	num := func(k string) (float64, bool) { return number(arg(k)) }
	switch name {
	case "@index":
		off, _ := num("offset")
		return float64(sc.Index) + off
	case "formatString":
		return s.interpolate(Text(raw["value"]), sc)
	case "formatNumber":
		v, ok := num("value")
		if !ok {
			return str("value")
		}
		d, has := num("decimals")
		return formatNumber(v, int(d), has, grouping(arg("grouping")))
	case "formatCurrency":
		v, ok := num("value")
		if !ok {
			return str("value")
		}
		d, has := num("decimals")
		if !has {
			d, has = 2, true
		}
		return currencySymbol(str("currency")) + formatNumber(v, int(d), has, grouping(arg("grouping")))
	case "formatDate":
		return formatDate(str("value"), str("format"))
	case "pluralize":
		v, _ := num("value")
		for _, k := range []string{map[bool]string{true: "zero"}[v == 0], map[bool]string{true: "one"}[v == 1], map[bool]string{true: "two"}[v == 2]} {
			if k != "" && raw[k] != nil {
				return str(k)
			}
		}
		return str("other")
	case "required":
		v := arg("value")
		return v != nil && Text(v) != ""
	case "regex":
		re, err := regexp.Compile(str("pattern"))
		return err == nil && len(str("pattern")) <= 512 && re.MatchString(str("value"))
	case "length":
		n := float64(len([]rune(str("value"))))
		return within(n, num)
	case "numeric":
		v, ok := num("value")
		return ok && within(v, num)
	case "email":
		return emailRe.MatchString(str("value"))
	case "and", "or":
		vals, _ := arg("values").([]any)
		all := name == "and"
		for _, v := range vals {
			b, _ := v.(bool)
			if name == "and" && !b {
				return false
			}
			if name == "or" && b {
				return true
			}
		}
		return all
	case "not":
		b, _ := arg("value").(bool)
		return !b
	}
	return nil // openUrl and any function a chat app doesn't have
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func within(n float64, num func(string) (float64, bool)) bool {
	if mn, ok := num("min"); ok && n < mn {
		return false
	}
	if mx, ok := num("max"); ok && n > mx {
		return false
	}
	return true
}

func grouping(v any) bool {
	b, ok := v.(bool)
	return !ok || b
}

func formatNumber(v float64, decimals int, fixed, group bool) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if fixed {
		s = strconv.FormatFloat(v, 'f', decimals, 64)
	}
	if !group {
		return s
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

func currencySymbol(code string) string {
	if s, ok := map[string]string{"USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥", "NGN": "₦", "INR": "₹"}[strings.ToUpper(code)]; ok {
		return s
	}
	if code == "" {
		return ""
	}
	return strings.ToUpper(code) + " "
}

// formatDate formats an ISO 8601 value with the spec's pattern letters (yyyy, MM, dd, HH, mm, ss, MMM, EEE, a…).
func formatDate(value, pattern string) string {
	var t time.Time
	var err error
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02", "15:04:05", "15:04"} {
		if t, err = time.Parse(layout, value); err == nil {
			break
		}
	}
	if err != nil {
		return value
	}
	repl := []struct{ from, to string }{
		{"yyyy", "2006"}, {"yy", "06"}, {"MMMM", "January"}, {"MMM", "Jan"}, {"MM", "01"}, {"M", "1"},
		{"EEEE", "Monday"}, {"EEE", "Mon"}, {"dd", "02"}, {"d", "2"}, {"HH", "15"}, {"hh", "03"}, {"h", "3"},
		{"mm", "04"}, {"ss", "05"}, {"a", "PM"},
	}
	var b strings.Builder
	for i := 0; i < len(pattern); {
		matched := false
		for _, r := range repl {
			if strings.HasPrefix(pattern[i:], r.from) {
				b.WriteString(t.Format(r.to))
				i += len(r.from)
				matched = true
				break
			}
		}
		if !matched {
			if pattern[i] == '\'' { // quoted literal text
				end := strings.IndexByte(pattern[i+1:], '\'')
				if end >= 0 {
					b.WriteString(pattern[i+1 : i+1+end])
					i += end + 2
					continue
				}
			}
			b.WriteByte(pattern[i])
			i++
		}
	}
	return b.String()
}

// interpolate fills formatString's ${…} parts: a path (${/a/b} or ${name} from the item) or a call
// (${formatDate(value:${/d}, format:'yyyy')}), a literal "${" written \${.
func (s *Surface) interpolate(tpl string, sc Scope) string {
	var b strings.Builder
	for i := 0; i < len(tpl); {
		if strings.HasPrefix(tpl[i:], `\${`) {
			b.WriteString("${")
			i += 3
			continue
		}
		if !strings.HasPrefix(tpl[i:], "${") {
			b.WriteByte(tpl[i])
			i++
			continue
		}
		end := closing(tpl, i+2)
		if end < 0 {
			b.WriteString(tpl[i:])
			break
		}
		b.WriteString(Text(s.expr(strings.TrimSpace(tpl[i+2:end]), sc)))
		i = end + 1
	}
	return b.String()
}

// closing is where the ${ opened before from closes, nested ${…} and quotes taken into account.
func closing(s string, from int) int {
	depth := 1
	quote := byte(0)
	for i := from; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// expr is the value of what is inside ${…}: a call name(k: v, …) or a path.
func (s *Surface) expr(e string, sc Scope) any {
	open := strings.IndexByte(e, '(')
	if open > 0 && strings.HasSuffix(e, ")") && !strings.ContainsAny(e[:open], "/ ") {
		name := strings.TrimSpace(e[:open])
		args := map[string]any{}
		for _, part := range splitArgs(e[open+1 : len(e)-1]) {
			k, v, ok := strings.Cut(part, ":")
			if !ok {
				continue
			}
			args[strings.TrimSpace(k)] = s.literal(strings.TrimSpace(v), sc)
		}
		return s.call(name, args, sc)
	}
	return s.Get(sc.abs(e))
}

// literal is an argument written inside ${…}: a quoted string, a number, a boolean or a nested ${…}.
func (s *Surface) literal(v string, sc Scope) any {
	switch {
	case strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}"):
		return s.expr(strings.TrimSpace(v[2:len(v)-1]), sc)
	case len(v) >= 2 && (v[0] == '\'' || v[0] == '"') && v[len(v)-1] == v[0]:
		return v[1 : len(v)-1]
	case v == "true" || v == "false":
		return v == "true"
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}

// splitArgs splits on commas that are not inside quotes or ${…}.
func splitArgs(s string) []string {
	var out []string
	depth, start := 0, 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '{' || c == '(':
			depth++
		case c == '}' || c == ')':
			depth--
		case c == ',' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if strings.TrimSpace(s[start:]) != "" {
		out = append(out, s[start:])
	}
	return out
}
