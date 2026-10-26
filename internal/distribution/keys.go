package distribution

import (
	"crypto/rand"
	"encoding/hex"
	"image"
	"time"
	"watermark-tool/internal/blind"
)

func randomID(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func (s *Store) Keys() ([]Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return nil, ErrStore
	}
	out := make([]Key, 0, len(s.keys.Keys))
	for _, k := range s.keys.Keys {
		out = append(out, k.Key)
	}
	return out, nil
}
func (s *Store) CreateKey(actor string) (Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Key{}, ErrStore
	}
	if len(s.keys.Keys) >= maxKeys || s.enabledCount() >= maxEnabledKeys {
		return Key{}, ErrLimit
	}
	id, err := randomID(12)
	if err != nil {
		return Key{}, err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return Key{}, err
	}
	now := time.Now().UTC()
	key := Key{ID: id, State: "decrypt-only", CreatedAt: now, UpdatedAt: now, CreatedBy: actor}
	next := s.keys
	next.Keys = append(append([]keySecret(nil), s.keys.Keys...), keySecret{key, secret})
	if err = s.saveKeys(next); err != nil {
		return Key{}, err
	}
	return key, nil
}
func (s *Store) SetKeyState(id, value string) (Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Key{}, ErrStore
	}
	if value != "active" && value != "decrypt-only" && value != "disabled" {
		return Key{}, ErrState
	}
	idx := -1
	for i, k := range s.keys.Keys {
		if k.ID == id {
			idx = i
		}
	}
	if idx < 0 {
		return Key{}, ErrMissing
	}
	if s.keys.Keys[idx].State == "active" && value != "active" {
		return Key{}, ErrState
	}
	if s.keys.Keys[idx].State == "disabled" && value != "disabled" && s.enabledCount() >= maxEnabledKeys {
		return Key{}, ErrLimit
	}
	next := s.keys
	next.Keys = append([]keySecret(nil), s.keys.Keys...)
	now := time.Now().UTC()
	if value == "active" {
		for i := range next.Keys {
			if next.Keys[i].State == "active" {
				next.Keys[i].State = "decrypt-only"
				next.Keys[i].UpdatedAt = now
			}
		}
	}
	next.Keys[idx].State = value
	next.Keys[idx].UpdatedAt = now
	if err := s.saveKeys(next); err != nil {
		return Key{}, err
	}
	return next.Keys[idx].Key, nil
}
func (s *Store) Current() (string, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return "", nil, ErrStore
	}
	for _, k := range s.keys.Keys {
		if k.State == "active" {
			return k.ID, append([]byte(nil), k.Secret...), nil
		}
	}
	return "", nil, ErrState
}
func (s *Store) Verify(img image.Image, id string) (blind.Result, string, error) {
	s.mu.Lock()
	if s.poisoned {
		s.mu.Unlock()
		return blind.Result{}, "", ErrStore
	}
	keys := append([]keySecret(nil), s.keys.Keys...)
	s.mu.Unlock()
	if id != "" {
		found := false
		for _, k := range keys {
			if k.ID == id {
				found = true
				if k.State == "disabled" {
					return blind.Result{}, "", ErrDisabled
				}
			}
		}
		if !found {
			return blind.Result{}, "", ErrMissing
		}
	}
	// Validate image bounds before repeated key attempts, and prefer current key.
	for _, state := range []string{"active", "decrypt-only"} {
		for _, k := range keys {
			if k.State != state || id != "" && k.ID != id {
				continue
			}
			result, err := blind.Extract(img, k.Secret)
			if err == nil {
				return result, k.ID, nil
			}
			if err != blind.ErrNotFound && err != blind.ErrInvalid {
				return blind.Result{}, "", err
			}
		}
	}
	return blind.Result{}, "", blind.ErrNotFound
}

func (s *Store) enabledCount() int {
	n := 0
	for _, k := range s.keys.Keys {
		if k.State != "disabled" {
			n++
		}
	}
	return n
}
