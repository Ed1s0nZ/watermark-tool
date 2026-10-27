package workspace

import (
	"golang.org/x/crypto/bcrypt"
	"sort"
)

func validPassword(password string) bool { return len(password) >= 12 && len(password) <= 72 }
func (s *Store) CreateUser(actor, username, password string) (User, error) {
	if actor != s.admin {
		return User{}, ErrDenied
	}
	if !userPattern.MatchString(username) || username == s.admin || !validPassword(password) {
		return User{}, ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, ErrInvalid
	}
	result := User{Username: username, Enabled: true, Revision: 1, UpdatedAt: now()}
	err = s.change(actor, "", "user.create", username, func(d *state) error {
		if _, ok := d.Users[username]; ok {
			return ErrConflict
		}
		if len(d.Users) >= MaxUsers {
			return ErrLimit
		}
		d.Users[username] = userSecret{result, hash}
		return nil
	})
	return result, err
}
func (s *Store) UpdateUser(actor, username string, revision uint64, enabled bool, password string) (User, error) {
	if actor != s.admin {
		return User{}, ErrDenied
	}
	var hash []byte
	if password != "" {
		if !validPassword(password) {
			return User{}, ErrInvalid
		}
		var err error
		hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, ErrInvalid
		}
	}
	var result User
	err := s.change(actor, "", "user.update", username, func(d *state) error {
		current, ok := d.Users[username]
		if !ok {
			return ErrMissing
		}
		if current.Revision != revision {
			return ErrConflict
		}
		current.Enabled = enabled
		current.Revision++
		current.UpdatedAt = now()
		if hash != nil {
			current.PasswordHash = hash
		}
		d.Users[username] = current
		result = current.User
		return nil
	})
	return result, err
}
func (s *Store) Authenticate(username, password string) (uint64, bool) {
	// Unknown users still pay bcrypt cost to reduce obvious username enumeration.
	s.mu.Lock()
	if s.closed || s.poisoned {
		s.mu.Unlock()
		return 0, false
	}
	user, ok := s.data.Users[username]
	s.mu.Unlock()
	hash := user.PasswordHash
	if !ok {
		hash = []byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO5uWj.FM6mV0bOCFhqgRQb35Ny6A8wYq")
	}
	valid := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if !ok || !user.Enabled || !valid {
		return 0, false
	}
	// Return the verified revision, never a newer password's revision.
	return user.Revision, true
}
func (s *Store) CredentialVersion(username string) (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return 0, false
	}
	if username == s.admin {
		return 0, true
	}
	user, ok := s.data.Users[username]
	return user.Revision, ok && user.Enabled
}
func (s *Store) Users(actor string) ([]User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if actor != s.admin {
		return nil, ErrDenied
	}
	out := make([]User, 0, len(s.data.Users))
	for _, u := range s.data.Users {
		out = append(out, u.User)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}
func (s *Store) IsAdmin(actor string) bool { return actor == s.admin }

// Administrator returns the configured bootstrap account name, never credentials.
func (s *Store) Administrator() string { return s.admin }
