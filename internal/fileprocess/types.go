// Package fileprocess executes one immutable, server-owned bundle record.
// It never interprets a logical archive path as a physical filesystem path.
package fileprocess

import (
	"errors"
	"time"
	"watermark-tool/internal/output"
	"watermark-tool/internal/pdfmark"
	"watermark-tool/internal/quality"
)

var (
	ErrUnsupported  = errors.New("file_format_unsupported")
	ErrOptions      = errors.New("file_options_invalid")
	ErrSource       = errors.New("file_source_changed")
	ErrStructure    = errors.New("file_structure_invalid")
	ErrVerification = errors.New("file_final_verification")
	ErrIO           = errors.New("file_io")
)

type Identity struct {
	DocumentID string    `json:"document_id"`
	KeyID      string    `json:"key_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type PDFOptions struct {
	Pages     string         `json:"pages"`
	SkipCover bool           `json:"skip_cover"`
	Text      string         `json:"text"`
	Overrides map[int]string `json:"overrides,omitempty"`
}
type Options struct {
	Text    string          `json:"text"`
	PDF     *PDFOptions     `json:"pdf,omitempty"`
	Image   *output.Options `json:"image_output,omitempty"`
	Quality quality.Policy  `json:"quality"`
}
type Plan struct {
	Kind            string             `json:"kind"`
	Algorithm       string             `json:"algorithm"`
	OutputExtension string             `json:"output_extension"`
	InputSHA256     string             `json:"input_sha256"`
	PageCount       int                `json:"page_count,omitempty"`
	Pages           []pdfmark.PagePlan `json:"pages,omitempty"`
	Authenticated   bool               `json:"authenticated"`
	Boundary        string             `json:"boundary"`
}
type Result struct {
	Plan          Plan             `json:"plan"`
	OutputSHA256  string           `json:"output_sha256"`
	OutputBytes   int64            `json:"output_bytes"`
	FinalVerified bool             `json:"final_verified"`
	RecordedAt    string           `json:"recorded_at"`
	Quality       *quality.Summary `json:"quality,omitempty"`
	PDF           *pdfmark.Report  `json:"pdf,omitempty"`
}
