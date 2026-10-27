package pdfmark

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func blankPDF(pages int) []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", ""}
	kids := []string{}
	for p := 0; p < pages; p++ {
		id := len(objects) + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", id))
		objects = append(objects, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << >> >>")
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Count %d /Kids [ %s ] >>", pages, strings.Join(kids, " "))
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for index, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}
func TestActualPageCountAndFileSizeLimits(t *testing.T) {
	input := filepath.Join(t.TempDir(), "many-pages.pdf")
	if err := os.WriteFile(input, blankPDF(MaxPages+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(t.Context(), input, nil); !errors.Is(err, ErrLimit) {
		t.Fatal("page count limit", err)
	}
	large := filepath.Join(t.TempDir(), "large.pdf")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxOutputBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := Inspect(t.Context(), large, nil); !errors.Is(err, ErrLimit) {
		t.Fatal("input limit", err)
	}
	var out bytes.Buffer
	writer := &boundedWriter{writer: &out, remaining: 4}
	if _, err = writer.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Write([]byte("5")); !errors.Is(err, ErrLimit) || out.String() != "1234" {
		t.Fatal("output overrun", out.String(), err)
	}
}
func TestDeclaredSignatureIsNotRewritten(t *testing.T) {
	input, _ := original(t)
	doc, _, err := openDocument(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := doc.DereferenceArray(doc.Form["Fields"])
	if err != nil {
		t.Fatal(err)
	}
	field, err := doc.DereferenceDict(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	field.Update("FT", types.Name("Sig"))
	field.Update("V", types.Dict{"Type": types.Name("Sig"), "Filter": types.Name("Adobe.PPKLite"), "SubFilter": types.Name("adbe.pkcs7.detached"), "Contents": types.HexLiteral("00"), "ByteRange": types.Array{types.Integer(0), types.Integer(1), types.Integer(2), types.Integer(1)}})
	doc.Form.Update("SigFlags", types.Integer(3))
	declared := filepath.Join(t.TempDir(), "declared-signature.pdf")
	f, err := os.Create(declared)
	if err != nil {
		t.Fatal(err)
	}
	err = api.WriteContext(t.Context(), doc, f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	// This is a structural signature fixture, not a claim of a valid cryptographic signature.
	output := filepath.Join(t.TempDir(), "must-not-write.pdf")
	if _, err = Protect(t.Context(), declared, output, params(), testKey); !errors.Is(err, ErrSigned) {
		t.Fatal("signature declaration was not rejected explicitly", err)
	}
	if _, err = os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("signed output was committed", err)
	}
}
