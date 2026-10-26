package distribution

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
	"unicode"
	"watermark-tool/internal/blind"
)

type limitedWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, ErrLimit
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

// CSVCell prevents spreadsheet formula evaluation, including hidden prefix characters.
func CSVCell(s string) string {
	trim := strings.TrimLeftFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) })
	if len(trim) > 0 && strings.ContainsAny(trim[:1], "=+-@") {
		return "'" + s
	}
	return s
}
func csvCell(s string) string { return CSVCell(s) }

// Archive embeds independently from img for every recipient; input pixels are never mutated.
func Archive(path string, img image.Image, b Batch, key []byte) error {
	return ArchiveContext(context.Background(), path, img, b, key)
}
func ArchiveContext(ctx context.Context, path string, img image.Image, b Batch, key []byte) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(path)
		}
	}()
	z := zip.NewWriter(&limitedWriter{f, MaxArchiveBytes})
	for _, c := range b.Copies {
		if err := ctx.Err(); err != nil {
			return err
		}
		marked, e := blind.Embed(img, c.ID, key, b.CreatedAt)
		if e != nil {
			return e
		}
		header := &zip.FileHeader{Name: c.ID + ".png", Method: zip.Store}
		header.SetModTime(b.CreatedAt)
		entry, e := z.CreateHeader(header)
		if e != nil {
			return e
		}
		if e = png.Encode(entry, marked); e != nil {
			return e
		}
	}
	h := &zip.FileHeader{Name: "manifest.csv", Method: zip.Store}
	h.SetModTime(b.CreatedAt)
	entry, err := z.CreateHeader(h)
	if err != nil {
		return err
	}
	csvOut := csv.NewWriter(entry)
	if err = csvOut.Write([]string{"filename", "recipient_id", "name", "watermark_id", "key_id"}); err != nil {
		return err
	}
	for _, c := range b.Copies {
		if err = csvOut.Write([]string{c.ID + ".png", csvCell(c.Recipient.ID), csvCell(c.Name), c.ID, b.KeyID}); err != nil {
			return err
		}
	}
	csvOut.Flush()
	if err = csvOut.Error(); err != nil {
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	return f.Sync()
}
