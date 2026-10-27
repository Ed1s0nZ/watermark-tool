package pdfmark

import (
	"context"
	"errors"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"io"
	"os"
	"path/filepath"
	"time"
)

type boundedWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, ErrLimit
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}
func Protect(c context.Context, input, output string, params Params, key []byte) (Report, error) {
	c, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()
	if _, err := os.Lstat(output); err == nil || !errors.Is(err, os.ErrNotExist) {
		return Report{}, ErrIO
	}
	if len(key) != 32 {
		return Report{}, ErrInvalid
	}
	doc, sourceHash, err := openDocument(c, input)
	if err != nil {
		return Report{}, err
	}
	if doc.SignatureExist || doc.AppendOnly {
		return Report{}, ErrSigned
	}
	plan, err := MakePlan(doc.PageCount, params)
	if err != nil {
		return Report{}, err
	}
	before := map[int]string{}
	for page := 1; page <= doc.PageCount; page++ {
		dict, _, _, err := doc.PageDict(c, page, false)
		if err != nil {
			return Report{}, classify(err)
		}
		token, found, err := pageToken(doc, dict)
		if err != nil {
			return Report{}, ErrInvalid
		}
		if found {
			before[page] = token
		}
	}
	expected := map[int]string{}
	for _, item := range plan {
		dict, _, _, err := doc.PageDict(c, item.Page, false)
		if err != nil {
			return Report{}, classify(err)
		}
		if dict.HasEntry(MarkerKey) {
			return Report{}, ErrMarked
		}
		marker := Marker{Version: 1, DocumentID: params.DocumentID, ID: pageID(params.DocumentID, item.Page), KeyID: params.KeyID, Page: item.Page, Pages: doc.PageCount, Text: item.Text, SourceSHA256: sourceHash, CreatedAt: params.CreatedAt.UTC()}
		token, err := encodeMarker(marker, key)
		if err != nil {
			return Report{}, err
		}
		dict.InsertString(MarkerKey, token)
		expected[item.Page] = token
	}
	temp, err := os.CreateTemp(filepath.Dir(output), ".pdfmark-*.pdf")
	if err != nil {
		return Report{}, ErrIO
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	closed := false
	defer func() {
		if !closed {
			temp.Close()
		}
	}()
	if err = api.WriteContext(c, doc, &boundedWriter{writer: temp, remaining: MaxOutputBytes}); err != nil {
		if errors.Is(err, ErrLimit) {
			return Report{}, ErrLimit
		}
		return Report{}, classify(err)
	}
	if err = temp.Sync(); err != nil {
		return Report{}, ErrIO
	}
	if err = temp.Close(); err != nil {
		return Report{}, ErrIO
	}
	closed = true
	// Reparse the complete final bytes, not the mutable in-memory PDF context.
	final, outputHash, err := openDocument(c, tempPath, MaxOutputBytes)
	if err != nil {
		if c.Err() != nil {
			return Report{}, c.Err()
		}
		return Report{}, ErrVerification
	}
	if final.PageCount != doc.PageCount {
		return Report{}, ErrVerification
	}
	for page := 1; page <= doc.PageCount; page++ {
		dict, _, _, err := final.PageDict(c, page, false)
		if err != nil {
			return Report{}, ErrVerification
		}
		token, found, err := pageToken(final, dict)
		if err != nil {
			return Report{}, ErrVerification
		}
		wanted, selected := expected[page]
		if selected {
			if !found || token != wanted {
				return Report{}, ErrVerification
			}
		} else {
			previous, existed := before[page]
			if existed != found || previous != token {
				return Report{}, ErrVerification
			}
		}
	}
	report, err := extractDocument(c, final, sourceHash, func(id string) ([]byte, bool) { return key, id == params.KeyID })
	if err != nil {
		return Report{}, ErrVerification
	}
	report.OutputSHA256 = outputHash
	report.FinalVerified = true
	for _, item := range plan {
		p := report.Pages[item.Page-1]
		if !p.MarkerVerified || !p.PositionMatches || p.Marker.DocumentID != params.DocumentID || p.Marker.Text != item.Text {
			return Report{}, ErrVerification
		}
		report.Selected = append(report.Selected, item.Page)
	}
	latest, err := hashPath(c, input)
	if err != nil {
		return Report{}, err
	}
	if latest != sourceHash {
		return Report{}, ErrChanged
	}
	if err = c.Err(); err != nil {
		return Report{}, err
	}
	// A hard link publishes atomically without replacing an existing destination.
	if err = os.Link(tempPath, output); err != nil {
		return Report{}, ErrIO
	}
	directory, err := os.Open(filepath.Dir(output))
	if err != nil {
		os.Remove(output)
		return Report{}, ErrIO
	}
	err = directory.Sync()
	directory.Close()
	if err != nil {
		os.Remove(output)
		return Report{}, ErrIO
	}
	return report, nil
}
