package filebundle

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type contextReader struct {
	context context.Context
	reader  io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type budgetWriter struct {
	writer        io.Writer
	fileRemaining int64
	remaining     *int64
}

func (w *budgetWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.fileRemaining || int64(len(p)) > *w.remaining {
		return 0, ErrLimit
	}
	n, err := w.writer.Write(p)
	w.fileRemaining -= int64(n)
	*w.remaining -= int64(n)
	return n, err
}
func copyBounded(c context.Context, out io.Writer, input io.Reader, max int64, remaining *int64) (int64, error) {
	if out == nil {
		out = io.Discard
	}
	writer := &budgetWriter{writer: out, fileRemaining: max, remaining: remaining}
	return io.CopyBuffer(writer, contextReader{context: c, reader: input}, make([]byte, 64<<10))
}
func copyRecord(c context.Context, destination, logical string, reader io.Reader, declared, max int64, remaining *int64) (Record, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return Record{}, ErrIO
	}
	id := hex.EncodeToString(raw)
	physical := filepath.Join(destination, id)
	f, err := os.OpenFile(physical, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Record{}, ErrIO
	}
	defer f.Close()
	hash := sha256.New()
	size, err := copyBounded(c, io.MultiWriter(f, hash), reader, max, remaining)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return Record{}, ErrIO
		}
		return Record{}, err
	}
	if size != declared {
		return Record{}, ErrChanged
	}
	if err = f.Sync(); err != nil {
		return Record{}, ErrIO
	}
	if err = f.Close(); err != nil {
		return Record{}, ErrIO
	}
	return Record{ID: id, RelativePath: logical, SHA256: hex.EncodeToString(hash.Sum(nil)), Bytes: size, PhysicalPath: physical}, nil
}
