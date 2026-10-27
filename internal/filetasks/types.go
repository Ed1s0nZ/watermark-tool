// Package filetasks owns bounded mixed-file jobs and their private temporary artifacts.
package filetasks

import (
	"context"
	"errors"
	"time"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/fileprocess"
)

const MaxJobs = 32
const MaxAttempts = 3
const MaxStateBytes = 16 << 20
const MaxJobBytes = 1 << 20
const MaxArchiveBytes int64 = 128 << 20
const MaxDiskBytes int64 = 2 << 30
const InputTTL = 15 * time.Minute

var (
	ErrStore   = errors.New("file_tasks_unavailable")
	ErrLimit   = errors.New("file_tasks_limit")
	ErrMissing = errors.New("file_task_missing")
	ErrState   = errors.New("file_task_state")
	ErrExpired = errors.New("file_task_expired")
	ErrDenied  = errors.New("file_task_denied")
)

var ErrBusy = errors.New("file_tasks_busy")

type Attempt struct {
	StartedAt time.Time `json:"started_at"`
	Number    int       `json:"number"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	At        time.Time `json:"at"`
}
type Item struct {
	Source       filebundle.Record   `json:"source"`
	Options      fileprocess.Options `json:"options"`
	Plan         *fileprocess.Plan   `json:"plan,omitempty"`
	PlanError    string              `json:"plan_error,omitempty"`
	OutputPath   string              `json:"output_path"`
	Status       string              `json:"status"`
	Attempts     []Attempt           `json:"attempts"`
	Result       *fileprocess.Result `json:"result,omitempty"`
	ReportSHA256 string              `json:"report_sha256,omitempty"`
	ReportBytes  int64               `json:"report_bytes,omitempty"`
	Error        string              `json:"error,omitempty"`
}
type Job struct {
	QueuedAt         time.Time `json:"queued_at,omitempty"`
	StartedAt        time.Time `json:"started_at,omitempty"`
	FinishedAt       time.Time `json:"finished_at,omitempty"`
	ID               string    `json:"id"`
	Team             string    `json:"team"`
	Operator         string    `json:"operator"`
	KeyID            string    `json:"key_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	InputExpiresAt   time.Time `json:"input_expires_at"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	Revision         uint64    `json:"revision"`
	Status           string    `json:"status"`
	Items            []Item    `json:"items"`
	Directories      []string  `json:"directories"`
	ArchiveSHA256    string    `json:"archive_sha256,omitempty"`
	ArchiveBytes     int64     `json:"archive_bytes,omitempty"`
	ArchiveAvailable bool      `json:"archive_available"`
	InputsAvailable  bool      `json:"inputs_available"`
	Error            string    `json:"error,omitempty"`
}
type Options struct {
	Team        string
	MetadataKey []byte
	Slots       chan struct{}
	CurrentKey  func() (string, []byte, error)
	ResolveKey  func(string) ([]byte, error)
	// Authorize is called at queue, dispatch, per-file, commit and download boundaries.
	Authorize  func(Job) error
	ArchiveTTL time.Duration
	Now        func() time.Time
}
type Request struct {
	Options fileprocess.Options            `json:"options"`
	PerFile map[string]fileprocess.Options `json:"per_file,omitempty"`
}

// An ingester is called only with a newly generated private directory. HTTP
// adapters map multipart temporary files here; physical paths are never JSON input.
type Ingester func(context.Context, string) (filebundle.Bundle, error)
