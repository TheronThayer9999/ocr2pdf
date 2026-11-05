# ocr2pdf

Turn an **image** or a **scanned PDF** (for example a Word document printed and scanned at ~200 dpi)
into a **PDF with an invisible text layer**: it looks exactly like the original, but you can select,
copy and search it like real text.

```bash
ocr2pdf.exe -in data\scan200.pdf -lang vie     # -> data\scan200-searchable.pdf
```

An A4 scan at 200 dpi (1654×2339) becomes a one-page A4 PDF (595×841 pt), ~278 KB, and copies out
correct Vietnamese with diacritics.

## Features

- OCR through the Tesseract CLI — **no CGO**, no native library dependencies.
- Input can be an **image** (png/jpg/gif) or a **multi-page PDF**; the output PDF has the same page count.
- JPEG images inside a scanned PDF are embedded as-is (`DCTDecode`): no re-encoding, no quality loss.
- Other images are compressed with zlib + a PNG-style filter (`Predictor 15`), keeping files small.
- The embedded font is **subset** to the glyphs actually used (~55 KB instead of the full ~1 MB Arial).
- A complete `ToUnicode` map means copy/search returns the right characters, Vietnamese included (`KỊCH BẢN`).

## Requirements

- Go 1.26+ (see `go.mod`).
- [Tesseract OCR 5.x](https://github.com/UB-Mannheim/tesseract/wiki) plus the `traineddata` for the language you need.

The binary is discovered in this order: the `-tesseract` flag → the `TESSERACT_PATH` environment variable → `PATH`
→ `%ProgramFiles%\Tesseract-OCR\tesseract.exe` → `%LOCALAPPDATA%\Programs\Tesseract-OCR\tesseract.exe`
→ `/usr/bin/tesseract`, `/usr/local/bin/tesseract`.

```powershell
# quick check
& "C:\Program Files\Tesseract-OCR\tesseract.exe" --list-langs
```

## Build & run the CLI

```powershell
go build -o ocr2pdf.exe ./cmd/ocr2pdf

.\ocr2pdf.exe -in data\image_one.png -out data\image_one.pdf -lang vie
.\ocr2pdf.exe -in data\scan200.pdf -lang vie          # output is named ...-searchable.pdf
```

| Flag | Default | Meaning |
|---|---|---|
| `-in` | – | input image (png/jpg/gif) or PDF |
| `-out` | derived | `x.png` → `x.pdf`; `x.pdf` → `x-searchable.pdf` (never overwrites the input) |
| `-lang` | `eng` | Tesseract language: `eng`, `vie`, `vie+eng` |
| `-psm` | `3` | page segmentation mode: 3 auto, 6 single block, 7 single line, 11 sparse text |
| `-dpi` | `0` | force a DPI; 0 = read image metadata (PNG pHYs / JFIF) or derive it from the PDF page size |
| `-conf` | `30` | drop words below this confidence |
| `-tesseract` | auto | path to `tesseract.exe` |
| `-font` | system font | TTF for the text layer (default Arial → Segoe UI → Tahoma → DejaVu) |
| `-native` | `false` | let Tesseract generate the PDF itself (images only, layout not under our control) |

> `-lang eng` on Vietnamese documents produces mangled characters (`trur©c`, `Phigu`). Use `-lang vie` when the document has diacritics.

## Using it as a module

```bash
go get github.com/TheronThayer9999/ocr2pdf
```

> The module path matches the repository: `github.com/TheronThayer9999/ocr2pdf`. If you fork it to another
> account, change the `module ...` line in `go.mod` and update the import paths.

Shortest path — every parameter has a default:

```go
res, err := ocr2pdf.Convert("scan.pdf") // -> scan-searchable.pdf
```

Override parameters when needed:

```go
res, err := ocr2pdf.Convert("scan.pdf",
	ocr2pdf.WithLanguage("vie"),       // default "eng"
	ocr2pdf.WithOutput("result.pdf"),  // default: derived from the input name
)
```

A complete program:

```go
package main

import (
	"fmt"
	"log"

	"github.com/TheronThayer9999/ocr2pdf"
)

func main() {
	res, err := ocr2pdf.Convert("scan.pdf", ocr2pdf.WithLanguage("vie"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res) // scan-searchable.pdf (277.7 KB) - 1 page, 23 lines, 165 words - 1.05s
}
```

Package-level functions — nothing to initialize:

| Function | Use it when |
|---|---|
| `Convert(in, opts...)` | shortest path, output name is derived |
| `ConvertContext(ctx, in, opts...)` | you need cancellation or a timeout |
| `ConvertNative(in, opts...)` | let Tesseract write the PDF (images only) |
| `DefaultOutputPath(in)` | you want to know the output name up front |
| `Languages(ctx, opts...)` | list the installed traineddata |

Options and their defaults:

| Option | Default |
|---|---|
| `WithLanguage` | `eng` |
| `WithOutput` | `x.png` → `x.pdf`, `x.pdf` → `x-searchable.pdf` |
| `WithPSM` | `3` |
| `WithDPI` | `0` (read from the image / PDF page size) |
| `WithMinConfidence` | `30` |
| `WithFont` | system font (Arial → Segoe UI → Tahoma → DejaVu) |
| `WithTesseractBinary` | auto-discovered via `PATH` / `TESSERACT_PATH` / Program Files |

For fine-grained control, use a `Converter`:

```go
conv, err := ocr2pdf.New(ocr2pdf.WithLanguage("vie"))

res, err := conv.Convert(ctx, "scan.pdf", "")  // "" = derive the name
data, res, err := conv.Render(ctx, "scan.png") // returns []byte, writes nothing
res, err = conv.ConvertNative(ctx, "scan.png", "")
```

Swapping components (dependency injection) — `Converter` depends only on interfaces:

```go
conv, err := ocr2pdf.New(
	ocr2pdf.WithEngine(myEngine),       // another OCR: PaddleOCR, a cloud API...
	ocr2pdf.WithBuilder(myBuilder),     // a different PDF builder
	ocr2pdf.WithImageLoader(myLoader),  // another image source: S3, a scanner...
	ocr2pdf.WithFontLoader(myFonts),
	ocr2pdf.WithDPI(200),               // force a DPI for every page
)
```

## Architecture

```
ocr2pdf.go          facade: Converter + Option, orchestrates the pipeline
cmd/ocr2pdf/        reference CLI
ocr/                Engine (Tesseract), TSV parser, Finder/Runner
imaging/            Loader: FileLoader (png/jpg), PDFLoader (pdfcpu), AutoLoader
pdf/                Builder: Document (objects/xref), ImageEncoder, fontResource, textLayer
fonts/              Font interface, TTF loader, subsetter
```

```
imaging.Loader ──[]Page{image, DPI}──► ocr.Engine ──[]Line{word, box}──► pdf.Builder ──► []byte PDF
```

| Interface | Responsibility | Default implementation |
|---|---|---|
| `ocr.Engine` | image → `[]ocr.Line` | `ocr.Tesseract` |
| `ocr.Finder` / `ocr.Runner` / `ocr.Parser` | locate the binary / run it / parse TSV | `PathFinder`, `ExecRunner`, `TSVParser` |
| `ocr.LanguageReporter`, `ocr.PDFRenderer` | optional engine capabilities | `Tesseract` |
| `imaging.Loader` | file → `[]imaging.Page` | `FileLoader`, `PDFLoader`, `AutoLoader` |
| `pdf.Builder` | `pdf.Input` → bytes | `pdf.NewBuilder()` |
| `pdf.ImageEncoder` | image → compressed data + dictionary | `pdf.DefaultEncoder` |
| `fonts.Font` / `fonts.Loader` | glyphs, widths, unicode, subsetting | `fonts.TTFLoader` |

Adding an input format = adding an `imaging.Loader`, without touching OCR or PDF.
Adding an OCR engine = implementing `ocr.Engine` and passing it through `WithEngine`.

## How the invisible text layer works

Each page is an image XObject covering the page, plus invisible text operators (`3 Tr`) placed at every word position:

1. Tesseract emits TSV → `ocr.Line` (words grouped by level 4 = line) with per-word pixel boxes.
2. `Converter` converts pixels to points with `scale = 72/DPI`; the page size is `pixels × 72 / DPI`.
3. The font is embedded as **Type0 / CIDFontType2 / Identity-H**; CID = glyph id. Because the font is
   subset, `fonts.Font.Subset` returns a `remap` (old glyph → new glyph) that **must be applied in all three places**:
   `/W`, `ToUnicode`, and the hex string in the content stream. Missing one place means wrong characters when copying.
4. Each word is placed with `Tz` to stretch it to the OCR box width; the whole line shares one font size
   and baseline so the copy order is correct.
5. `FontDescriptor` (`Ascent`, `Descent`, `CapHeight`, `FontBBox`) must be in **1/1000 em** units,
   not font units — getting this wrong makes selection boxes twice as tall.

## Testing & CI

```powershell
go test ./...                    # everything (no tesseract needed)
go test ./ocr -run TestTSVParser -v
go test ./pdf -run TestBuildPageSizeFollowsDPI -v
go vet ./... ; gofmt -l .
golangci-lint run ./...          # configured in .golangci.yml
```

CI (`.github/workflows/ci.yml`) runs build, `go vet`, `go test -race`, a `gofmt` check and
`golangci-lint` on every push to `main` and on every pull request.

The tests use a fake font (`pdf/builder_test.go`) and sample TSV (`ocr/tsv_test.go`), so they run offline.
Manual end-to-end check:

```powershell
.\ocr2pdf.exe -in data\scan200.pdf -lang vie
python -c "import pymupdf; d=pymupdf.open(r'data\scan200-searchable.pdf'); print(len(d), d[0].rect, len(d[0].get_text()))"
```

## Limitations

- The input PDF must be a **scan/image** (every page contains an image). PDFs with only vector/text need
  rasterizing first: `pdftoppm -r 200 in.pdf page`, then feed the `page-*.png` files in.
- Text quality depends on the `traineddata` and on how skewed the scan is; deskew first if the image is noticeably tilted.
- Output pages are sized from the image + DPI; the original PDF MediaBox is not preserved.
