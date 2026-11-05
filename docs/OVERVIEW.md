# ocr2pdf — project overview

## What it is

`ocr2pdf` turns a scanned document into a **searchable PDF**: the page still looks exactly like the
scan, but every word carries an invisible text layer, so you can select, copy, search and index it.

It is a Go module (`github.com/TheronThayer9999/ocr2pdf`) with a CLI and two example applications
on top. OCR is delegated to the Tesseract command-line tool; everything else — image handling, page
layout, font embedding and the PDF itself — is implemented in this repository, with no CGO.

```
input (image · PDF · URL)
        │
        ▼
   imaging.Loader ──pages{image, DPI}──► ocr.Engine ──lines{words, boxes}──► pdf.Builder ──► PDF
```

## The problem it solves

Scanned documents are pictures: text cannot be copied, searched or indexed. The usual fixes are
either heavyweight (a full OCR suite) or lossy (re-encoding the scan). This project keeps the original
pixels untouched and adds only what is missing: a precise, invisible text layer plus a `ToUnicode`
map so the copied text is the real text, diacritics included.

Typical use cases: digitising paperwork and invoices, making Vietnamese administrative documents
searchable, building an upload-and-download service around a scanner.

## How it works

1. **Load** — `imaging.AutoLoader` dispatches by extension/content: `FileLoader` for images,
   `PDFLoader` for scanned PDFs (pdfcpu `ExtractImagesRaw`), `RemoteLoader` for `http(s)` URLs
   (downloads to a temp file, 128 MB cap by default).
2. **Recognise** — `ocr.Tesseract` writes the page to a temp PNG, runs `tesseract … tsv`, and
   `ocr.TSVParser` groups level-5 words into level-4 lines, keeping each word's pixel box and confidence.
3. **Place** — `Converter` converts pixels to points (`× 72 / DPI`), so the text sits exactly on the
   pixels it came from.
4. **Render** — `pdf.Builder` writes an image XObject per page plus invisible text (`3 Tr`) at the
   OCR coordinates, using an embedded subset of a system TTF.

## Design decisions

| Decision | Why |
|---|---|
| **Own PDF writer** instead of a library | The requirement is a *word-level* invisible text layer with horizontal scaling (`Tz`) to match each OCR box. No Go library exposes both; `gopdf` has no text rendering mode, `go-pdf/fpdf` has `3 Tr` but no per-word `Tz`, `pdfcpu` writes text as stamps/watermarks. The writer is ~600 lines and has zero third-party dependencies. |
| **Tesseract via CLI** | No CGO, no native linking; the same binary the user already has. The wrapper is `ocr/` (finder → runner → parser), all injectable. |
| **TSV as the source of geometry** | `tsv` gives per-word boxes and confidence in one pass, without parsing hOCR or a second OCR run. |
| **Font subsetting by hand** | Embedding full Arial costs ~1 MB per PDF; subsetting to used glyphs brings it to ~55 KB. `fonts/subset.go` rewrites `glyf`/`loca`/`hmtx`/`maxp` and fixes checksums. |
| **JPEG passthrough** | A 200 dpi A4 scan is already JPEG; decoding and re-compressing it would inflate the file and lose quality. It is embedded with `/Filter /DCTDecode`. |
| **PNG-predictor for other images** | Reusing the IDAT stream with `/Predictor 15` keeps PNG-sourced pages small without extra compression work. |
| **Automatic language detection** | `eng` on Vietnamese silently corrupts diacritics (`phê phán` → `phé phan`) because the text layer copies what OCR produced. Default is `auto`: page one is OCR'd with each installed `traineddata` and the language with the highest length-weighted confidence wins, at the cost of one extra pass on page one only. |
| **Interfaces at every seam** | `imaging.Loader`, `ocr.Engine`, `pdf.Builder`, `pdf.ImageEncoder`, `fonts.Font`, `fonts.Loader` — each replaceable through `ocr2pdf.New(...)` options. |

## Packages

| Package | Responsibility |
|---|---|
| `ocr2pdf` (root) | `Converter` facade, options, auto language, output naming |
| `ocr/` | `Engine`, Tesseract CLI wrapper, TSV parser, binary finder, runner |
| `imaging/` | page loading: file, PDF (pdfcpu), URL, DPI detection |
| `pdf/` | document/object writer, page builder, image encoder, font resource, text layer |
| `fonts/` | TTF loading, metrics, glyph/unicode maps, subsetter |
| `cmd/ocr2pdf/` | reference CLI |

## API surface

```go
res, err := ocr2pdf.Convert("scan.pdf")                        // language auto-detected
res, err := ocr2pdf.Convert("https://host/files/scan.pdf")     // URL input
res, err := ocr2pdf.Convert("scan.pdf", ocr2pdf.WithLanguage("vie"))
data, res, err := conv.Render(ctx, "scan.pdf")                 // bytes, no file written
```

Package-level helpers (`Convert`, `ConvertContext`, `ConvertNative`, `DefaultOutputPath`, `Languages`)
cover the common cases; `Converter` + `Option`s cover everything else, including swapping the OCR
engine, the PDF builder, the image loader or the font loader.

## Examples in this repository

| Path | What it shows |
|---|---|
| `examples/simple` | The shortest possible program (~15 lines). |
| `examples/be` | A Gin HTTP API: `POST /api/convert` accepts an upload **or** a `url`, returns the PDF, exposes `X-Pdf-Language/Pages/Words`. |
| `examples/fe/ocr` | An Angular 22 app: pick a file, POST it to the API, preview the result in a PDF viewer component. |

## Quality

- **Tests** — TSV grouping/confidence, PDF structure/xref/page size, DPI and URL loading, output
  naming, auto-language scoring, plus Angular component and service tests. None of the Go tests need
  Tesseract.
- **CI** — GitHub Actions runs `go build`, `go vet`, `go test -race`, a `gofmt` gate and
  `golangci-lint` on every push and pull request.
- **Manual verification** — outputs are checked with `pdfcpu validate` and PyMuPDF (page size, image
  filter, extracted text).

## Measured behaviour

| Input | Output |
|---|---|
| `scan200.pdf` — A4, 200 dpi, 1654×2339 JPEG | 278 KB, 1 page 595×841 pt, 864 characters, image kept as `DCTDecode` |
| `image_one.png` — 987×826 at 120 dpi | 129 KB, 592×496 pt, 862 characters |
| `demo.png` — Vietnamese text | 143 KB, 30 lines, 312 words, ~0.7–1.4 s, copies `phê phán` correctly |

Language detection on the same image: `vie` scores 88.1 mean confidence versus 75.0 for `eng`;
on an English page the ranking flips (96.5 vs 87.8).

## Limitations

- Input PDFs must be scans (an image per page). Vector/text PDFs need rasterising first.
- Page size follows the image and its DPI; the original PDF MediaBox is not preserved.
- Text quality is bounded by Tesseract and the `traineddata`; skewed scans should be deskewed.
- `ConvertNative` (Tesseract writes the PDF itself) only accepts local images.
- OCR is CPU bound: one Tesseract process per page.

## Possible next steps

- Job queue + `GET /api/jobs/{id}` for long documents.
- Pre-processing pipeline (deskew, denoise, upscale) as an optional `imaging` decorator.
- PDF/A output and document metadata.
- More input formats (TIFF multi-page, HEIC) as additional `imaging.Loader`s.
