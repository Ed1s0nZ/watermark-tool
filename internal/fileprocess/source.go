package fileprocess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
	"watermark-tool/internal/filebundle"
)

var identityPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func hashFile(ctx context.Context, name string, limit int64) (string, int64, error) {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > limit {
		return "", 0, ErrSource
	}
	f, err := os.Open(name)
	if err != nil {
		return "", 0, ErrIO
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return "", 0, ErrSource
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(contextReader{ctx, f}, limit+1))
	if err != nil {
		return "", 0, err
	}
	if n != info.Size() || n > limit {
		return "", 0, ErrSource
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func checkSource(ctx context.Context, source filebundle.Record) error {
	if _, err := filebundle.SafePath(source.RelativePath); err != nil {
		return err
	}
	if !identityPattern.MatchString(source.ID) || !digestPattern.MatchString(source.SHA256) || source.Bytes < 1 || source.Bytes > 50<<20 {
		return ErrSource
	}
	digest, size, err := hashFile(ctx, source.PhysicalPath, 50<<20)
	if err != nil {
		return err
	}
	if digest != source.SHA256 || size != source.Bytes {
		return ErrSource
	}
	return nil
}
func validText(text string, max int) bool {
	if !utf8.ValidString(text) || strings.TrimSpace(text) == "" || len(text) > max {
		return false
	}
	for _, r := range text {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
func validateOptions(id Identity, options Options) error {
	if !identityPattern.MatchString(id.DocumentID) || !keyPattern.MatchString(id.KeyID) || id.CreatedAt.IsZero() || id.CreatedAt.Nanosecond() != 0 || !validText(options.Text, 100) || !options.Quality.Valid() {
		return ErrOptions
	}
	if _, err := outputOptions(options.Image); err != nil {
		return err
	}
	return nil
}
