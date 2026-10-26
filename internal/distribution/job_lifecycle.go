package distribution

import "time"

func (s *Store) jobIndex(id string) int {
	for i, j := range s.jobs {
		if j.ID == id {
			return i
		}
	}
	return -1
}
func (s *Store) commitJob(next []Job, index int, actor, action, outcome, code string) (Job, error) {
	next[index].UpdatedAt = time.Now().UTC()
	event := NewEvent(actor, action, next[index].ID, outcome, code)
	if err := s.commitPlatform(s.data, s.keys, next, event); err != nil {
		return Job{}, err
	}
	return publicJob(next[index]), nil
}
func uploadState(status string) bool {
	return status == "preparing" || status == "failed" || status == "cancelled" || status == "interrupted"
}
func (s *Store) JobInput(id, sourceID, copyID, digest string, size int64) (Job, error) {
	if !ValidDigest(digest) || size <= 0 || size > 50<<20 {
		return Job{}, ErrRecipients
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Job{}, ErrStore
	}
	i := s.jobIndex(id)
	if i < 0 {
		return Job{}, ErrMissing
	}
	next := s.jobsSnapshot()
	j := &next[i]
	if !uploadState(j.Status) || j.Generated {
		return Job{}, ErrState
	}
	sourceIndex := -1
	for n, src := range j.Sources {
		if src.ID == sourceID {
			sourceIndex = n
		}
	}
	if sourceIndex < 0 {
		return Job{}, ErrMissing
	}
	if copyID == "" {
		src := &j.Sources[sourceIndex]
		if src.SHA256 != digest {
			return Job{}, ErrConflict
		}
		src.Uploaded = true
		src.Bytes = size
	} else {
		if j.Mode != "dual" {
			return Job{}, ErrState
		}
		found := false
		for n, c := range j.Copies {
			if c.ID == copyID && c.SourceID == sourceID {
				found = true
				if c.RenderedSHA256 != "" && c.RenderedSHA256 != digest {
					return Job{}, ErrConflict
				}
				j.Copies[n].RenderedSHA256 = digest
				j.Copies[n].InputBytes = size
				j.Copies[n].Uploaded = true
			}
		}
		if !found {
			return Job{}, ErrMissing
		}
	}
	var original, total int64
	for _, src := range j.Sources {
		original += src.Bytes
	}
	total = original
	for _, c := range j.Copies {
		total += c.InputBytes
	}
	if original > MaxSourceBytes || total > MaxJobInputBytes {
		return Job{}, ErrLimit
	}
	if err := s.commitPlatform(s.data, s.keys, next); err != nil {
		return Job{}, err
	}
	return publicJob(*j), nil
}
func (s *Store) QueueJob(id, actor string, retry bool) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Job{}, ErrStore
	}
	if s.noNewWrites() {
		return Job{}, ErrLimit
	}
	i := s.jobIndex(id)
	if i < 0 {
		return Job{}, ErrMissing
	}
	next := s.jobsSnapshot()
	j := &next[i]
	if j.Status == "queued" || j.Status == "processing" || j.Status == "cancelling" {
		return publicJob(*j), nil
	}
	if j.Generated || !uploadState(j.Status) || !retry && j.Status != "preparing" || retry && j.Status == "preparing" {
		return Job{}, ErrState
	}
	for _, k := range s.keys.Keys {
		if k.ID == j.KeyID && k.State == "disabled" {
			return Job{}, ErrDisabled
		}
	}
	for _, src := range j.Sources {
		if !src.Uploaded {
			return Job{}, ErrState
		}
	}
	if j.Mode == "dual" {
		for _, c := range j.Copies {
			if !c.Uploaded {
				return Job{}, ErrState
			}
		}
	}
	pending := 0
	for n, job := range s.jobs {
		if n != i && pendingJob(job.Status) {
			pending++
		}
	}
	if pending >= MaxPendingJobs {
		return Job{}, ErrBusy
	}
	j.Status = "queued"
	j.Phase = "queued"
	j.Done = 0
	j.Error = ""
	j.Attempts++
	j.LastActor = actor
	action := "task.start"
	if retry {
		action = "task.retry"
	}
	return s.commitJob(next, i, actor, action, "queued", "")
}
func (s *Store) StartJob(id string) (Job, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Job{}, nil, ErrStore
	}
	i := s.jobIndex(id)
	if i < 0 {
		return Job{}, nil, ErrMissing
	}
	next := s.jobsSnapshot()
	j := &next[i]
	if j.Status != "queued" {
		return Job{}, nil, ErrState
	}
	var secret []byte
	for _, k := range s.keys.Keys {
		if k.ID == j.KeyID {
			secret = append([]byte(nil), k.Secret...)
		}
	}
	if len(secret) != 32 {
		return Job{}, nil, ErrStore
	}
	j.Status = "processing"
	j.Phase = "generating"
	out, err := s.commitJob(next, i, j.LastActor, "task.execute", "processing", "")
	return out, secret, err
}
func (s *Store) JobProgress(id string, done int, phase string) error {
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
		return ErrState
	}
	if done < j.Done || done > j.Total || (phase != "generating" && phase != "packing") {
		return ErrState
	}
	// Volatile observed progress is only promoted in a later atomic snapshot; recovery marks interrupted.
	j.Done = done
	j.Phase = phase
	return nil
}
func (s *Store) CancelJob(id, actor string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Job{}, ErrStore
	}
	i := s.jobIndex(id)
	if i < 0 {
		return Job{}, ErrMissing
	}
	next := s.jobsSnapshot()
	j := &next[i]
	if !pendingJob(j.Status) || j.Status == "cancelling" {
		return publicJob(*j), nil
	}
	if j.Status == "processing" || j.Status == "cancelling" {
		j.Status = "cancelling"
	} else {
		j.Status = "cancelled"
		j.Phase = "finished"
	}
	return s.commitJob(next, i, actor, "task.cancel", j.Status, "")
}
func (s *Store) FinishJob(id, status, code, archiveHash string, archiveBytes int64) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Job{}, ErrStore
	}
	i := s.jobIndex(id)
	if i < 0 {
		return Job{}, ErrMissing
	}
	next := s.jobsSnapshot()
	j := &next[i]
	if j.Status != "processing" && j.Status != "cancelling" {
		return Job{}, ErrState
	}
	if status != "completed" && status != "failed" && status != "cancelled" && status != "interrupted" {
		return Job{}, ErrState
	}
	if j.Status == "cancelling" {
		status = "cancelled"
		code = "cancelled"
	}
	if status == "completed" {
		if j.Done != j.Total || !ValidDigest(archiveHash) || archiveBytes <= 0 || archiveBytes > MaxArchiveBytes {
			return Job{}, ErrState
		}
		j.Generated = true
		j.ArchiveSHA256 = archiveHash
		j.ArchiveBytes = archiveBytes
		j.ExpiresAt = time.Now().UTC().Add(time.Hour)
	}
	j.Status = status
	j.Error = code
	j.Phase = "finished"
	return s.commitJob(next, i, j.LastActor, "task.finish", status, code)
}
func (s *Store) RecoverJobs() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return ErrStore
	}
	next := s.jobsSnapshot()
	events := []AuditEvent{}
	for i, j := range next {
		if pendingJob(j.Status) {
			next[i].Status = "interrupted"
			next[i].Phase = "finished"
			next[i].Error = "interrupted"
			next[i].UpdatedAt = time.Now().UTC()
			for n := range next[i].Sources {
				next[i].Sources[n].Uploaded = false
			}
			for n := range next[i].Copies {
				next[i].Copies[n].Uploaded = false
			}
			events = append(events, NewEvent("system", "task.recover", j.ID, "interrupted", "interrupted"))
		}
	}
	if len(events) == 0 {
		return nil
	}
	return s.commitPlatform(s.data, s.keys, next, events...)
}
func (s *Store) ClearJobInputs(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return ErrStore
	}
	i := s.jobIndex(id)
	if i < 0 {
		return ErrMissing
	}
	next := s.jobsSnapshot()
	for n := range next[i].Sources {
		next[i].Sources[n].Uploaded = false
	}
	for n := range next[i].Copies {
		next[i].Copies[n].Uploaded = false
	}
	return s.commitPlatform(s.data, s.keys, next)
}
