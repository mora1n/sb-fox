// Package models defines the persistent entities and API DTOs for sb-fox.
package models

import "time"

// Node is one proxy outbound. Raw holds the authoritative sing-box outbound
// JSON; the other columns are extracted metadata for filtering/grouping.
type Node struct {
	ID            int64     `json:"id"`
	OwnerUserID   int64     `json:"owner_user_id"`
	Tag           string    `json:"tag"`
	Type          string    `json:"type"`
	Server        string    `json:"server"`
	ServerPort    int       `json:"server_port"`
	CountryCode   string    `json:"country_code"`
	CountrySource string    `json:"country_source"` // auto | manual | override
	Source        string    `json:"source"`         // protocol | subscription | config | manual
	SourceRef     *int64    `json:"source_ref,omitempty"`
	HasDetour     bool      `json:"has_detour"`
	Detour        string    `json:"detour,omitempty"`
	Raw           string    `json:"raw"` // full outbound JSON blob
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Template is a base sing-box config. File-backed seed templates and panel
// edits are both stored as ordinary user templates.
type Template struct {
	ID          int64     `json:"id"`
	OwnerUserID int64     `json:"owner_user_id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Content     string    `json:"content"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SubscriptionSource is a remote URL from which nodes are (re)fetched.
type SubscriptionSource struct {
	ID          int64      `json:"id"`
	OwnerUserID int64      `json:"owner_user_id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	LastFetchAt *time.Time `json:"last_fetch_at,omitempty"`
	LastStatus  string     `json:"last_status"`
	NodeCount   int        `json:"node_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Profile ties a template + selected nodes + generation options into a
// tokenized subscription that renders a full config.json.
type Profile struct {
	ID           int64     `json:"id"`
	OwnerUserID  int64     `json:"owner_user_id"`
	Name         string    `json:"name"`
	TemplateID   int64     `json:"template_id"`
	Options      string    `json:"options"` // JSON: {autoCountryGroups, chainProxy, ...}
	Token        string    `json:"token"`
	NodeIDs      []int64   `json:"node_ids"`
	NodeGroupIDs []int64   `json:"node_group_ids"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProfileOptions is the parsed Profile.Options blob.
type ProfileOptions struct {
	AutoCountryGroups bool    `json:"autoCountryGroups"`
	ChainProxy        bool    `json:"chainProxy"`
	ChainProxyNodeID  int64   `json:"chainProxyNodeId,omitempty"`
	ChainProxyNodeIDs []int64 `json:"chainProxyNodeIds,omitempty"`
}

// NodeGroup is a reusable, ordered collection of nodes.
type NodeGroup struct {
	ID          int64     `json:"id"`
	OwnerUserID int64     `json:"owner_user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	NodeIDs     []int64   `json:"node_ids"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type NodeUsage struct {
	ProfileID    int64  `json:"profile_id"`
	ProfileName  string `json:"profile_name"`
	ViaGroupID   int64  `json:"via_group_id,omitempty"`
	ViaGroupName string `json:"via_group_name,omitempty"`
}

// Admin is a compatibility view over the first administrative user.
type Admin struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// User is an authenticated panel account. A zero limit means unlimited.
type User struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	Role          string    `json:"role"`
	NodeLimit     int       `json:"node_limit"`
	ProfileLimit  int       `json:"profile_limit"`
	TemplateLimit int       `json:"template_limit"`
	PasswordHash  string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (u *User) IsAdmin() bool {
	return u != nil && u.Role == RoleAdmin
}
