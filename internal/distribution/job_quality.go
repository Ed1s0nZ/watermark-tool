package distribution

import "watermark-tool/internal/quality"

// ObserveJobQuality is volatile until the next atomic task transaction, like progress.
func (s *Store) ObserveJobQuality(id, copyID string, value quality.Summary) error {
	if !value.Valid() {
		return ErrJobState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return ErrStore
	}
	i := s.jobIndex(id)
	if i < 0 {
		return ErrMissing
	}
	j := &s.jobs[i]
	if j.Status != "processing" && j.Status != "cancelling" {
		return ErrJobState
	}
	for n, c := range j.Copies {
		if c.ID == copyID {
			p := quality.Policy{}
			if j.QualityPolicy != nil {
				p = *j.QualityPolicy
			}
			if value.Accepted != p.Accept(value.Embedding) {
				return ErrJobState
			}
			j.Copies[n].Quality = &value
			return nil
		}
	}
	return ErrMissing
}
