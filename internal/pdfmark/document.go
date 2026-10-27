package pdfmark

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func configuration() *model.Configuration {
	conf := model.NewStatelessConfiguration()
	conf.Offline = true
	conf.ValidationMode = model.ValidationStrict
	conf.ValidateLinks = false
	conf.DecodeAllStreams = false
	conf.Optimize = false
	conf.OptimizeBeforeWriting = false
	conf.OptimizeResourceDicts = false
	conf.OptimizeDuplicateContentStreams = false
	conf.WriteObjectStream = false
	conf.WriteXRefStream = false
	conf.Limits = model.ResourceLimits{MaxInputBytes: MaxInputBytes, MaxObjectBytes: 4 << 20, MaxStreamBytes: MaxInputBytes, MaxDecodeBytes: 32 << 20, MaxImagePixels: 8_000_000, MaxImageBytes: 32 << 20, MaxObjectCount: 100000, MaxObjectStreamCount: 50000, MaxObjectStreamFirst: 4 << 20, MaxXRefEntries: 100000, MaxRecursionDepth: 32}
	return conf
}
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, pdfcpu.ErrWrongPassword) || errors.Is(err, pdfcpu.ErrOwnerPasswordRequired) {
		return ErrEncrypted
	}
	return ErrInvalid
}
func openDocument(c context.Context, path string, limits ...int64) (*model.Context, string, error) {
	readLimit := MaxInputBytes
	if len(limits) > 0 {
		readLimit = limits[0]
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, "", ErrIO
	}
	if info.Size() <= 0 || info.Size() > readLimit {
		return nil, "", ErrLimit
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", ErrIO
	}
	defer f.Close()
	hash, err := hashReader(c, f)
	if err != nil {
		return nil, "", err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, "", ErrIO
	}
	doc, err := api.ReadContext(c, f, func() *model.Configuration {
		conf := configuration()
		conf.Limits.MaxInputBytes = readLimit
		return conf
	}())
	if err != nil {
		return nil, "", classify(err)
	}
	if doc.Encrypt != nil {
		return nil, "", ErrEncrypted
	}
	if err = api.ValidateContext(c, doc); err != nil {
		return nil, "", classify(err)
	}
	if doc.PageCount < 1 || doc.PageCount > MaxPages {
		return nil, "", ErrLimit
	}
	return doc, hash, nil
}
func hashReader(c context.Context, r io.Reader) (string, error) {
	h := sha256.New()
	buffer := make([]byte, 64<<10)
	for {
		if err := c.Err(); err != nil {
			return "", err
		}
		n, err := r.Read(buffer)
		if n > 0 {
			h.Write(buffer[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", ErrIO
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func hashPath(c context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", ErrIO
	}
	defer f.Close()
	return hashReader(c, f)
}
func pageToken(doc *model.Context, page types.Dict) (string, bool, error) {
	value, found, err := doc.DereferenceStringEntry(page, MarkerKey)
	if err != nil {
		return "", true, ErrInvalid
	}
	if !found {
		return "", false, nil
	}
	if value == nil || len(*value) > MaxTokenBytes {
		return "", true, ErrInvalid
	}
	return *value, true, nil
}
func extractDocument(c context.Context, doc *model.Context, hash string, keys KeyResolver) (Report, error) {
	report := Report{Algorithm: Algorithm, PageCount: doc.PageCount, InputSHA256: hash, Pages: []PageResult{}}
	for p := 1; p <= doc.PageCount; p++ {
		if err := c.Err(); err != nil {
			return Report{}, err
		}
		page, _, _, err := doc.PageDict(c, p, false)
		if err != nil {
			return Report{}, classify(err)
		}
		token, found, err := pageToken(doc, page)
		result := PageResult{Page: p, Status: "unmarked"}
		if err != nil {
			result.Status = "malformed"
		} else if found {
			marker, status := decodeMarker(token, keys)
			result.Status = status
			result.Marker = marker
			if marker != nil {
				result.MarkerVerified = true
				result.PositionMatches = marker.Page == p && marker.Pages == doc.PageCount
				if !result.PositionMatches {
					result.Status = "verified_marker_position_changed"
				}
			}
		}
		report.Pages = append(report.Pages, result)
	}
	return report, nil
}
func Inspect(c context.Context, input string, keys KeyResolver) (Report, error) {
	c, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()
	doc, hash, err := openDocument(c, input, MaxOutputBytes)
	if err != nil {
		return Report{}, err
	}
	return extractDocument(c, doc, hash, keys)
}

// Preview parses the actual PDF and validates the exact requested page/text plan without producing a marked output.
func Preview(c context.Context, input string, params Params) (Report, []PagePlan, error) {
	c, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()
	doc, hash, err := openDocument(c, input)
	if err != nil {
		return Report{}, nil, err
	}
	if doc.SignatureExist || doc.AppendOnly {
		return Report{}, nil, ErrSigned
	}
	plan, err := MakePlan(doc.PageCount, params)
	if err != nil {
		return Report{}, nil, err
	}
	for _, item := range plan {
		page, _, _, err := doc.PageDict(c, item.Page, false)
		if err != nil {
			return Report{}, nil, classify(err)
		}
		if page.HasEntry(MarkerKey) {
			return Report{}, nil, ErrMarked
		}
	}
	report, err := extractDocument(c, doc, hash, nil)
	return report, plan, err
}
