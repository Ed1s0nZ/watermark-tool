// Package workspace owns encrypted collaboration metadata and team authorization.
package workspace

import (
	"errors"
	"time"
)

var (
	ErrApproval    = errors.New("approval_required")
	ErrDuplicate   = errors.New("workspace_duplicate")
	ErrUnavailable = errors.New("workspace_unavailable")
	ErrInvalid     = errors.New("workspace_invalid")
	ErrConflict    = errors.New("workspace_conflict")
	ErrDenied      = errors.New("workspace_denied")
	ErrMissing     = errors.New("workspace_missing")
	ErrLimit       = errors.New("workspace_limit")
)

const (
	PersonalTeam  = "personal"
	RoleOwner     = "owner"
	RoleOperator  = "operator"
	RoleReviewer  = "reviewer"
	MaxUsers      = 256
	MaxTeams      = 32
	maxAudit      = 100000
	maxStateBytes = 16 << 20
)

type User struct {
	Username  string    `json:"username"`
	Enabled   bool      `json:"enabled"`
	Revision  uint64    `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}
type userSecret struct {
	User
	PasswordHash []byte `json:"password_hash"`
}
type Team struct {
	RequireApproval bool              `json:"require_approval"`
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Enabled         bool              `json:"enabled"`
	Members         map[string]string `json:"members"`
	Revision        uint64            `json:"revision"`
	UpdatedAt       time.Time         `json:"updated_at"`
}
type Event struct {
	Sequence uint64    `json:"sequence"`
	At       time.Time `json:"at"`
	Actor    string    `json:"actor"`
	TeamID   string    `json:"team_id,omitempty"`
	Action   string    `json:"action"`
	Target   string    `json:"target"`
}
type state struct {
	Approvals map[string]Approval     `json:"approvals,omitempty"`
	Policies  map[string]SharedPolicy `json:"policies,omitempty"`
	Contacts  map[string]Contact      `json:"contacts,omitempty"`
	Version   int                     `json:"version"`
	Admin     string                  `json:"admin"`
	Users     map[string]userSecret   `json:"users"`
	Teams     map[string]Team         `json:"teams"`
	Events    []Event                 `json:"events"`
}
