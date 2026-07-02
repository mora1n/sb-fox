// Package merge ports the sub-store merge.js logic to Go: it takes a template
// sing-box config plus a set of proxy nodes and produces a final config.json,
// auto-generating per-country selector groups and handling detour/relay chains.
package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// OrderedMap is a JSON object that preserves key insertion order and
// distinguishes an explicit null value from an absent key.
//
// This fidelity matters: templates ship group outbounds as `"outbounds": null`
// and the merge logic only coerces that to `[]` under specific conditions.
// Round-tripping through Go's map[string]any would sort keys and lose the
// null-vs-missing distinction, so we keep an ordered representation instead.
//
// Stored value types mirror the JSON model:
//
//	nil          -> JSON null
//	bool         -> JSON bool
//	json.Number  -> JSON number (preserves integer/float formatting)
//	string       -> JSON string
//	[]any        -> JSON array (elements are these same types)
//	*OrderedMap  -> JSON object
type OrderedMap struct {
	keys   []string
	values map[string]any
}

// NewOrderedMap returns an empty ordered object.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: make(map[string]any)}
}

// Get returns the value for key and whether the key is present.
func (m *OrderedMap) Get(key string) (any, bool) {
	v, ok := m.values[key]
	return v, ok
}

// GetString returns the string value for key, or "" if absent/not a string.
func (m *OrderedMap) GetString(key string) string {
	if v, ok := m.values[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Has reports whether key is present (even if its value is null).
func (m *OrderedMap) Has(key string) bool {
	_, ok := m.values[key]
	return ok
}

// Set inserts or updates key, preserving first-insertion order.
func (m *OrderedMap) Set(key string, value any) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Delete removes key if present.
func (m *OrderedMap) Delete(key string) {
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// Keys returns the keys in insertion order.
func (m *OrderedMap) Keys() []string { return m.keys }

// UnmarshalJSON decodes a JSON object into the ordered map.
func (m *OrderedMap) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("merge: expected JSON object, got %v", tok)
	}
	return m.decodeObject(dec)
}

// MarshalJSON encodes the ordered map, preserving key order and not escaping
// HTML characters (matching JavaScript's JSON.stringify).
func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := marshalValue(k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		val, err := marshalValue(m.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (m *OrderedMap) decodeObject(dec *json.Decoder) error {
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key := keyTok.(string)
		value, err := decodeValue(dec)
		if err != nil {
			return err
		}
		m.Set(key, value)
	}
	_, err := dec.Token() // consume closing '}'
	return err
}

// decodeValue decodes the next JSON value from dec into the ordered model.
func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); ok {
		switch d {
		case '{':
			obj := NewOrderedMap()
			if err := obj.decodeObject(dec); err != nil {
				return nil, err
			}
			return obj, nil
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil { // consume closing ']'
				return nil, err
			}
			return arr, nil
		}
	}
	return tok, nil // nil, bool, json.Number, or string
}

// marshalValue encodes a single ordered-model value without HTML escaping.
func marshalValue(v any) ([]byte, error) {
	switch val := v.(type) {
	case *OrderedMap:
		return val.MarshalJSON()
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, elem := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			b, err := marshalValue(elem)
			if err != nil {
				return nil, err
			}
			buf.Write(b)
		}
		buf.WriteByte(']')
		return buf.Bytes(), nil
	default:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return nil, err
		}
		return bytes.TrimRight(buf.Bytes(), "\n"), nil
	}
}

// ParseOrdered parses a JSON document, preserving object key order.
func ParseOrdered(data []byte) (*OrderedMap, error) {
	m := NewOrderedMap()
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Indent pretty-prints compact JSON with two-space indentation.
func Indent(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
