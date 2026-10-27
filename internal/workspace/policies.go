package workspace

import (
	"sort"
	"time"
)

const MaxSharedPolicies = 20
const MaxPolicyVersions = 20

type PolicyVersion struct {
	Number    uint64        `json:"number"`
	Content   PolicyContent `json:"content"`
	SHA256    string        `json:"sha256"`
	Actor     string        `json:"actor"`
	CreatedAt time.Time     `json:"created_at"`
}
type SharedPolicy struct {
	ID        string          `json:"id"`
	TeamID    string          `json:"team_id"`
	Name      string          `json:"name"`
	Revision  uint64          `json:"revision"`
	Published uint64          `json:"published"`
	Versions  []PolicyVersion `json:"versions"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func (s *Store) validatePolicies(d state) error {
	counts := map[string]int{}
	for id, p := range d.Policies {
		if id != p.ID || !teamPattern.MatchString(id) || !validName(p.Name, 120) || p.Revision == 0 || p.UpdatedAt.IsZero() || len(p.Versions) == 0 || len(p.Versions) > MaxPolicyVersions || p.Published > uint64(len(p.Versions)) {
			return ErrInvalid
		}
		if _, ok := d.Teams[p.TeamID]; !ok {
			return ErrInvalid
		}
		counts[p.TeamID]++
		if counts[p.TeamID] > MaxSharedPolicies {
			return ErrLimit
		}
		for i, v := range p.Versions {
			if v.Number != uint64(i+1) || !validPolicyContent(v.Content) || v.SHA256 != policyDigest(v.Content) || !validName(v.Actor, 256) || v.CreatedAt.IsZero() {
				return ErrInvalid
			}
		}
	}
	return nil
}

// SavePolicy appends a new immutable draft. Publishing is a separate explicit action.
func (s *Store) SavePolicy(actor, team, id, name string, revision uint64, content PolicyContent) (SharedPolicy, error) {
	content, err := clonePolicyContent(content)
	if err != nil || !validName(name, 120) {
		return SharedPolicy{}, ErrInvalid
	}
	create := id == ""
	if create {
		id, err = newID()
		if err != nil {
			return SharedPolicy{}, err
		}
	}
	var result SharedPolicy
	err = s.change(actor, team, "policy.save", id, func(d *state) error {
		if !s.can(*d, actor, team, "manage") {
			return ErrDenied
		}
		if d.Policies == nil {
			d.Policies = map[string]SharedPolicy{}
		}
		p := SharedPolicy{ID: id, TeamID: team, Name: name}
		if create {
			if revision != 0 {
				return ErrInvalid
			}
		} else {
			current, ok := d.Policies[id]
			if !ok || current.TeamID != team {
				return ErrMissing
			}
			if current.Revision != revision {
				return ErrConflict
			}
			p = current
		}
		if len(p.Versions) >= MaxPolicyVersions {
			return ErrLimit
		}
		for otherID, other := range d.Policies {
			if otherID != id && other.TeamID == team && other.Name == name {
				return ErrConflict
			}
		}
		p.Name = name
		p.Revision++
		p.UpdatedAt = now()
		p.Versions = append(p.Versions, PolicyVersion{Number: uint64(len(p.Versions) + 1), Content: content, SHA256: policyDigest(content), Actor: actor, CreatedAt: p.UpdatedAt})
		d.Policies[id] = p
		result = p
		return nil
	})
	if err != nil {
		return SharedPolicy{}, err
	}
	return clonePolicy(result), nil
}

// PublishPolicy changes only the pointer; selecting an older version is a rollback.
func (s *Store) PublishPolicy(actor, team, id string, revision, version uint64) (SharedPolicy, error) {
	var result SharedPolicy
	err := s.change(actor, team, "policy.publish", id, func(d *state) error {
		if !s.can(*d, actor, team, "manage") {
			return ErrDenied
		}
		p, ok := d.Policies[id]
		if !ok || p.TeamID != team {
			return ErrMissing
		}
		if p.Revision != revision {
			return ErrConflict
		}
		if version < 1 || version > uint64(len(p.Versions)) {
			return ErrInvalid
		}
		p.Published = version
		p.Revision++
		p.UpdatedAt = now()
		d.Policies[id] = p
		result = p
		return nil
	})
	if err != nil {
		return SharedPolicy{}, err
	}
	return clonePolicy(result), nil
}
func (s *Store) SharedPolicies(actor, team string) ([]SharedPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if !s.can(s.data, actor, team, "read") {
		return nil, ErrDenied
	}
	result := []SharedPolicy{}
	manage := s.can(s.data, actor, team, "manage")
	for _, p := range s.data.Policies {
		if p.TeamID != team {
			continue
		}
		if !manage && p.Published == 0 {
			continue
		}
		p = clonePolicy(p)
		if !manage {
			p.Versions = []PolicyVersion{p.Versions[p.Published-1]}
		}
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
