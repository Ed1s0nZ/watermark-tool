# Synthetic immutable originals

`source.docx`, `source.pptx`, `source.xlsx` were generated with python-docx, python-pptx and openpyxl; they contain synthetic paragraphs, a table, a slide and a formula. `source.odt` is an explicitly assembled ODF 1.2 package with first uncompressed mimetype; `source.rtf` is synthetic RTF. No personal content or downloaded library code.

`generate.py` uses the bundled authoring Python and refuses to overwrite any originals. These Python packages are fixture-authoring/independent QA tools, not application runtime dependencies. Tests first copy every original to a random private physical filename and operate only on that copy; output is always another path. The separate PDF fixture is reused read-only from pdfmark/testdata/source.pdf.

Independent QA reopened final DOCX/PPTX/XLSX with their respective authoring libraries and checked all original body members byte for byte, as recorded in docs/feature/platform-expansion/file-evidence/application-roundtrip.json. This is not a claim of desktop Microsoft Office/LibreOffice visual rendering.
