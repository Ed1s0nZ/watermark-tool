package fileprocess

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/output"
	"watermark-tool/internal/pdfmark"
	"watermark-tool/internal/watermark"
	"watermark-tool/internal/watermark/rtf"
)

// Execute is synchronous: cancellation never leaves a detached writer after return.
// Output is published exclusively only after final verification and an unchanged-source check.
func Execute(ctx context.Context, source filebundle.Record, destination string, id Identity, options Options, key []byte) (result Result, err error) {
	if len(key) != 32 {
		return result, blind.ErrKey
	}
	if _, e := os.Lstat(destination); e == nil || !os.IsNotExist(e) {
		return result, ErrIO
	}
	plan, err := Preview(ctx, source, id, options)
	if err != nil {
		return result, err
	}
	result.Plan = plan
	dir, err := os.MkdirTemp(filepath.Dir(destination), ".fileprocess-*")
	if err != nil {
		return result, ErrIO
	}
	defer os.RemoveAll(dir)
	if err = os.Chmod(dir, 0700); err != nil {
		return result, ErrIO
	}
	staged := filepath.Join(dir, "verified."+plan.OutputExtension)
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(source.RelativePath), "."))
	switch plan.Kind {
	case "pdf":
		report, e := pdfmark.Protect(ctx, source.PhysicalPath, staged, pdfParams(id, options), key)
		if e != nil {
			return result, e
		}
		if !report.FinalVerified {
			return result, ErrVerification
		}
		result.PDF = &report
		result.RecordedAt = id.CreatedAt.UTC().Format(time.RFC3339)
	case "image":
		img, e := blind.Load(source.PhysicalPath)
		if e != nil {
			return result, e
		}
		marked, e := blind.Embed(img, options.Text, key, id.CreatedAt)
		if e != nil {
			return result, e
		}
		if e = ctx.Err(); e != nil {
			return result, e
		}
		raw, summary, e := output.Encode(img, marked, nil, key, options.Text, id.CreatedAt, imageOptions(ext, options.Image), options.Quality)
		if e != nil {
			return result, e
		}
		if e = os.WriteFile(staged, raw, 0600); e != nil {
			return result, ErrIO
		}
		result.Quality = &summary
		result.RecordedAt = id.CreatedAt.UTC().Format(time.RFC3339)
	case "document":
		if ext == "rtf" {
			err = rtf.NewRTFWatermarker().AddWatermarkAt(source.PhysicalPath, staged, options.Text, id.CreatedAt)
		} else {
			err = watermark.WriteXMLArchiveMetadata(source.PhysicalPath, staged, metadataTargets(ext), options.Text, id.CreatedAt)
		}
		if err != nil {
			return result, err
		}
		if err = validateOffice(ctx, staged, ext); err != nil {
			return result, err
		}
		var actual, stamp string
		if ext == "rtf" {
			actual, stamp, err = rtf.NewRTFWatermarker().ExtractWatermark(staged)
			if err == nil {
				seconds, e := strconv.ParseInt(stamp, 10, 64)
				if e != nil {
					return result, ErrVerification
				}
				stamp = time.Unix(seconds, 0).UTC().Format(time.RFC3339)
			}
		} else {
			var found bool
			actual, stamp, found, err = watermark.ReadXMLArchiveMetadata(staged, metadataTargets(ext))
			if err == nil && !found {
				return result, ErrVerification
			}
		}
		if err != nil {
			return result, err
		}
		if actual != options.Text || stamp != id.CreatedAt.UTC().Format(time.RFC3339) {
			return result, ErrVerification
		}
		result.RecordedAt = stamp
	default:
		return result, ErrUnsupported
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = checkSource(ctx, source); err != nil {
		return result, err
	}
	if err = os.Chmod(staged, 0600); err != nil {
		return result, ErrIO
	}
	f, e := os.OpenFile(staged, os.O_RDWR, 0600)
	if e != nil {
		return result, ErrIO
	}
	e = f.Sync()
	closeErr := f.Close()
	if e != nil || closeErr != nil {
		return result, ErrIO
	}
	result.OutputSHA256, result.OutputBytes, err = hashFile(ctx, staged, pdfmark.MaxOutputBytes)
	if err != nil {
		return result, err
	}
	if result.PDF != nil && result.PDF.OutputSHA256 != result.OutputSHA256 || result.Quality != nil && result.Quality.OutputSHA256 != result.OutputSHA256 {
		return result, ErrVerification
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = os.Link(staged, destination); err != nil {
		return result, ErrIO
	}
	committed := false
	defer func() {
		if !committed {
			os.Remove(destination)
		}
	}()
	parent, e := os.Open(filepath.Dir(destination))
	if e != nil {
		return result, ErrIO
	}
	e = parent.Sync()
	parent.Close()
	if e != nil {
		return result, ErrIO
	}
	result.FinalVerified = true
	committed = true
	return result, nil
}
