package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

func (s *Server) dedupeNodesForUser(ownerUserID int64, nodes []*models.Node, ignoreSourceRef *int64) ([]*models.Node, int, error) {
	existing, err := s.Store.ListNodes(store.NodeFilter{OwnerUserID: ownerUserID})
	if err != nil {
		return nil, 0, err
	}
	seen := make(map[string]struct{}, len(existing)+len(nodes))
	for _, n := range existing {
		if ignoreSourceRef != nil && n.SourceRef != nil && *n.SourceRef == *ignoreSourceRef {
			continue
		}
		fp, err := nodeFingerprint(n.Raw)
		if err != nil {
			return nil, 0, err
		}
		seen[fp] = struct{}{}
	}

	unique := make([]*models.Node, 0, len(nodes))
	deduped := 0
	for _, n := range nodes {
		fp, err := nodeFingerprint(n.Raw)
		if err != nil {
			return nil, 0, err
		}
		if _, ok := seen[fp]; ok {
			deduped++
			continue
		}
		seen[fp] = struct{}{}
		unique = append(unique, n)
	}
	return unique, deduped, nil
}

func nodeFingerprint(raw string) (string, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := writeCanonicalJSON(&buf, v); err != nil {
		return "", err
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func writeCanonicalJSON(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			key, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(key)
			buf.WriteByte(':')
			if err := writeCanonicalJSON(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonicalJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case json.Number:
		buf.WriteString(x.String())
	case string:
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(b)
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case nil:
		buf.WriteString("null")
	default:
		return fmt.Errorf("unsupported JSON value %T", v)
	}
	return nil
}
