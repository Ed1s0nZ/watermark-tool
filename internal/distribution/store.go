package distribution

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	unified   bool
	jobs      []Job
	audit     []AuditEvent
	sequence  uint64
	keyActor  string
	lastBytes int
	mu        sync.Mutex
	dir       string
	master    []byte
	lock      *os.File
	data      state
	keys      keyState
	busy      map[string]bool
	poisoned  bool
	write     func(string, []byte) error
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".txn-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func readPrivate(path string) ([]byte, error) {
	info, statErr := os.Lstat(path)
	if statErr != nil {
		return nil, statErr
	}
	if !info.Mode().IsRegular() {
		return nil, ErrStore
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, ErrStore
	}
	data, err := io.ReadAll(io.LimitReader(f, maxStateBytes+1))
	if len(data) > maxStateBytes {
		return nil, ErrLimit
	}
	return data, err
}
func decode(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return ErrStore
	}
	return nil
}
func (s *Store) seal(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.derive("keys"))
	if err != nil {
		return nil, err
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return a.Seal(nonce, nonce, data, []byte("watermark/keys/v1")), nil
}
func (s *Store) unseal(data []byte) ([]byte, error) {
	block, _ := aes.NewCipher(s.derive("keys"))
	a, _ := cipher.NewGCM(block)
	if len(data) < a.NonceSize() {
		return nil, ErrStore
	}
	return a.Open(nil, data[:a.NonceSize()], data[a.NonceSize():], []byte("watermark/keys/v1"))
}

type signedState struct {
	Payload json.RawMessage `json:"payload"`
	MAC     []byte          `json:"mac"`
}

func (s *Store) mac(data []byte) []byte {
	m := hmac.New(sha256.New, s.derive("records"))
	m.Write([]byte("watermark/records/v1/"))
	m.Write(data)
	return m.Sum(nil)
}
func (s *Store) saveState(next state) error {
	if s.unified {
		return s.commitPlatform(next, s.keys, s.jobs, s.batchEvent(next))
	}
	payload, err := json.Marshal(next)
	if err != nil {
		return err
	}
	data, _ := json.Marshal(signedState{payload, s.mac(payload)})
	if len(data) > maxStateBytes {
		return ErrLimit
	}
	if err = s.write(filepath.Join(s.dir, "records.json"), data); err != nil {
		s.poisoned = true
		return ErrStore
	}
	s.data = next
	return nil
}
func (s *Store) saveKeys(next keyState) error {
	if s.unified {
		event := NewEvent("system", "key.update", "platform", "completed", "")
		if len(next.Keys) > len(s.keys.Keys) {
			k := next.Keys[len(next.Keys)-1]
			event = NewEvent(k.CreatedBy, "key.create", k.ID, "completed", "")
		} else {
			for i, k := range next.Keys {
				if k.State != s.keys.Keys[i].State {
					event = NewEvent(s.keyActor, "key.state", k.ID, "completed", "")
					if k.State == "active" {
						break
					}
				}
			}
		}
		return s.commitPlatform(s.data, next, s.jobs, event)
	}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	sealed, err := s.seal(data)
	if err != nil {
		return err
	}
	if err = s.write(filepath.Join(s.dir, "keys.enc"), sealed); err != nil {
		s.poisoned = true
		return ErrStore
	}
	s.keys = next
	return nil
}
func Open(dir string, master, legacy []byte) (store *Store, err error) {
	if dir == "" || len(master) != 32 || len(legacy) != 0 && len(legacy) != 32 || len(legacy) > 0 && hmac.Equal(master, legacy) {
		return nil, ErrConfig
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, ErrStore
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("%w: data directory must be private (0700)", ErrStore)
	}
	lock, err := lockFile(filepath.Join(dir, ".lock"))
	if err != nil {
		return nil, fmt.Errorf("%w: data directory already locked or unsupported", ErrStore)
	}
	s := &Store{dir: dir, master: append([]byte(nil), master...), lock: lock, busy: map[string]bool{}, write: atomicWrite}
	defer func() {
		if err != nil {
			lock.Close()
		}
	}()
	marker := filepath.Join(dir, "format.json")
	data, readErr := readPrivate(marker)
	if os.IsNotExist(readErr) {
		entries, e := os.ReadDir(dir)
		if e != nil {
			return nil, ErrStore
		}
		for _, entry := range entries {
			if entry.Name() != ".lock" {
				return nil, ErrStore
			}
		}
		for _, name := range []string{"keys.enc", "records.json"} {
			if _, e := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(e) {
				return nil, ErrStore
			}
		}
		if err = atomicWrite(marker, []byte(`{"version":1,"ready":false}`)); err != nil {
			return nil, ErrStore
		}
		key := make([]byte, 32)
		id := ""
		if len(legacy) > 0 {
			copy(key, legacy)
			id = "legacy-env"
		} else {
			if _, err = rand.Read(key); err != nil {
				return nil, err
			}
			id, err = randomID(12)
			if err != nil {
				return nil, err
			}
		}
		now := time.Now().UTC().Truncate(time.Second)
		if err = s.saveKeys(keyState{Version: 1, Keys: []keySecret{{Key: Key{ID: id, State: "active", CreatedAt: now, UpdatedAt: now, CreatedBy: "bootstrap"}, Secret: key}}}); err != nil {
			return nil, err
		}
		if err = s.saveState(state{Version: 1, Batches: []batchDisk{}}); err != nil {
			return nil, err
		}
		if err = atomicWrite(marker, []byte(`{"version":1,"ready":true}`)); err != nil {
			return nil, ErrStore
		}
	} else {
		if readErr != nil {
			return nil, ErrStore
		}
		var format struct {
			Version int  `json:"version"`
			Ready   bool `json:"ready"`
		}
		if decode(data, &format) != nil || (format.Version != 1 && format.Version != 2) || !format.Ready {
			return nil, ErrStore
		}
		if format.Version == 2 {
			if err = s.loadPlatform(); err != nil {
				return nil, err
			}
		} else {
			data, err = readPrivate(filepath.Join(dir, "keys.enc"))
			if err != nil {
				return nil, ErrStore
			}
			data, err = s.unseal(data)
			if err != nil || decode(data, &s.keys) != nil {
				return nil, ErrStore
			}
			data, err = readPrivate(filepath.Join(dir, "records.json"))
			if err != nil {
				return nil, ErrStore
			}
			var envelope signedState
			if decode(data, &envelope) != nil || !hmac.Equal(envelope.MAC, s.mac(envelope.Payload)) || decode(envelope.Payload, &s.data) != nil {
				return nil, ErrStore
			}
			for i := range s.data.Batches {
				s.data.Batches[i].Batch.RequestHash = s.data.Batches[i].RequestHash
				s.data.Batches[i].Batch.Fingerprint = s.data.Batches[i].Fingerprint
			}
		}
	}
	if err = s.validate(); err != nil {
		return nil, err
	}
	if len(legacy) > 0 {
		found := false
		for _, k := range s.keys.Keys {
			if k.ID == "legacy-env" {
				found = true
				if !hmac.Equal(k.Secret, legacy) {
					return nil, fmt.Errorf("%w: imported legacy key mismatch; preserve original or unset legacy environment variable", ErrConfig)
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: legacy environment key was not part of initial migration", ErrConfig)
		}
	}
	if err = s.upgradePlatform(); err != nil {
		return nil, err
	}
	next := s.snapshot()
	changed := false
	for i := range next.Batches {
		if next.Batches[i].Status == "processing" {
			next.Batches[i].Status = "failed"
			next.Batches[i].Error = "interrupted"
			changed = true
		}
	}
	if changed {
		if err = s.saveState(next); err != nil {
			return nil, err
		}
	}
	if err = s.prepareRuntime(); err != nil {
		return nil, ErrStore
	}
	return s, nil
}
func (s *Store) RuntimeDir() string { return filepath.Join(s.dir, "runtime") }
func (s *Store) prepareRuntime() error {
	path := s.RuntimeDir()
	info, err := os.Lstat(path)
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrStore
		}
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Mkdir(path, 0700)
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poisoned = true
	return s.lock.Close()
}
func (s *Store) snapshot() state {
	next := s.data
	next.Batches = append([]batchDisk(nil), s.data.Batches...)
	return next
}
func (s *Store) validate() error {
	if s.keys.Version != 1 || s.data.Version != 1 || len(s.keys.Keys) == 0 || len(s.keys.Keys) > maxKeys || len(s.data.Batches) > 10000 {
		return ErrStore
	}
	keys := map[string]bool{}
	active := 0
	enabled := 0
	for _, k := range s.keys.Keys {
		if k.ID == "" || keys[k.ID] || len(k.Secret) != 32 {
			return ErrStore
		}
		keys[k.ID] = true
		if k.State != "disabled" {
			enabled++
		}
		switch k.State {
		case "active":
			active++
		case "decrypt-only", "disabled":
		default:
			return ErrStore
		}
	}
	if active != 1 || enabled > maxEnabledKeys {
		return ErrStore
	}
	ids, requests, copies := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, b := range s.data.Batches {
		if !keys[b.KeyID] || b.ID == "" || ids[b.ID] || b.RequestHash == "" || requests[b.RequestHash] || b.Fingerprint == "" || len(b.Copies) < 1 || len(b.Copies) > 20 {
			return ErrStore
		}
		ids[b.ID] = true
		requests[b.RequestHash] = true
		if b.Status == "completed" && !b.Generated {
			return ErrStore
		}
		if b.Status != "processing" && b.Status != "completed" && b.Status != "failed" {
			return ErrStore
		}
		for _, c := range b.Copies {
			if len(c.ID) != 32 || copies[c.ID] {
				return ErrStore
			}
			copies[c.ID] = true
		}
	}
	return s.validateJobs()
}

func (s *Store) derive(label string) []byte {
	m := hmac.New(sha256.New, s.master)
	m.Write([]byte("watermark/storage-key/v1/" + label))
	return m.Sum(nil)
}
