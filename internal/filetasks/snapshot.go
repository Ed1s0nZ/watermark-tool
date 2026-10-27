package filetasks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/fileprocess"
)

// Snapshot binds immutable source bytes, paths, actual plans, key and parameters.
// Attempts, progress, output verification and temporary availability do not change it.
func Snapshot(j Job) (string, error) {
	type item struct {
		Source     filebundle.Record   `json:"source"`
		Options    fileprocess.Options `json:"options"`
		Plan       *fileprocess.Plan   `json:"plan,omitempty"`
		PlanError  string              `json:"plan_error,omitempty"`
		OutputPath string              `json:"output_path"`
	}
	payload := struct {
		Version                   int `json:"version"`
		ID, Team, Operator, KeyID string
		CreatedAt                 time.Time
		Directories               []string
		Items                     []item
	}{Version: 1, ID: j.ID, Team: j.Team, Operator: j.Operator, KeyID: j.KeyID, CreatedAt: j.CreatedAt, Directories: j.Directories, Items: []item{}}
	if !idPattern.MatchString(j.ID) || j.Team == "" || len(j.Items) < 1 {
		return "", ErrState
	}
	for _, i := range j.Items {
		payload.Items = append(payload.Items, item{i.Source, i.Options, i.Plan, i.PlanError, i.OutputPath})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", ErrState
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
