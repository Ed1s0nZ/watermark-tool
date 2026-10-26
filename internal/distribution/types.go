// Package distribution owns durable records and versioned, server-only watermark keys.
package distribution

import (
	"errors"
	"time"
)

var (
	ErrStore      = errors.New("store_unavailable")
	ErrConfig     = errors.New("platform_config")
	ErrConflict   = errors.New("request_conflict")
	ErrBusy       = errors.New("batch_busy")
	ErrRecipients = errors.New("invalid_recipients")
	ErrLimit      = errors.New("platform_limit")
	ErrMissing    = errors.New("record_not_found")
	ErrDisabled   = errors.New("key_disabled")
	ErrState      = errors.New("invalid_key_state")
)

const MaxRecipients = 20
const maxKeys = 64
const maxEnabledKeys = 8
const MaxArchiveBytes = 128 << 20
const maxStateBytes = 64 << 20

type Recipient struct {
	ID   string `json:"recipient_id"`
	Name string `json:"name"`
}
type Copy struct {
	ID string `json:"id"`
	Recipient
}
type Batch struct {
	ID           string    `json:"id"`
	RequestHash  string    `json:"-"`
	Fingerprint  string    `json:"-"`
	SourceSHA256 string    `json:"source_sha256"`
	Operator     string    `json:"operator"`
	CreatedAt    time.Time `json:"created_at"`
	KeyID        string    `json:"key_id"`
	Algorithm    string    `json:"algorithm"`
	Status       string    `json:"status"`
	Generated    bool      `json:"generated"`
	Error        string    `json:"error,omitempty"`
	Attempts     int       `json:"attempts"`
	Copies       []Copy    `json:"copies"`
}

// batchDisk adds private idempotency fields without exposing them through public JSON.
type batchDisk struct {
	Batch
	RequestHash string `json:"request_hash"`
	Fingerprint string `json:"fingerprint"`
}
type state struct {
	Version int         `json:"version"`
	Batches []batchDisk `json:"batches"`
}
type Key struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy string    `json:"created_by"`
}
type keySecret struct {
	Key
	Secret []byte `json:"secret"`
}
type keyState struct {
	Version int         `json:"version"`
	Keys    []keySecret `json:"keys"`
}
type Record struct {
	Copy
	BatchID      string    `json:"batch_id"`
	KeyID        string    `json:"key_id"`
	KeyState     string    `json:"key_state"`
	Algorithm    string    `json:"algorithm"`
	CreatedAt    time.Time `json:"created_at"`
	Operator     string    `json:"operator"`
	Status       string    `json:"status"`
	Generated    bool      `json:"generated"`
	SourceSHA256 string    `json:"source_sha256"`
}
