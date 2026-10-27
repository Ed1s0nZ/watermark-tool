# PDF parser attribution

The platform's `pdf-page-v1` marker protocol and selection/verification code are implemented in `internal/pdfmark`. PDF object parsing and serialization use **pdfcpu v0.16.3**, under the [Apache License 2.0](pdfcpu/LICENSE.txt).

- Source: https://github.com/pdfcpu/pdfcpu/tree/v0.16.3
- Pinned commit: `35ea259eb27d5a607f797edd41fd8a9e50a1f818`
- Minimum Go version: 1.26.0
- No `blind_watermark` or Python server dependency is introduced. The fixture generator uses ReportLab only to prepare the committed synthetic PDF; Go tests and the server do not execute it.

The exact dependency graph and integrity hashes are recorded in `go.mod`/`go.sum`. Upstream source modules, including their own notices, remain available through Go modules; preserve applicable upstream licenses/notices when redistributing a build.
