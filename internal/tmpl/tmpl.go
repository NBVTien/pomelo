package tmpl

import "strings"

type Filter func(string) string

func Resolve(s string, lookup func(key string) (string, bool), filters map[string]Filter) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			b.WriteString(s)
			break
		}
		j := strings.Index(s[i:], "}}")
		if j < 0 {
			b.WriteString(s)
			break
		}
		j += i
		b.WriteString(s[:i])
		if v, ok := resolveOne(s[i+2:j], lookup, filters); ok {
			b.WriteString(v)
		} else {
			b.WriteString(s[i : j+2])
		}
		s = s[j+2:]
	}
	return b.String()
}

func resolveOne(inner string, lookup func(string) (string, bool), filters map[string]Filter) (string, bool) {
	parts := strings.Split(inner, "|")
	key := strings.TrimSpace(parts[0])
	val, ok := lookup(key)
	if !ok {
		return "", false
	}
	for _, f := range parts[1:] {
		if fn := filters[strings.TrimSpace(f)]; fn != nil {
			val = fn(val)
		}
	}
	return val, true
}

func Refs(s string) []string {
	var out []string
	seen := map[string]bool{}
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "}}")
		if j < 0 {
			break
		}
		j += i
		key := strings.TrimSpace(strings.Split(s[i+2:j], "|")[0])
		if key != "" && !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
		s = s[j+2:]
	}
	return out
}
