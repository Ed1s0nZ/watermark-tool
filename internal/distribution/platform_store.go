package distribution

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
)

// platformDisk is the v2 commit unit. Keys are encrypted inside the authenticated metadata.
type platformDisk struct {
	Version    int          `json:"version"`
	Batches    state        `json:"batches"`
	SealedKeys []byte       `json:"sealed_keys"`
	Jobs       []Job        `json:"jobs"`
	Audit      []AuditEvent `json:"audit"`
	Sequence   uint64       `json:"audit_sequence"`
}

func (s *Store) platformMAC(data []byte) []byte {
	m := hmac.New(sha256.New, s.derive("platform-v2"))
	m.Write(data)
	return m.Sum(nil)
}
func (s *Store) commitPlatform(data state, keys keyState, jobs []Job, events ...AuditEvent) error {
	if s.poisoned {
		return ErrStore
	}
	audit := append([]AuditEvent(nil), s.audit...)
	sequence := s.sequence
	for _, event := range events {
		if err := validateEvent(event); err != nil {
			return err
		}
		sequence++
		event.Sequence = sequence
		audit = append(audit, event)
	}
	if len(audit) > 100000 || len(jobs) > MaxJobs {
		return ErrLimit
	}
	keyBytes, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	sealed, err := s.seal(keyBytes)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(platformDisk{2, data, sealed, jobs, audit, sequence})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(signedState{payload, s.platformMAC(payload)})
	if err != nil {
		return err
	}
	if len(raw) > maxStateBytes {
		return ErrLimit
	}
	if err = s.write(filepath.Join(s.dir, "platform.json"), raw); err != nil {
		s.poisoned = true
		return ErrStore
	}
	s.data = data
	s.keys = keys
	s.jobs = jobs
	s.audit = audit
	s.sequence = sequence
	s.unified = true
	s.lastBytes = len(raw)
	return nil
}
func (s *Store) loadPlatform() error {
	raw, err := readPrivate(filepath.Join(s.dir, "platform.json"))
	if err != nil {
		return ErrStore
	}
	var env signedState
	if decode(raw, &env) != nil || !hmac.Equal(env.MAC, s.platformMAC(env.Payload)) {
		return ErrStore
	}
	var disk platformDisk
	if decode(env.Payload, &disk) != nil || disk.Version != 2 || len(disk.Jobs) > MaxJobs || len(disk.Audit) > 100000 || disk.Sequence != uint64(len(disk.Audit)) {
		return ErrStore
	}
	keys, err := s.unseal(disk.SealedKeys)
	if err != nil || decode(keys, &s.keys) != nil {
		return ErrStore
	}
	for i, event := range disk.Audit {
		if event.Sequence != uint64(i+1) || validateEvent(event) != nil {
			return ErrStore
		}
	}
	s.data = disk.Batches
	s.jobs = disk.Jobs
	s.audit = disk.Audit
	s.sequence = disk.Sequence
	s.unified = true
	s.lastBytes = len(raw)
	for i := range s.data.Batches {
		s.data.Batches[i].Batch.RequestHash = s.data.Batches[i].RequestHash
		s.data.Batches[i].Batch.Fingerprint = s.data.Batches[i].Fingerprint
	}
	return nil
}
func (s *Store) upgradePlatform() error {
	if !s.unified {
		_, err := os.Lstat(filepath.Join(s.dir, "platform.json"))
		if err == nil {
			if err = s.loadPlatform(); err != nil {
				return err
			}
			if err = s.validate(); err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			if err = s.commitPlatform(s.data, s.keys, []Job{}, NewEvent("system", "storage.migrate", "platform", "completed", "")); err != nil {
				return err
			}
		} else {
			return ErrStore
		}
	}
	if err := atomicWrite(filepath.Join(s.dir, "format.json"), []byte(`{"version":2,"ready":true}`)); err != nil {
		return ErrStore
	}
	return nil
}
func (s *Store) RootDir() string { return s.dir }

func (s *Store) noNewWrites() bool {
	return len(s.audit) >= 99968 || s.lastBytes > maxStateBytes-(1<<20)
}
