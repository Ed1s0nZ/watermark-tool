package workspace

import "sort"

func cloneTeam(team Team) Team {
	members := make(map[string]string, len(team.Members))
	for name, role := range team.Members {
		members[name] = role
	}
	team.Members = members
	return team
}
func (s *Store) Teams(actor string) ([]Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if !s.active(s.data, actor) {
		return nil, ErrDenied
	}
	out := []Team{}
	for _, team := range s.data.Teams {
		if actor == s.admin || team.Enabled && team.Members[actor] != "" {
			out = append(out, cloneTeam(team))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *Store) Team(actor, id string) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return Team{}, ErrUnavailable
	}
	if !s.can(s.data, actor, id, "read") {
		return Team{}, ErrDenied
	}
	return cloneTeam(s.data.Teams[id]), nil
}
func (s *Store) Authorize(actor, team, permission string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return ErrUnavailable
	}
	if !s.can(s.data, actor, team, permission) {
		return ErrDenied
	}
	return nil
}
func (s *Store) CreateTeam(actor, name string) (Team, error) {
	if actor != s.admin {
		return Team{}, ErrDenied
	}
	if !validName(name, 120) {
		return Team{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Team{}, err
	}
	team := Team{ID: id, Name: name, Enabled: true, Members: map[string]string{actor: RoleOwner}, Revision: 1, UpdatedAt: now()}
	err = s.change(actor, id, "team.create", id, func(d *state) error {
		if len(d.Teams) >= MaxTeams {
			return ErrLimit
		}
		d.Teams[id] = team
		return nil
	})
	return cloneTeam(team), err
}
func (s *Store) UpdateTeam(actor, id, name string, enabled bool, revision uint64) (Team, error) {
	if !validName(name, 120) || id == PersonalTeam {
		return Team{}, ErrInvalid
	}
	var result Team
	err := s.change(actor, id, "team.update", id, func(d *state) error {
		if actor != s.admin && !s.can(*d, actor, id, "manage") {
			return ErrDenied
		}
		team, ok := d.Teams[id]
		if !ok {
			return ErrMissing
		}
		if team.Revision != revision {
			return ErrConflict
		}
		team.Name = name
		team.Enabled = enabled
		team.Revision++
		team.UpdatedAt = now()
		d.Teams[id] = team
		result = cloneTeam(team)
		return nil
	})
	return result, err
}
func (s *Store) SetMember(actor, id, username, role string, revision uint64) (Team, error) {
	if id == PersonalTeam || role != "" && !validRole(role) {
		return Team{}, ErrInvalid
	}
	var result Team
	err := s.change(actor, id, "team.member", username, func(d *state) error {
		if !s.can(*d, actor, id, "manage") {
			return ErrDenied
		}
		if username != s.admin {
			if _, ok := d.Users[username]; !ok {
				return ErrMissing
			}
		}
		// The environment administrator remains an owner as a recovery path.
		if username == s.admin && role != RoleOwner {
			return ErrDenied
		}
		team := d.Teams[id]
		if team.Revision != revision {
			return ErrConflict
		}
		if role == "" {
			delete(team.Members, username)
		} else {
			team.Members[username] = role
		}
		team.Revision++
		team.UpdatedAt = now()
		d.Teams[id] = team
		result = cloneTeam(team)
		return nil
	})
	return result, err
}
func (s *Store) Events(actor, team string, after uint64, limit int) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if !s.can(s.data, actor, team, "read") {
		return nil, ErrDenied
	}
	if limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	out := []Event{}
	for _, event := range s.data.Events {
		if event.TeamID == team && event.Sequence > after {
			out = append(out, event)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}
