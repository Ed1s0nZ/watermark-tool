package distribution

func (s *Store) jobRecord(j Job, c JobCopy) Record {
	state := ""
	for _, k := range s.keys.Keys {
		if k.ID == j.KeyID {
			state = k.State
		}
	}
	sourceSHA, name := "", ""
	for _, src := range j.Sources {
		if src.ID == c.SourceID {
			sourceSHA = src.SHA256
			name = src.Name
		}
	}
	return Record{Copy: c.Copy, JobID: j.ID, SourceID: c.SourceID, SourceName: name, RenderedSHA256: c.RenderedSHA256, KeyID: j.KeyID, KeyState: state, Algorithm: j.Algorithm, CreatedAt: j.CreatedAt, Operator: j.Operator, Status: j.Status, Generated: j.Generated, SourceSHA256: sourceSHA}
}
