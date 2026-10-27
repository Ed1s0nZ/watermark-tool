package filebundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name   string
	data   []byte
	mode   os.FileMode
	method uint16
}

func archiveBytes(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: entry.method}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func archiveFile(t *testing.T, raw []byte) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "input.zip")
	if err := os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return name
}
func TestArchiveActualExtractionPathsAndPrivateRandomFiles(t *testing.T) {
	raw := archiveBytes(t, []zipEntry{{name: "资料/", mode: os.ModeDir | 0755}, {name: "资料/empty/", mode: os.ModeDir | 0755}, {name: "资料/a.pdf", data: []byte("first bytes"), method: zip.Deflate}, {name: "资料/sub/b.png", data: []byte("second bytes"), method: zip.Store}})
	input := archiveFile(t, raw)
	archive, err := Open(t.Context(), input, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if err = archive.Validate(t.Context()); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "staging")
	bundle, err := archive.Extract(t.Context(), destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Files) != 2 || len(bundle.Directories) != 2 {
		t.Fatal(bundle)
	}
	for index, file := range bundle.Files {
		expected := []string{"first bytes", "second bytes"}[index]
		body, err := os.ReadFile(file.PhysicalPath)
		if err != nil || string(body) != expected || len(file.SHA256) != 64 || len(file.ID) != 32 || strings.Contains(filepath.Base(file.PhysicalPath), ".") {
			t.Fatal(file, err)
		}
		info, _ := os.Stat(file.PhysicalPath)
		if info.Mode().Perm() != 0600 {
			t.Fatal(info.Mode())
		}
	}
	info, _ := os.Stat(destination)
	if info.Mode().Perm() != 0700 {
		t.Fatal(info.Mode())
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(raw, after) {
		t.Fatal("archive source modified")
	}
	if _, err = archive.Extract(t.Context(), destination); !errors.Is(err, ErrIO) {
		t.Fatal("existing directory reused", err)
	}
	if _, err = os.Stat(bundle.Files[0].PhysicalPath); err != nil {
		t.Fatal("existing directory removed", err)
	}
}
func TestArchiveRejectsTraversalDuplicatesLinksAndSpecialFiles(t *testing.T) {
	entries := []zipEntry{{name: "good/a.txt", data: []byte("ok")}}
	for _, name := range []string{"../escape.txt", "/absolute.txt", "C:/drive.txt", "a\\backslash.txt", "a/../b.txt", "a//b.txt", "a/./b.txt", "CON.txt", "safe/file.", "a/file:stream", "trailing ", "bad\x00name"} {
		raw := archiveBytes(t, append(append([]zipEntry{}, entries...), zipEntry{name: name, data: []byte("unsafe")}))
		if archive, err := Open(t.Context(), archiveFile(t, raw), DefaultLimits()); err == nil {
			archive.Close()
			t.Fatal("unsafe path accepted", name)
		}
	}
	for _, names := range [][]string{{"same.txt", "same.txt"}, {"A.txt", "a.TXT"}, {"café.txt", "cafe\u0301.txt"}, {"folder", "folder/child.txt"}, {"folder/child.txt", "folder"}} {
		raw := archiveBytes(t, []zipEntry{{name: names[0], data: []byte("first")}, {name: names[1], data: []byte("second")}})
		if archive, err := Open(t.Context(), archiveFile(t, raw), DefaultLimits()); !errors.Is(err, ErrPath) {
			if archive != nil {
				archive.Close()
			}
			t.Fatal(names, err)
		}
	}
	for _, entry := range []zipEntry{{name: "link", mode: os.ModeSymlink | 0777, data: []byte("../target")}, {name: "fifo", mode: os.ModeNamedPipe | 0600}, {name: "fifo/", mode: os.ModeNamedPipe | 0600}} {
		raw := archiveBytes(t, []zipEntry{entries[0], entry})
		if archive, err := Open(t.Context(), archiveFile(t, raw), DefaultLimits()); !errors.Is(err, ErrLink) {
			if archive != nil {
				archive.Close()
			}
			t.Fatal(entry.name, err)
		}
	}
}
func TestCentralBudgetsUnsupportedFlagsAndTruncation(t *testing.T) {
	original := archiveBytes(t, []zipEntry{{name: "one.txt", data: []byte("value")}})
	central := bytes.Index(original, []byte{'P', 'K', 1, 2})
	end := bytes.LastIndex(original, []byte{'P', 'K', 5, 6})
	for _, mutate := range []func([]byte){
		func(raw []byte) { binary.LittleEndian.PutUint16(raw[end+10:end+12], 65535) },
		func(raw []byte) { binary.LittleEndian.PutUint16(raw[end+4:end+6], 1) },
		func(raw []byte) { binary.LittleEndian.PutUint16(raw[central+8:central+10], 1) },
		func(raw []byte) { binary.LittleEndian.PutUint16(raw[central+8:central+10], 64) },
		func(raw []byte) { binary.LittleEndian.PutUint32(raw[central+24:central+28], 0xffffffff) },
		func(raw []byte) { binary.LittleEndian.PutUint32(raw[end+12:end+16], 0x7fffffff) },
		func(raw []byte) { binary.LittleEndian.PutUint16(raw[central+28:central+30], 65535) },
	} {
		raw := append([]byte{}, original...)
		mutate(raw)
		if archive, err := Open(t.Context(), archiveFile(t, raw), DefaultLimits()); err == nil {
			archive.Close()
			t.Fatal("bad directory metadata accepted")
		}
	}
	if _, err := Open(t.Context(), archiveFile(t, original[:len(original)-2]), DefaultLimits()); err == nil {
		t.Fatal("truncated archive accepted")
	}
	limits := DefaultLimits()
	limits.MaxCentralBytes = 46
	if _, err := Open(t.Context(), archiveFile(t, original), limits); !errors.Is(err, ErrLimit) {
		t.Fatal("central directory budget", err)
	}
	tooMany := archiveBytes(t, []zipEntry{{name: "a"}, {name: "b"}})
	limits = DefaultLimits()
	limits.MaxFiles = 1
	limits.MaxEntries = 1
	if _, err := Open(t.Context(), archiveFile(t, tooMany), limits); !errors.Is(err, ErrLimit) {
		t.Fatal("entry budget", err)
	}
}
func TestActualCRCAndExpandedLimitsRollbackStaging(t *testing.T) {
	raw := archiveBytes(t, []zipEntry{{name: "first", data: []byte("original first")}, {name: "second", data: []byte("original second")}})
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	offset, err := reader.File[1].DataOffset()
	if err != nil {
		t.Fatal(err)
	}
	raw[offset] ^= 1
	archive, err := Open(t.Context(), archiveFile(t, raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if err = archive.Validate(t.Context()); !errors.Is(err, ErrChecksum) {
		t.Fatal("corruption not found", err)
	}
	destination := filepath.Join(t.TempDir(), "must-rollback")
	if _, err = archive.Extract(t.Context(), destination); !errors.Is(err, ErrChecksum) {
		t.Fatal(err)
	}
	if _, err = os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial staging remained", err)
	}
	raw = archiveBytes(t, []zipEntry{{name: "large", data: bytes.Repeat([]byte("x"), 128), method: zip.Deflate}})
	central := bytes.Index(raw, []byte{'P', 'K', 1, 2})
	binary.LittleEndian.PutUint32(raw[central+24:central+28], 1)
	limits := DefaultLimits()
	limits.MaxFileBytes = 32
	limits.MaxExpandedBytes = 32
	archive, err = Open(t.Context(), archiveFile(t, raw), limits)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	destination = filepath.Join(t.TempDir(), "expanded-limit")
	if _, err = archive.Extract(t.Context(), destination); err == nil {
		t.Fatal("forged expanded size accepted")
	}
	if _, err = os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	remaining := int64(3)
	var written bytes.Buffer
	if _, err = copyBounded(t.Context(), &written, strings.NewReader("four"), 100, &remaining); !errors.Is(err, ErrLimit) || written.Len() != 0 {
		t.Fatal("actual byte budget bypass", err)
	}
}
func TestFolderManifestAndCancellation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	os.WriteFile(source, []byte("original folder bytes"), 0600)
	destination := filepath.Join(t.TempDir(), "folder-staging")
	bundle, err := Ingest(t.Context(), []Source{{RelativePath: "folder/文件.docx", PhysicalPath: source}}, destination, DefaultLimits())
	if err != nil || len(bundle.Files) != 1 || bundle.Files[0].RelativePath != "folder/文件.docx" {
		t.Fatal(bundle, err)
	}
	body, _ := os.ReadFile(source)
	if string(body) != "original folder bytes" {
		t.Fatal("folder source modified")
	}
	if _, err = Ingest(t.Context(), []Source{{RelativePath: "one", PhysicalPath: source}, {RelativePath: "ONE", PhysicalPath: source}}, filepath.Join(t.TempDir(), "duplicates"), DefaultLimits()); !errors.Is(err, ErrPath) {
		t.Fatal(err)
	}
	symlink := filepath.Join(t.TempDir(), "link")
	if err = os.Symlink(source, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err = Ingest(t.Context(), []Source{{RelativePath: "link", PhysicalPath: symlink}}, filepath.Join(t.TempDir(), "links"), DefaultLimits()); !errors.Is(err, ErrLink) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = Ingest(ctx, []Source{{RelativePath: "source", PhysicalPath: source}}, filepath.Join(t.TempDir(), "cancelled"), DefaultLimits()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	raw := archiveBytes(t, []zipEntry{{name: "file", data: []byte("bytes")}})
	archive, err := Open(t.Context(), archiveFile(t, raw), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	destination = filepath.Join(t.TempDir(), "cancelled-extract")
	if _, err = archive.Extract(ctx, destination); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	file, err := os.Open(bundle.Files[0].PhysicalPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err = io.Copy(io.Discard, file); err != nil {
		t.Fatal(err)
	}
}
