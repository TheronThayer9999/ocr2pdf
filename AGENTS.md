# AGENTS.md

## Commands

- Build the CLI: `go build -o ocr2pdf.exe ./cmd/ocr2pdf`
- Run: `.\ocr2pdf.exe -in data\scan200.pdf` (language is auto-detected; output is auto-named `...-searchable.pdf`)
- Tests (no Tesseract needed): `go test ./...`
- Single test: `go test ./ocr -run TestTSVParser -v` · `go test ./pdf -run TestBuild -v`
- Before finishing: `go vet ./...`, `gofmt -l .` (must be empty), `golangci-lint run ./...`

## Repo layout — three modules

- Root module `github.com/TheronThayer9999/ocr2pdf`: the library, `cmd/ocr2pdf`, and `examples/simple`.
- `examples/be` is a **separate** Go module (Gin HTTP API) with `replace ... => ../..`, so it compiles against
  local code. `go test ./...` and CI do **not** cover it — run `cd examples/be; go build ./...; go vet ./...` explicitly.
- `examples/fe/ocr` is an Angular 22 app (own `package.json`, not a Go module).
- `gofmt -l .` from the root does walk into `examples/be`.

## Frontend (`examples/fe/ocr`)

- `npm start` → http://localhost:4200 · `npx ng build` · `npx ng test --watch=false` (plain `ng test` stays in watch mode).
- Angular 22: standalone components, signals, **zoneless** (no zone.js), SSR enabled with prerendering.
  Browser-only APIs must be guarded (`isPlatformBrowser`, `afterNextRender`) or `ng build` fails.
- API base URL is hardcoded in `src/app/ocr-api.ts` (`http://localhost:8080`); the API enables CORS for it.
- Tailwind 4 is wired through `.postcssrc.json` + `@import 'tailwindcss'` in `src/styles.css`.

## Environment (this machine)

- Tesseract is **not on PATH**; the binary lives at `C:\Program Files\Tesseract-OCR\tesseract.exe`
  (v5.4, langs: `eng`, `vie`, `osd`). The code auto-discovers it; override with `TESSERACT_PATH` or `-tesseract`.
- **Never edit source files with PowerShell** (`Get-Content | Set-Content`): PS 5.1 re-encodes and turns
  Vietnamese characters in comments/strings into mojibake. Use the edit/write tools instead.
- Available for verification: Python + `pymupdf` (read text/rects/images of a PDF), `PIL`; `pdfcpu` (Go) to validate.
  No poppler/pdftoppm.
- `go.mod` pins `go 1.26.0` because `pdfcpu` and `x/image` require 1.26; do not raise it to 1.27.
  golangci-lint 2.14 is built with go1.26, so on a Go 1.27 machine it panics with
  `file requires newer Go version go1.27`; run it as `$env:GOTOOLCHAIN="go1.26.5"; golangci-lint run ./...`.
- Sample inputs: `data/image_one.png`, `data/scan200.pdf` (A4 200 dpi), `examples/simple/demo.png` (Vietnamese).

## Architecture — keep the boundaries

- Pipeline: `imaging.Loader` → `ocr.Engine` → `pdf.Builder`; `Converter` (root package) depends **only** on interfaces.
- `pdf.Builder` returns the whole PDF as `[]byte`; `pdf.StreamingBuilder` (`BuildTo(io.Writer, Input)`)
  is the optional long-document path: it writes object by object and calls `pdf.Image.Open` just
  before writing that page, so RAM stays O(one page). Keep both in sync — `Build` is `BuildTo` into
  a `bytes.Buffer`.
- `imaging.AutoLoader` handles local paths, `imaging.RemoteLoader` downloads `http(s)` inputs to a temp file first.
- Language defaults to `auto`: page one is OCR'd once per installed `traineddata` and the highest mean
  confidence wins, so a run costs one extra OCR pass. Explicit `WithLanguage("vie")` skips detection.
- Public packages on purpose (so others can import the module): `ocr/`, `imaging/`, `pdf/`, `fonts/`. No `internal/`.
- New input format = new `imaging.Loader`; new OCR engine = implement `ocr.Engine` (pass it via `WithEngine`).
- Module path `github.com/TheronThayer9999/ocr2pdf` matches the GitHub repo; forking to another account means changing `module` in `go.mod` and rewriting imports.

## PDF layer invariants (easy to break)

- The embedded font is Type0 / CIDFontType2 / Identity-H with **CID = glyph id**. `fonts.Font.Subset` returns a `remap`
  (old glyph → new glyph) that must be applied in **all 3 places**: the `/W` array, the `/ToUnicode` CMap, and the hex string in the content stream.
- `FontDescriptor` (`Ascent`, `Descent`, `CapHeight`, `FontBBox`) is in **1/1000 em**, not font units.
- Page size = `pixels × 72 / DPI`. DPI comes from PNG `pHYs` / JPEG JFIF, defaulting to 200;
  for PDFs it is `imageWidth / (pageWidthPoint / 72)`.
- Invisible text uses `3 Tr`; a line shares one font size + baseline, and each word uses `Tz` to match the OCR box width.
- JPEG images must be embedded as-is (`/Filter /DCTDecode`), never decoded and re-compressed; other images go through the
  PNG-predictor path (`/Predictor 15`) in `pdf/image.go`.

## Known limitations

- The input PDF must be a scan/image (read with pdfcpu `ExtractImagesRaw`). Vector/text PDFs return an error asking you to rasterize first.
- `-lang eng` mangles Vietnamese diacritics (`trur©c`, `phê phán` → `phé phan`); the default `auto` avoids it when `vie` is installed.
- Output pages are sized from the image + DPI; the original PDF MediaBox is not preserved.
- `ConvertNative` (Tesseract writes the PDF itself) needs a local file and only supports images.
