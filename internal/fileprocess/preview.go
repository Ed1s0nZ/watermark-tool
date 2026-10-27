package fileprocess

import (
	"context"
	"path"
	"strings"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/filebundle"
	"watermark-tool/internal/output"
	"watermark-tool/internal/pdfmark"
)

func outputOptions(p *output.Options) (*output.Options, error) { return output.Normalize(p) }
func imageOptions(ext string, p *output.Options) *output.Options {
	if p != nil {
		return p
	}
	if ext == "jpg" || ext == "jpeg" {
		return &output.Options{Format: "jpeg", JPEGQuality: 90}
	}
	return nil
}
func pdfParams(id Identity, options Options) pdfmark.Params {
	p := pdfmark.Params{DocumentID: id.DocumentID, KeyID: id.KeyID, CreatedAt: id.CreatedAt, Text: options.Text}
	if o := options.PDF; o != nil {
		p.Pages = o.Pages
		p.SkipCover = o.SkipCover
		p.Overrides = o.Overrides
		if o.Text != "" {
			p.Text = o.Text
		}
	}
	return p
}

// Preview validates actual private input bytes; the caller must supply server-owned records.
// PDF plans describe selected page objects, not rendered visual page previews.
func Preview(ctx context.Context, source filebundle.Record, id Identity, options Options) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	if err := validateOptions(id, options); err != nil {
		return Plan{}, err
	}
	if err := checkSource(ctx, source); err != nil {
		return Plan{}, err
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(source.RelativePath), "."))
	p := Plan{OutputExtension: ext, InputSHA256: source.SHA256}
	switch ext {
	case "png", "jpg", "jpeg":
		if !validText(options.Text, blind.MaxTextBytes) {
			return Plan{}, blind.ErrText
		}
		img, err := blind.Load(source.PhysicalPath)
		if err != nil {
			return Plan{}, err
		}
		if err = blind.Validate(img); err != nil {
			return Plan{}, err
		}
		p.Kind = "image"
		p.Algorithm = blind.Algorithm
		p.Authenticated = true
		p.OutputExtension = output.Extension(imageOptions(ext, options.Image))
		p.Boundary = "payload_authentication_not_image_content_signature"
	case "pdf":
		report, pages, err := pdfmark.Preview(ctx, source.PhysicalPath, pdfParams(id, options))
		if err != nil {
			return Plan{}, err
		}
		p.Kind = "pdf"
		p.Algorithm = pdfmark.Algorithm
		p.Authenticated = true
		p.PageCount = report.PageCount
		p.Pages = pages
		p.Boundary = "page_marker_authentication_not_visual_content_signature"
	case "docx", "pptx", "xlsx", "odt", "rtf":
		if err := validateOffice(ctx, source.PhysicalPath, ext); err != nil {
			return Plan{}, err
		}
		p.Kind = "document"
		p.Algorithm = "compat-xml-v2"
		if ext == "rtf" {
			p.Algorithm = "compat-rtf-v1"
		}
		p.Boundary = "recoverable_metadata_not_secret_or_issuer_authentication"
	default:
		return Plan{}, ErrUnsupported
	}
	if err := checkSource(ctx, source); err != nil {
		return Plan{}, err
	}
	return p, nil
}
