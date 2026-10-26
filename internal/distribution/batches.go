package distribution

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
	"watermark-tool/internal/blind"
)

func Digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func ParseRecipients(text string) ([]Recipient, error) {
	if len(text) > 16<<10 || !utf8.ValidString(text) {
		return nil, ErrRecipients
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	reader.FieldsPerRecord = 2
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 || len(rows) > MaxRecipients+1 || rows[0][0] != "recipient_id" || rows[0][1] != "name" {
		return nil, ErrRecipients
	}
	out := make([]Recipient, 0, len(rows)-1)
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		id, name := strings.TrimSpace(row[0]), strings.TrimSpace(row[1])
		if id == "" || name == "" || len(id) > 64 || len(name) > 128 || seen[id] {
			return nil, ErrRecipients
		}
		seen[id] = true
		out = append(out, Recipient{id, name})
	}
	return out, nil
}
func fingerprint(source, actor string, recipients []Recipient) string {
	b, _ := json.Marshal(struct {
		Source, Actor string
		Recipients    []Recipient
	}{source, actor, recipients})
	return Digest(b)
}
func (s *Store) Begin(request, source, actor string, recipients []Recipient) (Batch, []byte, error) {
	if len(request) < 16 || len(request) > 128 || len(source) != 64 || (strings.TrimSpace(actor) == "" || len(actor) > 256 || !utf8.ValidString(actor)) || len(recipients) < 1 || len(recipients) > MaxRecipients {
		return Batch{}, nil, ErrRecipients
	}
	// Revalidate at the domain boundary, not only in CSV parsing.
	seen := map[string]bool{}
	for _, r := range recipients {
		if r.ID == "" || r.Name == "" || len(r.ID) > 64 || len(r.Name) > 128 || (!utf8.ValidString(r.ID) || !utf8.ValidString(r.Name)) || seen[r.ID] {
			return Batch{}, nil, ErrRecipients
		}
		seen[r.ID] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Batch{}, nil, ErrStore
	}
	requestHash := Digest([]byte(request))
	fp := fingerprint(source, actor, recipients)
	for i, b := range s.data.Batches {
		if b.RequestHash == requestHash {
			if b.Fingerprint != fp {
				return Batch{}, nil, ErrConflict
			}
			return s.resume(i)
		}
	}
	if len(s.data.Batches) >= 10000 {
		return Batch{}, nil, ErrLimit
	}
	id, err := randomID(16)
	if err != nil {
		return Batch{}, nil, err
	}
	var key keySecret
	for _, k := range s.keys.Keys {
		if k.State == "active" {
			key = k
			break
		}
	}
	if key.ID == "" {
		return Batch{}, nil, ErrState
	}
	batch := Batch{ID: id, RequestHash: requestHash, Fingerprint: fp, SourceSHA256: source, Operator: actor, CreatedAt: time.Now().UTC().Truncate(time.Second), KeyID: key.ID, Algorithm: blind.Algorithm, Status: "processing", Attempts: 1}
	for _, r := range recipients {
		copyID, err := randomID(16)
		if err != nil {
			return Batch{}, nil, err
		}
		batch.Copies = append(batch.Copies, Copy{ID: copyID, Recipient: r})
	}
	next := s.snapshot()
	next.Batches = append(next.Batches, batchDisk{batch, requestHash, fp})
	if err = s.saveState(next); err != nil {
		return Batch{}, nil, err
	}
	s.busy[id] = true
	return cloneBatch(batch), append([]byte(nil), key.Secret...), nil
}
func cloneBatch(b Batch) Batch { b.Copies = append([]Copy(nil), b.Copies...); return b }
func (s *Store) resume(i int) (Batch, []byte, error) {
	b := s.data.Batches[i]
	if s.busy[b.ID] {
		return Batch{}, nil, ErrBusy
	}
	var key []byte
	for _, k := range s.keys.Keys {
		if k.ID == b.KeyID {
			if k.State == "disabled" {
				return Batch{}, nil, ErrDisabled
			}
			key = k.Secret
		}
	}
	if key == nil {
		return Batch{}, nil, ErrStore
	}
	next := s.snapshot()
	next.Batches[i].Status = "processing"
	next.Batches[i].Error = ""
	next.Batches[i].Attempts++
	if err := s.saveState(next); err != nil {
		return Batch{}, nil, err
	}
	s.busy[b.ID] = true
	return cloneBatch(next.Batches[i].Batch), append([]byte(nil), key...), nil
}
func (s *Store) Resume(id, source string) (Batch, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Batch{}, nil, ErrStore
	}
	for i, b := range s.data.Batches {
		if b.ID == id {
			if b.SourceSHA256 != source {
				return Batch{}, nil, ErrConflict
			}
			return s.resume(i)
		}
	}
	return Batch{}, nil, ErrMissing
}
func (s *Store) Finish(id, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return ErrStore
	}
	for i, b := range s.data.Batches {
		if b.ID == id {
			if !s.busy[id] {
				return ErrConflict
			}
			next := s.snapshot()
			next.Batches[i].Status = "completed"
			next.Batches[i].Generated = true
			next.Batches[i].Error = code
			if code != "" {
				next.Batches[i].Status = "failed"
				next.Batches[i].Generated = b.Generated
			}
			if err := s.saveState(next); err != nil {
				return err
			}
			delete(s.busy, id)
			return nil
		}
	}
	return ErrMissing
}
