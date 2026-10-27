package filetasks

import (
	"context"
	"encoding/json"
	"watermark-tool/internal/fileprocess"
)

// ValidateInputs reparses actual server-owned copies before issuing an approval
// preview receipt. The caller supplies current read authorization and a work slot.
func (m *Manager) ValidateInputs(ctx context.Context, id string) (Job, error) {
	j, err := m.Get(id)
	if err != nil {
		return Job{}, err
	}
	if !j.InputsAvailable || !m.now().Before(j.InputExpiresAt) {
		return Job{}, ErrExpired
	}
	for _, item := range j.Items {
		plan, e := fileprocess.Preview(ctx, m.source(id, item.Source), fileprocess.Identity{DocumentID: item.Source.ID, KeyID: j.KeyID, CreatedAt: j.CreatedAt}, item.Options)
		if ctx.Err() != nil {
			return Job{}, ctx.Err()
		}
		if item.PlanError != "" {
			if e == nil || errorCode(e) != item.PlanError {
				return Job{}, ErrState
			}
			continue
		}
		if e != nil {
			return Job{}, e
		}
		expected, _ := json.Marshal(item.Plan)
		actual, _ := json.Marshal(plan)
		if string(expected) != string(actual) {
			return Job{}, ErrState
		}
	}
	current, err := m.Get(id)
	if err != nil {
		return Job{}, err
	}
	if current.Revision != j.Revision {
		return Job{}, ErrState
	}
	return current, nil
}
