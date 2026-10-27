package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var userPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@-]{0,63}$`)
var teamPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func now() time.Time { return time.Now().UTC() }
func validName(value string, max int) bool {
	if value == "" || len(value) > max || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, ch := range value {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}
func validRole(role string) bool {
	return role == RoleOwner || role == RoleOperator || role == RoleReviewer
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", ErrUnavailable
	}
	return hex.EncodeToString(b), nil
}
func (s *Store) active(d state, actor string) bool {
	if actor == s.admin {
		return true
	}
	user, ok := d.Users[actor]
	return ok && user.Enabled
}
func (s *Store) can(d state, actor, team, permission string) bool {
	if !s.active(d, actor) {
		return false
	}
	t, ok := d.Teams[team]
	if !ok || !t.Enabled {
		return false
	}
	if actor == s.admin {
		return true
	}
	switch t.Members[actor] {
	case RoleOwner:
		return permission == "read" || permission == "produce" || permission == "manage" || permission == "approve"
	case RoleOperator:
		return permission == "read" || permission == "produce"
	case RoleReviewer:
		return permission == "read" || permission == "approve"
	}
	return false
}
func (s *Store) validate(d state) error {
	if d.Version != 1 || d.Admin != s.admin || d.Users == nil || d.Teams == nil || len(d.Users) > MaxUsers || len(d.Teams) > MaxTeams || len(d.Events) > maxAudit {
		return ErrInvalid
	}
	for name, u := range d.Users {
		if !userPattern.MatchString(name) || name == s.admin || u.Username != name || u.Revision == 0 || u.UpdatedAt.IsZero() {
			return ErrInvalid
		}
		cost, err := bcrypt.Cost(u.PasswordHash)
		if err != nil || cost != bcrypt.DefaultCost {
			return ErrInvalid
		}
	}
	personal, ok := d.Teams[PersonalTeam]
	if !ok || personal.RequireApproval || !personal.Enabled || len(personal.Members) != 1 || personal.Members[s.admin] != RoleOwner {
		return ErrInvalid
	}
	for id, t := range d.Teams {
		if (id != PersonalTeam && !teamPattern.MatchString(id)) || id != t.ID || !validName(t.Name, 120) || t.Revision == 0 || t.UpdatedAt.IsZero() || len(t.Members) > MaxUsers+1 || len(t.Members) == 0 {
			return ErrInvalid
		}
		owners := 0
		for actor, role := range t.Members {
			if actor != s.admin {
				if _, ok := d.Users[actor]; !ok {
					return ErrInvalid
				}
			}
			if !validRole(role) {
				return ErrInvalid
			}
			if role == RoleOwner {
				owners++
			}
		}
		if owners == 0 {
			return ErrInvalid
		}
	}
	for i, event := range d.Events {
		if event.Sequence != uint64(i+1) || event.At.IsZero() || !validName(event.Actor, 256) || !validName(event.Action, 64) || len(event.Target) > 256 {
			return ErrInvalid
		}
		if event.TeamID != "" {
			if _, ok := d.Teams[event.TeamID]; !ok {
				return ErrInvalid
			}
		}
	}
	if err := s.validateContacts(d); err != nil {
		return err
	}
	if err := s.validatePolicies(d); err != nil {
		return err
	}
	return s.validateApprovals(d)
}
