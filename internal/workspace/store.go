package workspace

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var aad = []byte("watermark/workspace/v1")

type Store struct {
	mu               sync.Mutex
	dir              string
	admin            string
	key              []byte
	aead             cipher.AEAD
	lock             *os.File
	closed, poisoned bool
	data             state
	write            func(string, []byte) error
}

func DerivedKey(master []byte, purpose string) []byte {
	h := hmac.New(sha256.New, master)
	h.Write([]byte("watermark/workspace/v1/" + purpose))
	return h.Sum(nil)
}
func atomicWrite(path string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".workspace-txn-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func decode(raw []byte, dest any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(dest) != nil || d.Decode(new(any)) != io.EOF {
		return ErrUnavailable
	}
	return nil
}
func Open(dir string, master []byte, admin string) (*Store, error) {
	if len(master) != 32 || !validName(admin, 256) {
		return nil, ErrInvalid
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, ErrUnavailable
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, ErrUnavailable
	}
	lock, err := lockFile(filepath.Join(dir, ".lock"))
	if err != nil {
		return nil, ErrUnavailable
	}
	ok := false
	defer func() {
		if !ok {
			lock.Close()
		}
	}()
	key := DerivedKey(master, "metadata")
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	s := &Store{dir: dir, admin: admin, key: append([]byte(nil), master...), aead: aead, lock: lock, write: atomicWrite}
	path := filepath.Join(dir, "workspace-v1.enc")
	info, err = os.Lstat(path)
	if os.IsNotExist(err) {
		entries, e := os.ReadDir(dir)
		if e != nil {
			return nil, ErrUnavailable
		}
		for _, entry := range entries {
			if entry.Name() != ".lock" {
				return nil, ErrUnavailable
			}
		}
		s.data = state{Version: 1, Admin: admin, Users: map[string]userSecret{}, Teams: map[string]Team{}, Events: []Event{}}
		s.data.Teams[PersonalTeam] = Team{ID: PersonalTeam, Name: "Personal workspace", Enabled: true, Members: map[string]string{admin: RoleOwner}, Revision: 1, UpdatedAt: now()}
		if err = s.commit(s.data); err != nil {
			return nil, err
		}
	} else {
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maxStateBytes+64 {
			return nil, ErrUnavailable
		}
		raw, e := os.ReadFile(path)
		if e != nil || len(raw) < aead.NonceSize() {
			return nil, ErrUnavailable
		}
		plain, e := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], aad)
		if e != nil || decode(plain, &s.data) != nil || s.validate(s.data) != nil {
			return nil, ErrUnavailable
		}
	}
	ok = true
	return s, nil
}
func (s *Store) commit(next state) error {
	if s.closed || s.poisoned {
		return ErrUnavailable
	}
	if err := s.validate(next); err != nil {
		return err
	}
	plain, err := json.Marshal(next)
	if err != nil {
		return ErrInvalid
	}
	if len(plain) > maxStateBytes {
		return ErrLimit
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return ErrUnavailable
	}
	raw := s.aead.Seal(nonce, nonce, plain, aad)
	if err = s.write(filepath.Join(s.dir, "workspace-v1.enc"), raw); err != nil {
		s.poisoned = true
		return ErrUnavailable
	}
	s.data = next
	return nil
}
func (s *Store) change(actor, team, action, target string, mutate func(*state) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return ErrUnavailable
	}
	if !s.active(s.data, actor) {
		return ErrDenied
	}
	raw, _ := json.Marshal(s.data)
	var next state
	if decode(raw, &next) != nil {
		return ErrUnavailable
	}
	if err := mutate(&next); err != nil {
		return err
	}
	// Reserve bounded audit/storage capacity for environment-admin revocation.
	critical := actor == s.admin && (action == "user.update" || action == "team.member" || action == "team.update")
	if len(next.Events) >= maxAudit || !critical && len(next.Events) >= maxAudit-512 {
		return ErrLimit
	}
	if !critical {
		encoded, err := json.Marshal(next)
		if err != nil || len(encoded) > maxStateBytes-(1<<20) {
			return ErrLimit
		}
	}
	next.Events = append(next.Events, Event{Sequence: uint64(len(next.Events) + 1), At: now(), Actor: actor, TeamID: team, Action: action, Target: target})
	return s.commit(next)
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	for i := range s.key {
		s.key[i] = 0
	}
	return s.lock.Close()
}
func (s *Store) EngineKey(team string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.poisoned {
		return nil, ErrUnavailable
	}
	if t, ok := s.data.Teams[team]; !ok || !t.Enabled || team == PersonalTeam {
		return nil, ErrDenied
	}
	return DerivedKey(s.key, "engine/"+team), nil
}
