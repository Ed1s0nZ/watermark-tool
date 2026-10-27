package fileprocess

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/output"
	"watermark-tool/internal/pdfmark"
	"watermark-tool/internal/quality"
	"watermark-tool/internal/watermark"
)

func identity() Identity {
	return Identity{DocumentID: strings.Repeat("a", 32), KeyID: "test-key", CreatedAt: time.Date(2002, 3, 4, 5, 6, 7, 0, time.UTC)}
}
func record(t *testing.T, fixture, relative string) filebundle.Record {
	t.Helper()
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	// Every attempt starts from a fresh private copy. The original is never a destination.
	input := filepath.Join(t.TempDir(), "opaque-private-id")
	if err = os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	digest, n, err := hashFile(context.Background(), input, 50<<20)
	if err != nil {
		t.Fatal(err)
	}
	return filebundle.Record{ID: identity().DocumentID, RelativePath: relative, PhysicalPath: input, SHA256: digest, Bytes: n}
}
func imageRecord(t *testing.T) filebundle.Record {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(70 + x%120), uint8(60 + y%100), uint8(90 + (x+y)%100), 255})
		}
	}
	name := filepath.Join(t.TempDir(), "original.png")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return record(t, name, "images/原始.png")
}
func member(t *testing.T, name, part string) []byte {
	t.Helper()
	r, err := zip.OpenReader(name)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == part {
			reader, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			raw, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			return raw
		}
	}
	t.Fatal("missing part", part)
	return nil
}
func TestActualDocumentFinalRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x72}, 32)
	for _, ext := range []string{"docx", "pptx", "xlsx", "odt", "rtf"} {
		t.Run(ext, func(t *testing.T) {
			source := record(t, "testdata/source."+ext, "folder/原件."+ext)
			before, _ := os.ReadFile(source.PhysicalPath)
			destination := filepath.Join(t.TempDir(), "output."+ext)
			options := Options{Text: `A&B -- 中文 <tag> "two words"`}
			plan, err := Preview(context.Background(), source, identity(), options)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Authenticated || plan.Kind != "document" {
				t.Fatal(plan)
			}
			result, err := Execute(context.Background(), source, destination, identity(), options, key)
			if err != nil {
				t.Fatal(err)
			}
			if !result.FinalVerified || result.RecordedAt != "2002-03-04T05:06:07Z" || result.OutputSHA256 == source.SHA256 {
				t.Fatal(result)
			}
			digest, n, _ := hashFile(context.Background(), destination, 128<<20)
			if digest != result.OutputSHA256 || n != result.OutputBytes {
				t.Fatal("delivered bytes differ")
			}
			after, _ := os.ReadFile(source.PhysicalPath)
			if !bytes.Equal(before, after) {
				t.Fatal("original changed")
			}
			part := map[string]string{"docx": "word/document.xml", "pptx": "ppt/slides/slide1.xml", "xlsx": "xl/worksheets/sheet1.xml", "odt": "content.xml"}[ext]
			if part != "" && !bytes.Equal(member(t, source.PhysicalPath, part), member(t, destination, part)) {
				t.Fatal("body changed")
			}
			if dir := os.Getenv("WATERMARK_FILE_QA_OUTPUT"); dir != "" {
				raw, _ := os.ReadFile(destination)
				if err = os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "marked."+ext), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestPDFFinalSelectedPages(t *testing.T) {
	source := record(t, "../pdfmark/testdata/source.pdf", "docs/source.pdf")
	key := bytes.Repeat([]byte{0x51}, 32)
	options := Options{Text: "common", PDF: &PDFOptions{Pages: "1-3", SkipCover: true, Text: "Page {page}/{pages}", Overrides: map[int]string{3: "Final page 中文"}}}
	plan, err := Preview(context.Background(), source, identity(), options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Pages) != 2 || plan.Pages[0].Page != 2 || plan.Pages[1].Text != "Final page 中文" {
		t.Fatal(plan)
	}
	destination := filepath.Join(t.TempDir(), "delivered.pdf")
	result, err := Execute(context.Background(), source, destination, identity(), options, key)
	if err != nil {
		t.Fatal(err)
	}
	if !result.FinalVerified || !result.Plan.Authenticated || result.PDF == nil || !result.PDF.FinalVerified {
		t.Fatal(result)
	}
	report, err := pdfmark.Inspect(context.Background(), destination, func(id string) ([]byte, bool) { return key, id == identity().KeyID })
	if err != nil {
		t.Fatal(err)
	}
	if report.Pages[0].Status != "unmarked" || !report.Pages[1].MarkerVerified || report.Pages[2].Marker.Text != "Final page 中文" {
		t.Fatal(report)
	}
}
func TestImageFinalBytesAndQualityGate(t *testing.T) {
	source := imageRecord(t)
	key := bytes.Repeat([]byte{0x33}, 32)
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			options := Options{Text: "test 中文", Image: &output.Options{Format: format}}
			destination := filepath.Join(t.TempDir(), "output")
			result, err := Execute(context.Background(), source, destination, identity(), options, key)
			if err != nil {
				t.Fatal(err)
			}
			if !result.FinalVerified || !result.Plan.Authenticated || result.Quality == nil || !result.Quality.Verified || result.Quality.Final == nil {
				t.Fatal(result)
			}
			actual, err := blind.ExtractFile(destination, key)
			if err != nil || actual.Watermark != options.Text || actual.Timestamp != result.RecordedAt {
				t.Fatal(actual, err)
			}
			if _, err = Execute(context.Background(), source, destination, identity(), options, key); !errors.Is(err, ErrIO) {
				t.Fatal("existing destination not refused", err)
			}
		})
	}
	minimum := 60.0
	destination := filepath.Join(t.TempDir(), "must-not-exist")
	_, err := Execute(context.Background(), source, destination, identity(), Options{Text: "test", Quality: quality.Policy{MinimumPSNR: &minimum}}, key)
	if !errors.Is(err, quality.ErrThreshold) {
		t.Fatal(err)
	}
	if _, err = os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
func TestChangedCancelledAndUnsupportedLeaveNoOutput(t *testing.T) {
	source := record(t, "testdata/source.docx", "folder/other.exe")
	key := bytes.Repeat([]byte{0x12}, 32)
	dest := filepath.Join(t.TempDir(), "absent")
	if _, err := Execute(context.Background(), source, dest, identity(), Options{Text: "text"}, key); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	source.RelativePath = "doc.docx"
	source.SHA256 = strings.Repeat("0", 64)
	if _, err := Execute(context.Background(), source, dest, identity(), Options{Text: "text"}, key); !errors.Is(err, ErrSource) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Execute(ctx, source, dest, identity(), Options{Text: "text"}, key); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
func TestRTFStructuralParsing(t *testing.T) {
	for _, raw := range []string{`{\rtf1 text \{escaped\} \'e9}`, `{\rtf1\bin4 {}{} body}`} {
		if err := validateRTF(context.Background(), []byte(raw)); err != nil {
			t.Fatal(raw, err)
		}
	}
	for _, raw := range []string{`{\rtf1 unclosed`, `{\rtf1}trailing`, `{\rtf1\'gg}`, `{\rtf1\bin100 no}`, `{\rtf1\bin-1 }`, "{\\rtf1\x00}"} {
		if err := validateRTF(context.Background(), []byte(raw)); !errors.Is(err, ErrStructure) {
			t.Fatal(raw, err)
		}
	}
}
func TestXMLStructureAndSignatures(t *testing.T) {
	for _, raw := range []string{`<a/><b/>`, `<a><b></a>`, `<!DOCTYPE a><a/>`, strings.Repeat("<a>", 129) + strings.Repeat("</a>", 129)} {
		if _, err := validateXML(context.Background(), strings.NewReader(raw)); !errors.Is(err, ErrStructure) {
			t.Fatal(err)
		}
	}
	_, err := validateXML(context.Background(), strings.NewReader(`<Relationships><Relationship Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/origin"/></Relationships>`))
	if !errors.Is(err, watermark.ErrSignedCompatibility) {
		t.Fatal(err)
	}
}
