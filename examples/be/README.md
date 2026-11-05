# ocr2pdf HTTP API (example)

A tiny [Gin](https://github.com/gin-gonic/gin) service around the `ocr2pdf` module.
Upload an image or a scanned PDF, get the searchable PDF back in the same request.

## Run

```powershell
cd examples\be
go run ./cmd            # listens on :8080, override with $env:PORT
```

`go.mod` uses `replace github.com/TheronThayer9999/ocr2pdf => ../..`, so it always
builds against the local module instead of a published version.

## Endpoints

| Method | Path | Body | Response |
|---|---|---|---|
| `GET` | `/health` | – | `{"status":"ok"}` |
| `GET` | `/api/languages` | – | `{"languages":["eng","vie"]}` |
| `POST` | `/api/convert` | multipart | `application/pdf` (attachment) |

### `POST /api/convert`

| Field | Required | Default | Meaning |
|---|---|---|---|
| `file` | one of | – | png, jpg, jpeg, gif or pdf, up to 32 MB |
| `url` | one of | – | remote file the server downloads instead of an upload |
| `lang` | no | `auto` | `auto` detects the language, or force `eng`, `vie`, `vie+eng` |
| `psm` | no | `3` | page segmentation mode, 0–13 |
| `dpi` | no | auto | force a DPI |
| `conf` | no | `30` | minimum word confidence, 0–100 |

```powershell
curl.exe -F "file=@..\..\data\scan200.pdf" -F "lang=vie" `
  http://localhost:8080/api/convert -o scan-searchable.pdf

curl.exe -F "url=https://example.com/files/scan.pdf" `
  http://localhost:8080/api/convert -o scan-searchable.pdf
```

Response headers:

```
Content-Disposition: attachment; filename="scan200-searchable.pdf"
X-Pdf-Language: vie
X-Pdf-Pages: 1
X-Pdf-Words: 165
```

Errors are JSON with a meaningful status code:

| Status | When |
|---|---|
| `400` | missing `file`/`url`, or `psm`/`dpi`/`conf` out of range |
| `413` | upload larger than 32 MB |
| `415` | unsupported file type |
| `422` | OCR failed (for example a PDF with no image — rasterize it first, or an unreachable URL) |
| `500` | Tesseract missing, font not found, temp file failure |

```json
{ "error": "unsupported file type \".txt\" (allowed: png, jpg, gif, pdf)" }
```

## How it works

1. `MaxBytesReader` caps the request body at 32 MB.
2. The upload is copied to a temp file with its original extension, because the
   loaders read from paths; the file is removed in a `defer`.
3. `conv.Render(ctx, path)` returns the PDF bytes — **nothing extra is written to disk**.
4. The bytes are streamed back with `Content-Disposition: attachment`.

Each request builds its own `ocr2pdf.Converter`, so concurrent uploads share no
mutable state. The whole conversion is bounded by a 5-minute `context`.

## CORS

The middleware allows any origin and exposes `Content-Disposition`, `X-Pdf-Pages`
and `X-Pdf-Words`, so the Angular example (`examples/fe/ocr`, dev server on `:4200`)
can call it directly and read the filename from the response.

## Production notes

- Put it behind a reverse proxy with its own body limit and timeouts.
- OCR is CPU bound: one Tesseract process per request. Add a worker pool or a queue
  before exposing it to real traffic.
- Consider `POST` + job id + `GET /api/jobs/{id}` if documents take minutes.
