package pdfmark

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testKey = bytes.Repeat([]byte{35}, 32)

func params() Params {
	return Params{DocumentID: strings.Repeat("a", 32), KeyID: "test-key-v1", CreatedAt: time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC), Pages: "all", SkipCover: true, Text: "Page {page}/{pages}", Overrides: map[int]string{3: "第三页独立标识"}}
}
func original(t *testing.T) (string, []byte) {
	t.Helper()
	raw, err := os.ReadFile("testdata/source.pdf")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "original.pdf")
	if err = os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return input, raw
}
func resolver(id string) ([]byte, bool) { return testKey, id == "test-key-v1" }
func TestProtectActualPagesCoverOverridesAndReparse(t *testing.T) {
	input, raw := original(t)
	out := filepath.Join(t.TempDir(), "marked.pdf")
	preview, plan, err := Preview(t.Context(), input, params())
	if err != nil || preview.PageCount != 3 || len(plan) != 2 || plan[0].Page != 2 || plan[1].Text != "第三页独立标识" {
		t.Fatal(preview, plan, err)
	}
	report, err := Protect(t.Context(), input, out, params(), testKey)
	if err != nil {
		t.Fatal(err)
	}
	if !report.FinalVerified || len(report.Selected) != 2 || report.Pages[0].Status != "unmarked" || report.Pages[1].Marker.Text != "Page 2/3" || report.Pages[2].Marker.Text != "第三页独立标识" || len(report.OutputSHA256) != 64 {
		t.Fatal(report)
	}
	hash := sha256.Sum256(raw)
	if report.InputSHA256 != hex.EncodeToString(hash[:]) || report.Pages[1].Marker.SourceSHA256 != report.InputSHA256 {
		t.Fatal("source identity mismatch")
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(after, raw) {
		t.Fatal("original changed")
	}
	extracted, err := Inspect(t.Context(), out, resolver)
	if err != nil || !extracted.Pages[1].MarkerVerified || !extracted.Pages[2].PositionMatches {
		t.Fatal(extracted, err)
	}
	wrong, err := Inspect(t.Context(), out, func(string) ([]byte, bool) { return bytes.Repeat([]byte{36}, 32), true })
	if err != nil || wrong.Pages[1].Status != "invalid_marker" || wrong.Pages[1].Marker != nil {
		t.Fatal(wrong, err)
	}
	unknown, err := Inspect(t.Context(), out, nil)
	if err != nil || unknown.Pages[1].Status != "unknown_key" {
		t.Fatal(unknown, err)
	}
	// Final page dictionaries and canonical form values survive the rewrite.
	doc, _, err := openDocument(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := doc.DereferenceArray(doc.Form["Fields"])
	if err != nil || len(fields) != 1 {
		t.Fatal(fields, err)
	}
	field, err := doc.DereferenceDict(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	value, found, err := doc.DereferenceStringEntry(field, "V")
	if err != nil || !found || value == nil || *value != "unchanged-value" {
		t.Fatal(value, err)
	}
	if path := os.Getenv("WATERMARK_PDF_QA_OUTPUT"); path != "" {
		final, _ := os.ReadFile(out)
		if err = os.WriteFile(path, final, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestNoOverwriteExistingMarkersAndDestinations(t *testing.T) {
	input, raw := original(t)
	out := filepath.Join(t.TempDir(), "marked.pdf")
	if _, err := Protect(t.Context(), input, out, params(), testKey); err != nil {
		t.Fatal(err)
	}
	if _, err := Protect(t.Context(), out, filepath.Join(t.TempDir(), "twice.pdf"), params(), testKey); !errors.Is(err, ErrMarked) {
		t.Fatal("existing marker overwritten", err)
	}
	if _, err := Protect(t.Context(), input, input, params(), testKey); err == nil {
		t.Fatal("overwrote source")
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(raw, after) {
		t.Fatal("source changed")
	}
	existing := filepath.Join(t.TempDir(), "existing.pdf")
	os.WriteFile(existing, []byte("keep existing destination"), 0600)
	if _, err := Protect(t.Context(), input, existing, params(), testKey); !errors.Is(err, ErrIO) {
		t.Fatal(err)
	}
	after, _ = os.ReadFile(existing)
	if string(after) != "keep existing destination" {
		t.Fatal("destination replaced")
	}
	p := params()
	p.DocumentID = strings.Repeat("b", 32)
	p.Pages = "1"
	p.SkipCover = false
	p.Overrides = nil
	p.Text = "Cover only"
	second := filepath.Join(t.TempDir(), "cover-added.pdf")
	report, err := Protect(t.Context(), out, second, p, testKey)
	if err != nil || report.Pages[0].Marker.DocumentID != p.DocumentID || report.Pages[1].Marker.DocumentID == p.DocumentID {
		t.Fatal(report, err)
	}
}
func TestMarkerTamperAndMovedMetadataAreNotPageContentAuthentication(t *testing.T) {
	input, _ := original(t)
	out := filepath.Join(t.TempDir(), "marked.pdf")
	if _, err := Protect(t.Context(), input, out, params(), testKey); err != nil {
		t.Fatal(err)
	}
	doc, _, err := openDocument(t.Context(), out)
	if err != nil {
		t.Fatal(err)
	}
	page, _, _, _ := doc.PageDict(t.Context(), 2, false)
	token, _, _ := pageToken(doc, page)
	last := "0"
	if strings.HasSuffix(token, last) {
		last = "1"
	}
	page.Update(MarkerKey, types.StringLiteral(token[:len(token)-1]+last))
	tampered := filepath.Join(t.TempDir(), "tampered.pdf")
	f, _ := os.Create(tampered)
	err = api.WriteContext(t.Context(), doc, f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	report, err := Inspect(t.Context(), tampered, resolver)
	if err != nil || report.Pages[1].Status != "invalid_marker" {
		t.Fatal(report, err)
	}
	doc, _, _ = openDocument(t.Context(), out)
	page, _, _, _ = doc.PageDict(t.Context(), 2, false)
	token, _, _ = pageToken(doc, page)
	cover, _, _, _ := doc.PageDict(t.Context(), 1, false)
	cover.InsertString(MarkerKey, token)
	moved := filepath.Join(t.TempDir(), "moved.pdf")
	f, _ = os.Create(moved)
	err = api.WriteContext(t.Context(), doc, f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	report, err = Inspect(t.Context(), moved, resolver)
	if err != nil || !report.Pages[0].MarkerVerified || report.Pages[0].PositionMatches || report.Pages[0].Status != "verified_marker_position_changed" {
		t.Fatal(report, err)
	}
}
func TestInvalidEncryptedAndCancelledPDFs(t *testing.T) {
	input, _ := original(t)
	invalid := filepath.Join(t.TempDir(), "invalid.pdf")
	os.WriteFile(invalid, []byte("%PDF-1.7\nnot a real PDF"), 0600)
	if _, err := Inspect(t.Context(), invalid, resolver); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	encrypted := filepath.Join(t.TempDir(), "encrypted.pdf")
	conf := configuration()
	conf.UserPW = "test-only-user"
	conf.OwnerPW = "test-only-owner"
	conf.EncryptUsingAES = true
	conf.EncryptKeyLength = 128
	if err := api.EncryptFile(t.Context(), input, encrypted, conf); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(t.Context(), encrypted, resolver); !errors.Is(err, ErrEncrypted) {
		t.Fatal("encrypted PDF accepted", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out := filepath.Join(t.TempDir(), "cancelled.pdf")
	if _, err := Protect(ctx, input, out, params(), testKey); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled output committed", err)
	}
}
