// Package pdfmark writes and verifies signed page-object identifiers, not visual-content signatures.
package pdfmark

import (
	"errors"
	"time"
)

const Algorithm = "pdf-page-v1"
const MarkerKey = "WMPlatformV1"
const MaxPages = 500
const MaxInputBytes int64 = 50 << 20
const MaxOutputBytes int64 = 128 << 20
const MaxTokenBytes = 8192

var (
	ErrInvalid      = errors.New("pdf_invalid")
	ErrEncrypted    = errors.New("pdf_encrypted_unsupported")
	ErrSigned       = errors.New("pdf_signed_unsupported")
	ErrLimit        = errors.New("pdf_limit")
	ErrSelection    = errors.New("pdf_page_selection")
	ErrText         = errors.New("pdf_page_text")
	ErrMarked       = errors.New("pdf_page_already_marked")
	ErrVerification = errors.New("pdf_final_verification")
	ErrIO           = errors.New("pdf_io")
	ErrChanged      = errors.New("pdf_input_changed")
)

type Params struct {
	DocumentID string         `json:"document_id"`
	KeyID      string         `json:"key_id"`
	CreatedAt  time.Time      `json:"created_at"`
	Pages      string         `json:"pages"`
	SkipCover  bool           `json:"skip_cover"`
	Text       string         `json:"text"`
	Overrides  map[int]string `json:"overrides,omitempty"`
}
type Marker struct {
	Version      int       `json:"version"`
	DocumentID   string    `json:"document_id"`
	ID           string    `json:"id"`
	KeyID        string    `json:"key_id"`
	Page         int       `json:"page"`
	Pages        int       `json:"pages"`
	Text         string    `json:"text"`
	SourceSHA256 string    `json:"source_sha256"`
	CreatedAt    time.Time `json:"created_at"`
}
type PageResult struct {
	Page            int     `json:"page"`
	Status          string  `json:"status"`
	MarkerVerified  bool    `json:"marker_verified"`
	PositionMatches bool    `json:"position_matches"`
	Marker          *Marker `json:"marker,omitempty"`
}
type Report struct {
	Algorithm     string       `json:"algorithm"`
	PageCount     int          `json:"page_count"`
	InputSHA256   string       `json:"input_sha256"`
	OutputSHA256  string       `json:"output_sha256,omitempty"`
	Selected      []int        `json:"selected,omitempty"`
	Pages         []PageResult `json:"pages"`
	FinalVerified bool         `json:"final_verified"`
}
type KeyResolver func(keyID string) ([]byte, bool)
type PagePlan struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}
