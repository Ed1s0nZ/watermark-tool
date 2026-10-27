package fileprocess

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"strconv"
	"strings"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/watermark"
)

func metadataTargets(ext string) []string {
	switch ext {
	case "docx":
		return []string{"docProps/core.xml", "word/document.xml"}
	case "pptx":
		return []string{"docProps/core.xml", "ppt/presentation.xml"}
	case "xlsx":
		return []string{"docProps/core.xml", "xl/workbook.xml"}
	case "odt":
		return []string{"meta.xml", "content.xml"}
	}
	return nil
}
func validateXML(ctx context.Context, r io.Reader) (xml.Name, error) {
	d := xml.NewDecoder(contextReader{ctx, r})
	depth, roots := 0, 0
	var root xml.Name
	for {
		token, err := d.Token()
		if err == io.EOF {
			if depth != 0 || roots != 1 {
				return root, ErrStructure
			}
			return root, nil
		}
		if err != nil {
			return root, ErrStructure
		}
		switch t := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				root = t.Name
			}
			depth++
			if depth > 128 || roots > 1 {
				return root, ErrStructure
			}
			for _, a := range t.Attr {
				if a.Name.Local == "Type" && strings.HasPrefix(a.Value, "http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/") {
					return root, watermark.ErrSignedCompatibility
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(t)) != 0 {
				return root, ErrStructure
			}
		case xml.Directive:
			return root, ErrStructure // no DTD/entities in accepted Office XML
		}
	}
}
func validateOffice(ctx context.Context, input, ext string) error {
	if ext == "rtf" {
		raw, err := os.ReadFile(input)
		if err != nil {
			return ErrIO
		}
		return validateRTF(ctx, raw)
	}
	archive, err := filebundle.Open(ctx, input, filebundle.OfficeLimits())
	if err != nil {
		return err
	}
	err = archive.Validate(ctx)
	archive.Close()
	if err != nil {
		return err
	}
	reader, err := zip.OpenReader(input)
	if err != nil {
		return ErrStructure
	}
	defer reader.Close()
	required := map[string]struct{ name, space string }{
		"docx": {"word/document.xml", "http://schemas.openxmlformats.org/wordprocessingml/2006/main"},
		"pptx": {"ppt/presentation.xml", "http://schemas.openxmlformats.org/presentationml/2006/main"},
		"xlsx": {"xl/workbook.xml", "http://schemas.openxmlformats.org/spreadsheetml/2006/main"},
		"odt":  {"content.xml", "urn:oasis:names:tc:opendocument:xmlns:office:1.0"},
	}[ext]
	rootName := map[string]string{"docx": "document", "pptx": "presentation", "xlsx": "workbook", "odt": "document-content"}[ext]
	found, types, manifest, mimetype := false, false, false, false
	for _, file := range reader.File {
		if err = ctx.Err(); err != nil {
			return err
		}
		lower := strings.ToLower(file.Name)
		if strings.HasPrefix(lower, "_xmlsignatures/") || lower == "meta-inf/documentsignatures.xml" || lower == "meta-inf/macrosignatures.xml" {
			return watermark.ErrSignedCompatibility
		}
		if file.Name == "mimetype" && ext == "odt" {
			r, e := file.Open()
			if e != nil {
				return e
			}
			raw, e := io.ReadAll(io.LimitReader(r, 128))
			r.Close()
			if e != nil || string(raw) != "application/vnd.oasis.opendocument.text" || reader.File[0] != file || file.Method != zip.Store || len(file.Extra) != 0 {
				return ErrStructure
			}
			mimetype = true
		}
		if !strings.HasSuffix(lower, ".xml") && !strings.HasSuffix(lower, ".rels") {
			continue
		}
		r, e := file.Open()
		if e != nil {
			return ErrStructure
		}
		root, e := validateXML(ctx, r)
		r.Close()
		if e != nil {
			return e
		}
		if file.Name == required.name {
			if root.Local != rootName || root.Space != required.space {
				return ErrStructure
			}
			found = true
		}
		if file.Name == "[Content_Types].xml" && root.Local == "Types" && root.Space == "http://schemas.openxmlformats.org/package/2006/content-types" {
			types = true
		}
		if file.Name == "META-INF/manifest.xml" && root.Local == "manifest" && root.Space == "urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" {
			manifest = true
		}
	}
	if !found || ext == "odt" && (!mimetype || !manifest) || ext != "odt" && !types {
		return ErrStructure
	}
	return nil
}

// validateRTF counts actual groups, escaped braces and binary spans; regexes cannot do this.
func validateRTF(ctx context.Context, raw []byte) error {
	if !bytes.HasPrefix(raw, []byte(`{\rtf1`)) {
		return ErrStructure
	}
	depth := 0
	for i := 0; i < len(raw); i++ {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		switch raw[i] {
		case 0:
			return ErrStructure
		case '{':
			depth++
			if depth > 128 {
				return ErrStructure
			}
		case '}':
			depth--
			if depth < 0 {
				return ErrStructure
			}
			if depth == 0 && len(bytes.TrimSpace(raw[i+1:])) != 0 {
				return ErrStructure
			}
		case '\\':
			i++
			if i >= len(raw) {
				return ErrStructure
			}
			if raw[i] == '\'' {
				if i+2 >= len(raw) {
					return ErrStructure
				}
				if _, err := strconv.ParseUint(string(raw[i+1:i+3]), 16, 8); err != nil {
					return ErrStructure
				}
				i += 2
				continue
			}
			if raw[i] < 'a' || raw[i] > 'z' {
				continue
			}
			start := i
			for i < len(raw) && raw[i] >= 'a' && raw[i] <= 'z' {
				i++
			}
			word := string(raw[start:i])
			start = i
			if i < len(raw) && raw[i] == '-' {
				i++
			}
			for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
				i++
			}
			number := string(raw[start:i])
			if i < len(raw) && raw[i] == ' ' {
				i++
			}
			if word == "bin" {
				n, err := strconv.Atoi(number)
				if err != nil || n < 0 || n > len(raw)-i {
					return ErrStructure
				}
				i += n
			}
			i--
		}
	}
	if depth != 0 {
		return ErrStructure
	}
	return nil
}
