package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
	"watermark-tool/internal/watermark"
	_ "watermark-tool/internal/watermark/odt"
	_ "watermark-tool/internal/watermark/rtf"
	_ "watermark-tool/internal/watermark/xlsx"
)

func compatibilityArchive(t *testing.T, ext string, extra map[string]string) string {
	t.Helper()
	main := `<w:document xmlns:w="urn:word"><w:body><w:p><w:r><w:t>Original body</w:t></w:r></w:p></w:body></w:document>`
	name := "word/document.xml"
	if ext == "xlsx" {
		name = "xl/workbook.xml"
		main = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheets/></workbook>`
	}
	if ext == "pptx" {
		name = "ppt/presentation.xml"
		main = `<p:presentation xmlns:p="urn:ppt"><p:sldIdLst/></p:presentation>`
	}
	parts := map[string]string{name: main, "docProps/core.xml": `<?xml version="1.0"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"><cp:keywords>original-keywords</cp:keywords></cp:coreProperties>`, "[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`}
	for name, raw := range extra {
		parts[name] = raw
	}
	input := filepath.Join(t.TempDir(), "original."+ext)
	file, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, raw := range parts {
		entry, e := writer.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = entry.Write([]byte(raw)); e != nil {
			t.Fatal(e)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()
	return input
}
func partBytes(t *testing.T, input, name string) []byte {
	t.Helper()
	archive, err := zip.OpenReader(input)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name == name {
			reader, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(reader)
			reader.Close()
			if e != nil {
				t.Fatal(e)
			}
			return raw
		}
	}
	t.Fatal("part missing", name)
	return nil
}
func TestCompatibilityXMLSpecialTextAndStoredTime(t *testing.T) {
	for _, ext := range []string{"docx", "pptx", "xlsx"} {
		t.Run(ext, func(t *testing.T) {
			input := compatibilityArchive(t, ext, nil)
			before, _ := os.ReadFile(input)
			output := filepath.Join(t.TempDir(), "marked."+ext)
			text := `A&B -- 中文 <tag> "quoted words"`
			at := time.Date(2002, 3, 4, 5, 6, 7, 0, time.UTC)
			target := "word/document.xml"
			if ext == "xlsx" {
				target = "xl/workbook.xml"
			}
			if ext == "pptx" {
				target = "ppt/presentation.xml"
			}
			if err := watermark.WriteXMLArchiveMetadata(input, output, []string{"docProps/core.xml", target}, text, at); err != nil {
				t.Fatal(err)
			}
			actual, stamp, err := NewWatermarkService().ExtractWatermarkWithTimestamp(output)
			if err != nil || actual != text || stamp != "2002-03-04T05:06:07Z" {
				t.Fatal(actual, stamp, err)
			}
			if !bytes.Equal(partBytes(t, input, target), partBytes(t, output, target)) {
				t.Fatal("body/presentation bytes changed")
			}
			after, _ := os.ReadFile(input)
			if !bytes.Equal(before, after) {
				t.Fatal("original changed")
			}
			publicOutput := filepath.Join(t.TempDir(), "public."+ext)
			start := time.Now().Add(-time.Second)
			if err = NewWatermarkService().AddWatermark(input, publicOutput, text); err != nil {
				t.Fatal(err)
			}
			actual, stamp, err = NewWatermarkService().ExtractWatermarkWithTimestamp(publicOutput)
			if err != nil || actual != text {
				t.Fatal(actual, err)
			}
			added, err := time.Parse(time.RFC3339, stamp)
			if err != nil || added.Before(start) || added.After(time.Now()) {
				t.Fatal("invalid addition time", stamp, err)
			}
		})
	}
}
func TestLegacyPropertiesWithoutTimeDoNotInventTime(t *testing.T) {
	for _, ext := range []string{"docx", "pptx"} {
		input := compatibilityArchive(t, ext, map[string]string{"docProps/core.xml": `<cp:coreProperties xmlns:cp="urn:test"><cp:keywords>Watermark:legacy</cp:keywords></cp:coreProperties>`})
		text, stamp, err := NewWatermarkService().ExtractWatermarkWithTimestamp(input)
		if err != nil || text != "legacy" || stamp != "" {
			t.Fatal(ext, text, stamp, err)
		}
	}
}
func TestCompatibilityServiceRejectsUnsafeInnerZIP(t *testing.T) {
	input := compatibilityArchive(t, "docx", map[string]string{"../../escape.txt": "must not be unpacked"})
	output := filepath.Join(t.TempDir(), "no-output.docx")
	if err := NewWatermarkService().AddWatermark(input, output, "safe body"); !errors.Is(err, ErrFileCorrupted) {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
func TestCompatibilityWriteRejectsDeclaredSignatureStructures(t *testing.T) {
	for _, parts := range []map[string]string{
		{"_xmlsignatures/sig1.xml": "<Signature/>"},
		{"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="signature" Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/origin" Target="other/path.xml"/></Relationships>`},
	} {
		input := compatibilityArchive(t, "docx", parts)
		output := filepath.Join(t.TempDir(), "signed-must-not-write.docx")
		if err := NewWatermarkService().AddWatermark(input, output, "text"); !errors.Is(err, watermark.ErrSignedCompatibility) {
			t.Fatal(err)
		}
		if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
}

func TestRTFCompatibilityDoesNotAppendNullBytes(t *testing.T) {
	for _, source := range []string{`{\rtf1\ansi Original body}`, `{\rtf1\ansi{\info{\author Author}} Original body}`} {
		input := filepath.Join(t.TempDir(), "original.rtf")
		os.WriteFile(input, []byte(source), 0600)
		output := filepath.Join(t.TempDir(), "marked.rtf")
		text := "A&B -- 中文"
		if err := NewWatermarkService().AddWatermark(input, output, text); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasSuffix(raw, []byte("}")) || bytes.Contains(raw, []byte{0}) {
			t.Fatal("RTF ends in unexpected padding")
		}
		actual, stamp, err := NewWatermarkService().ExtractWatermarkWithTimestamp(output)
		if err != nil || actual != text || stamp == "" {
			t.Fatal(actual, stamp, err)
		}
	}
}

// This fixture verifies ODF packaging invariants, not application rendering.
func TestODTStoredMimetypeFirstAndUnchangedContent(t *testing.T) {
	input := filepath.Join(t.TempDir(), "original.odt")
	file, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	parts := []struct {
		name, raw string
		method    uint16
	}{
		{"mimetype", "application/vnd.oasis.opendocument.text", zip.Store},
		{"META-INF/manifest.xml", `<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0"><manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/><manifest:file-entry manifest:full-path="meta.xml" manifest:media-type="text/xml"/></manifest:manifest>`, zip.Deflate},
		{"content.xml", `<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"><office:body><office:text><text:p>Original body</text:p></office:text></office:body></office:document-content>`, zip.Deflate},
		{"meta.xml", `<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"><office:meta/></office:document-meta>`, zip.Deflate},
	}
	for _, part := range parts {
		entry, e := archive.CreateHeader(&zip.FileHeader{Name: part.name, Method: part.method})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = io.WriteString(entry, part.raw); e != nil {
			t.Fatal(e)
		}
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(input)
	output := filepath.Join(t.TempDir(), "marked.odt")
	text := `A&B -- 中文 <tag>`
	if err = NewWatermarkService().AddWatermark(input, output, text); err != nil {
		t.Fatal(err)
	}
	actual, stamp, err := NewWatermarkService().ExtractWatermarkWithTimestamp(output)
	if err != nil || actual != text || stamp == "" {
		t.Fatal(actual, stamp, err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != len(parts) || reader.File[0].Name != "mimetype" || reader.File[0].Method != zip.Store || len(reader.File[0].Extra) != 0 {
		t.Fatal("ODF mimetype/order/entries changed")
	}
	for _, name := range []string{"mimetype", "content.xml", "META-INF/manifest.xml"} {
		if !bytes.Equal(partBytes(t, input, name), partBytes(t, output, name)) {
			t.Fatal("unchanged member changed", name)
		}
	}
	after, _ := os.ReadFile(input)
	if !bytes.Equal(before, after) {
		t.Fatal("original changed")
	}
}
