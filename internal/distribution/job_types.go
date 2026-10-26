package distribution

import "time"

const MaxSources = 5
const MaxJobs = 1000
const MaxPendingJobs = 4
const MaxJobInputBytes int64 = 512 << 20
const MaxSourceBytes int64 = 128 << 20
const MaxTaskDiskBytes int64 = 2 << 30

type JobSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SHA256   string `json:"source_sha256"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int64  `json:"bytes"`
	Uploaded bool   `json:"uploaded"`
}
type JobCopy struct {
	Copy
	SourceID       string `json:"source_id"`
	RenderedSHA256 string `json:"rendered_sha256,omitempty"`
	InputBytes     int64  `json:"input_bytes"`
	Uploaded       bool   `json:"input_uploaded"`
}
type Job struct {
	ID            string         `json:"id"`
	Operator      string         `json:"operator"`
	LastActor     string         `json:"last_actor"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	KeyID         string         `json:"key_id"`
	Algorithm     string         `json:"algorithm"`
	Mode          string         `json:"mode"`
	TemplateText  string         `json:"template_text,omitempty"`
	Style         map[string]any `json:"style,omitempty"`
	Sources       []JobSource    `json:"sources"`
	Copies        []JobCopy      `json:"copies"`
	Status        string         `json:"status"`
	Phase         string         `json:"phase"`
	Done          int            `json:"done"`
	Total         int            `json:"total"`
	Attempts      int            `json:"attempts"`
	Error         string         `json:"error,omitempty"`
	Generated     bool           `json:"generated"`
	ArchiveSHA256 string         `json:"archive_sha256,omitempty"`
	ArchiveBytes  int64          `json:"archive_bytes"`
	ExpiresAt     time.Time      `json:"expires_at"`
	DownloadState string         `json:"download_state,omitempty"`
	RequestHash   string         `json:"request_hash,omitempty"`
	Fingerprint   string         `json:"fingerprint,omitempty"`
}

func cloneJob(j Job) Job {
	j.Sources = append([]JobSource(nil), j.Sources...)
	j.Copies = append([]JobCopy(nil), j.Copies...)
	if j.Style != nil {
		m := map[string]any{}
		for k, v := range j.Style {
			m[k] = v
		}
		j.Style = m
	}
	return j
}
func publicJob(j Job) Job { j = cloneJob(j); j.RequestHash = ""; j.Fingerprint = ""; return j }
