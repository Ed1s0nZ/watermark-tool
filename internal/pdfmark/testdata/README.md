# Synthetic PDF fixture

`source.pdf` is a deterministic three-page ReportLab fixture created specifically for this repository. It contains no user data, external links or scripts.

1. Cover page, used to test exclusion.
2. Content with one interactive text field (`preserved_field`, value `unchanged-value`).
3. Appendix, used to test a different per-page marker body.

Tests copy it into a private temporary directory and only modify the copies. `make_fixture.py` refuses to overwrite an existing reference file; it is an optional maintenance tool requiring ReportLab, never a server or Go-test runtime dependency.

To export a synthetic marked result for independent rendering:

```sh
mkdir -p /tmp/watermark-pdf-qa
WATERMARK_PDF_QA_OUTPUT=/tmp/watermark-pdf-qa/marked.pdf go test ./internal/pdfmark -run '^TestProtectActualPagesCoverOverridesAndReparse$' -count=1
pdftoppm -scale-to 1280 -png internal/pdfmark/testdata/source.pdf /tmp/watermark-pdf-qa/original
pdftoppm -scale-to 1280 -png /tmp/watermark-pdf-qa/marked.pdf /tmp/watermark-pdf-qa/marked
```

Compare every page's rendered pixels and decoded content, and check canonical AcroForm field values, effective widget values and nonempty normal appearance streams. Marker bodies live in page objects and are not encrypted; marker authentication does not authenticate arbitrary changes to visual page content.
