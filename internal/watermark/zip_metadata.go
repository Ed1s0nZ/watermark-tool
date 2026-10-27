package watermark

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"watermark-tool/internal/filebundle"
)

var ErrXMLMetadataMissing = errors.New("xml_watermark_metadata_missing")
var ErrSignedCompatibility = errors.New("signed_document_unsupported")
var ErrCompatibilityOutput = errors.New("compatibility_output_limit")

func validateXMLArchive(c context.Context, input string) error {
	archive, err := filebundle.Open(c, input, filebundle.OfficeLimits())
	if err != nil {
		return err
	}
	defer archive.Close()
	return archive.Validate(c)
}
func readXMLPart(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > 50<<20 {
		return nil, ErrCompatibilityOutput
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, 50<<20+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 50<<20 {
		return nil, ErrCompatibilityOutput
	}
	return raw, nil
}

// ReadXMLArchiveMetadata checks bounded archive structure/CRC before examining the preferred metadata parts.
func ReadXMLArchiveMetadata(input string, targets []string) (text, at string, found bool, err error) {
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = validateXMLArchive(c, input); err != nil {
		return "", "", false, err
	}
	reader, err := zip.OpenReader(input)
	if err != nil {
		return "", "", false, err
	}
	defer reader.Close()
	for _, target := range targets {
		for _, file := range reader.File {
			if file.Name != target {
				continue
			}
			raw, e := readXMLPart(file)
			if e != nil {
				return "", "", false, e
			}
			text, at, found, e = ReadXMLMetadata(raw)
			if found {
				return text, at, true, e
			}
		}
	}
	return "", "", false, nil
}

type compatibilityWriter struct {
	context   context.Context
	out       io.Writer
	remaining int64
}

func (w *compatibilityWriter) Write(raw []byte) (int, error) {
	if err := w.context.Err(); err != nil {
		return 0, err
	}
	if int64(len(raw)) > w.remaining {
		return 0, ErrCompatibilityOutput
	}
	n, err := w.out.Write(raw)
	w.remaining -= int64(n)
	return n, err
}

// WriteXMLArchiveMetadata preserves unchanged ZIP members and verifies the final declared metadata before publishing.
func WriteXMLArchiveMetadata(input, output string, targets []string, text string, at time.Time) error {
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := validateXMLArchive(c, input); err != nil {
		return err
	}
	reader, err := zip.OpenReader(input)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		lowerName := strings.ToLower(file.Name)
		if strings.HasPrefix(lowerName, "_xmlsignatures/") || lowerName == "meta-inf/documentsignatures.xml" || lowerName == "meta-inf/macrosignatures.xml" {
			return ErrSignedCompatibility
		}
		if file.Name == "_rels/.rels" {
			raw, e := readXMLPart(file)
			if e != nil {
				return e
			}
			decoder := xml.NewDecoder(bytes.NewReader(raw))
			for {
				token, e := decoder.Token()
				if e == io.EOF {
					break
				}
				if e != nil {
					return e
				}
				if start, ok := token.(xml.StartElement); ok {
					for _, attribute := range start.Attr {
						if attribute.Name.Local == "Type" && strings.HasPrefix(attribute.Value, "http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/") {
							return ErrSignedCompatibility
						}
					}
				}
			}
		}
	}
	var selected *zip.File
	for _, target := range targets {
		for _, file := range reader.File {
			if file.Name == target {
				selected = file
				break
			}
		}
		if selected != nil {
			break
		}
	}
	if selected == nil {
		return ErrXMLMetadataMissing
	}
	raw, err := readXMLPart(selected)
	if err != nil {
		return err
	}
	modified, err := SetXMLMetadata(raw, text, at)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(output), ".compat-*.zip")
	if err != nil {
		return err
	}
	path := temp.Name()
	defer os.Remove(path)
	defer temp.Close()
	writer := zip.NewWriter(&compatibilityWriter{context: c, out: temp, remaining: 50 << 20})
	for _, file := range reader.File {
		if err = c.Err(); err != nil {
			return err
		}
		if file != selected {
			if err = writer.Copy(file); err != nil {
				return err
			}
			continue
		}
		header := file.FileHeader
		header.CRC32 = 0
		header.CompressedSize64 = 0
		header.UncompressedSize64 = 0
		entry, e := writer.CreateHeader(&header)
		if e != nil {
			return e
		}
		if _, err = io.Copy(entry, bytes.NewReader(modified)); err != nil {
			return err
		}
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	actual, recorded, found, err := ReadXMLArchiveMetadata(path, targets)
	if err != nil {
		return err
	}
	if !found || actual != text || recorded != at.UTC().Format(time.RFC3339) {
		return ErrXMLMetadata
	}
	if err = c.Err(); err != nil {
		return err
	}
	if err = os.Rename(path, output); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(output))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
