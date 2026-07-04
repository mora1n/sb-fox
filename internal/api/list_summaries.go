package api

import (
	"time"

	"github.com/mora1n/sb-fox/internal/models"
)

type nodeSummaryResponse struct {
	ID            int64     `json:"id"`
	OwnerUserID   int64     `json:"owner_user_id"`
	Tag           string    `json:"tag"`
	Type          string    `json:"type"`
	Server        string    `json:"server"`
	ServerPort    int       `json:"server_port"`
	CountryCode   string    `json:"country_code"`
	CountrySource string    `json:"country_source"`
	Source        string    `json:"source"`
	SourceRef     *int64    `json:"source_ref,omitempty"`
	HasDetour     bool      `json:"has_detour"`
	Detour        string    `json:"detour,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type templateSummaryResponse struct {
	ID          int64     `json:"id"`
	OwnerUserID int64     `json:"owner_user_id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func nodeSummaries(nodes []*models.Node) []nodeSummaryResponse {
	out := make([]nodeSummaryResponse, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeSummaryResponse{
			ID:            n.ID,
			OwnerUserID:   n.OwnerUserID,
			Tag:           n.Tag,
			Type:          n.Type,
			Server:        n.Server,
			ServerPort:    n.ServerPort,
			CountryCode:   n.CountryCode,
			CountrySource: n.CountrySource,
			Source:        n.Source,
			SourceRef:     n.SourceRef,
			HasDetour:     n.HasDetour,
			Detour:        n.Detour,
			CreatedAt:     n.CreatedAt,
			UpdatedAt:     n.UpdatedAt,
		})
	}
	return out
}

func templateSummaries(templates []*models.Template) []templateSummaryResponse {
	out := make([]templateSummaryResponse, 0, len(templates))
	for _, t := range templates {
		out = append(out, templateSummaryResponse{
			ID:          t.ID,
			OwnerUserID: t.OwnerUserID,
			Name:        t.Name,
			Kind:        t.Kind,
			Description: t.Description,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}
	return out
}
