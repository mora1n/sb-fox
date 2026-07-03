package sblink

import (
	"encoding/json"
	"fmt"

	"github.com/mora1n/sb-fox/internal/merge"
)

type sip008Document struct {
	Servers []sip008Server `json:"servers"`
}

type sip008Server struct {
	ID         string `json:"id"`
	Remarks    string `json:"remarks"`
	Server     string `json:"server"`
	ServerPort any    `json:"server_port"`
	Method     string `json:"method"`
	Password   string `json:"password"`
}

func parseSIP008(text string) ([]*merge.OrderedMap, error) {
	var doc sip008Document
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, err
	}
	if len(doc.Servers) == 0 {
		return nil, fmt.Errorf("sblink: sip008 has no servers")
	}
	out := make([]*merge.OrderedMap, 0, len(doc.Servers))
	for _, s := range doc.Servers {
		server := cleanServer(s.Server)
		port := anyToString(s.ServerPort)
		pn, err := portNumber(port)
		if err != nil {
			return nil, err
		}
		tag := s.Remarks
		if tag == "" {
			tag = s.ID
		}
		m := merge.NewOrderedMap()
		m.Set("type", "shadowsocks")
		m.Set("tag", tagOrDefault(tag, server, port))
		m.Set("server", server)
		m.Set("server_port", pn)
		m.Set("method", s.Method)
		m.Set("password", s.Password)
		out = append(out, m)
	}
	return out, nil
}
