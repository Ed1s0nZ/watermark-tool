package distribution

import (
	"sort"
	"strings"
	"watermark-tool/internal/blind"
)

func pageBounds(total, page, size int) (int, int) {
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	return start, end
}
func ValidPage(page, size int) bool { return page >= 1 && page <= 1000000 && size >= 1 && size <= 100 }
func matches(q string, values ...string) bool {
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), q) {
			return true
		}
	}
	return q == ""
}
func (s *Store) Batches(q string, page, size int) ([]Batch, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return nil, 0, ErrStore
	}
	if !ValidPage(page, size) || len(q) > 256 {
		return nil, 0, ErrRecipients
	}
	q = strings.ToLower(q)
	out := []Batch{}
	for i := len(s.data.Batches) - 1; i >= 0; i-- {
		b := s.data.Batches[i]
		found := matches(q, b.ID, b.SourceSHA256, b.Status)
		for _, c := range b.Copies {
			found = found || matches(q, c.ID, c.Recipient.ID, c.Name)
		}
		if found {
			out = append(out, cloneBatch(b.Batch))
		}
	}
	total := len(out)
	a, z := pageBounds(total, page, size)
	return out[a:z], total, nil
}
func (s *Store) record(b batchDisk, c Copy) Record {
	state := ""
	for _, k := range s.keys.Keys {
		if k.ID == b.KeyID {
			state = k.State
		}
	}
	return Record{Copy: c, BatchID: b.ID, KeyID: b.KeyID, KeyState: state, Algorithm: b.Algorithm, CreatedAt: b.CreatedAt, Operator: b.Operator, Status: b.Status, Generated: b.Generated, SourceSHA256: b.SourceSHA256}
}
func (s *Store) Records(q string, page, size int) ([]Record, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return nil, 0, ErrStore
	}
	if !ValidPage(page, size) || len(q) > 256 {
		return nil, 0, ErrRecipients
	}
	q = strings.ToLower(q)
	out := []Record{}
	for i := len(s.data.Batches) - 1; i >= 0; i-- {
		b := s.data.Batches[i]
		for _, c := range b.Copies {
			if matches(q, c.ID, c.Recipient.ID, c.Name, b.ID, b.SourceSHA256) {
				out = append(out, s.record(b, c))
			}
		}
	}
	for _, j := range s.jobs {
		for _, c := range j.Copies {
			if matches(q, c.ID, c.Recipient.ID, c.Name, j.ID, c.SourceID) {
				out = append(out, s.jobRecord(j, c))
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	total := len(out)
	a, z := pageBounds(total, page, size)
	return out[a:z], total, nil
}
func (s *Store) Lookup(result blind.Result, keyID string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return nil, ErrStore
	}
	for _, b := range s.data.Batches {
		if !b.Generated || b.KeyID != keyID || result.Algorithm != b.Algorithm || result.Timestamp != b.CreatedAt.Format("2006-01-02T15:04:05Z07:00") {
			continue
		}
		for _, c := range b.Copies {
			if c.ID == result.Watermark {
				r := s.record(b, c)
				return &r, nil
			}
		}
	}
	for _, j := range s.jobs {
		if !j.Generated || j.KeyID != keyID || j.Algorithm != result.Algorithm || result.Timestamp != j.CreatedAt.Format("2006-01-02T15:04:05Z07:00") {
			continue
		}
		for _, c := range j.Copies {
			if c.ID == result.Watermark {
				r := s.jobRecord(j, c)
				return &r, nil
			}
		}
	}
	return nil, nil
}
