# ocr2pdf

Chuyển **ảnh** hoặc **PDF scan** (ví dụ PDF in từ Word rồi scan ~200 dpi) thành **PDF có lớp text ẩn**:
nhìn y như bản gốc nhưng bôi đen, copy, tìm kiếm được như văn bản thật.

```bash
ocr2pdf.exe -in data\scan200.pdf -lang vie     # -> data\scan200-searchable.pdf
```

Ảnh A4 scan 200 dpi (1654×2339) → PDF 1 trang A4 (595×841 pt), ~278 KB, copy ra đúng tiếng Việt có dấu.

## Tính năng

- OCR bằng Tesseract qua CLI — **không cần CGO**, không phụ thuộc thư viện native.
- Đầu vào **ảnh** (png/jpg/gif) hoặc **PDF nhiều trang**; đầu ra PDF nhiều trang tương ứng.
- Ảnh JPEG trong PDF scan được chèn thẳng (`DCTDecode`): không nén lại, không mất chất lượng.
- Ảnh khác được nén zlib + filter kiểu PNG (`Predictor 15`) nên file nhỏ.
- Font nhúng **rút gọn** còn đúng glyph đã dùng (~55 KB thay vì cả Arial ~1 MB).
- Có `ToUnicode` đầy đủ → copy/tìm kiếm ra đúng ký tự, kể cả tiếng Việt (`KỊCH BẢN`).

## Yêu cầu

- Go 1.26+ (theo `go.mod`).
- [Tesseract OCR 5.x](https://github.com/UB-Mannheim/tesseract/wiki) + `traineddata` của ngôn ngữ cần dùng.

Chương trình tự tìm binary theo thứ tự: cờ `-tesseract` → biến môi trường `TESSERACT_PATH` → `PATH`
→ `%ProgramFiles%\Tesseract-OCR\tesseract.exe` → `%LOCALAPPDATA%\Programs\Tesseract-OCR\tesseract.exe`
→ `/usr/bin/tesseract`, `/usr/local/bin/tesseract`.

```powershell
# kiểm tra nhanh
& "C:\Program Files\Tesseract-OCR\tesseract.exe" --list-langs
```

## Build & chạy CLI

```powershell
go build -o ocr2pdf.exe ./cmd/ocr2pdf

.\ocr2pdf.exe -in data\image_one.png -out data\image_one.pdf -lang vie
.\ocr2pdf.exe -in data\scan200.pdf -lang vie          # tự đặt tên ...-searchable.pdf
```

| Cờ | Mặc định | Ý nghĩa |
|---|---|---|
| `-in` | – | ảnh (png/jpg/gif) hoặc PDF đầu vào |
| `-out` | tự đặt | `x.png` → `x.pdf`; `x.pdf` → `x-searchable.pdf` (tránh ghi đè đầu vào) |
| `-lang` | `eng` | ngôn ngữ tesseract: `eng`, `vie`, `vie+eng` |
| `-psm` | `3` | page segmentation mode: 3 tự động, 6 một khối text, 7 một dòng, 11 text rời |
| `-dpi` | `0` | ép DPI; 0 = đọc từ metadata ảnh (PNG pHYs / JFIF) hoặc tính từ khổ trang PDF |
| `-conf` | `30` | bỏ qua từ có confidence thấp hơn |
| `-tesseract` | tự tìm | đường dẫn `tesseract.exe` |
| `-font` | font hệ thống | TTF cho lớp text ẩn (mặc định Arial → Segoe UI → Tahoma → DejaVu) |
| `-native` | `false` | để tesseract tự sinh PDF (chỉ dùng cho ảnh, không kiểm soát được layout) |

> `-lang eng` trên tài liệu tiếng Việt sẽ ra ký tự sai dấu (`trur©c`, `Phigu`). Dùng `-lang vie` nếu tài liệu có dấu.

## Dùng như module

```bash
go get github.com/TheronThayer9999/ocr2pdf
```

> Module path trùng với repo: `github.com/TheronThayer9999/ocr2pdf`. Nếu fork sang tài khoản khác,
> đổi dòng `module ...` trong `go.mod` rồi thay import path tương ứng.

Đường ngắn nhất — mọi tham số đều có mặc định:

```go
res, err := ocr2pdf.Convert("scan.pdf") // -> scan-searchable.pdf
```

Đổi tham số khi cần:

```go
res, err := ocr2pdf.Convert("scan.pdf",
	ocr2pdf.WithLanguage("vie"),       // mặc định "eng"
	ocr2pdf.WithOutput("ket-qua.pdf"), // mặc định tự đặt tên
)
```

Chương trình hoàn chỉnh:

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
	fmt.Println(res) // scan-searchable.pdf (277.7 KB) - 1 trang, 23 dòng, 165 từ - 1.05s
}
```

Hàm cấp package — không cần khởi tạo gì:

| Hàm | Dùng khi |
|---|---|
| `Convert(in, opts...)` | đường ngắn nhất, tự đặt tên output |
| `ConvertContext(ctx, in, opts...)` | cần huỷ giữa chừng hoặc timeout |
| `ConvertNative(in, opts...)` | để tesseract tự xuất PDF (chỉ với ảnh) |
| `DefaultOutputPath(in)` | biết trước tên file sẽ ghi |
| `Languages(ctx, opts...)` | liệt kê traineddata đang cài |

Tuỳ chọn (`Option`) và mặc định:

| Option | Mặc định |
|---|---|
| `WithLanguage` | `eng` |
| `WithOutput` | `x.png` → `x.pdf`, `x.pdf` → `x-searchable.pdf` |
| `WithPSM` | `3` |
| `WithDPI` | `0` (tự đọc từ ảnh / khổ trang PDF) |
| `WithMinConfidence` | `30` |
| `WithFont` | font hệ thống (Arial → Segoe UI → Tahoma → DejaVu) |
| `WithTesseractBinary` | tự dò `PATH` / `TESSERACT_PATH` / Program Files |

Khi cần kiểm soát chi tiết thì dùng `Converter`:

```go
conv, err := ocr2pdf.New(ocr2pdf.WithLanguage("vie"))

res, err := conv.Convert(ctx, "scan.pdf", "")  // "" = tự đặt tên
data, res, err := conv.Render(ctx, "scan.png") // trả []byte, không ghi file
res, err = conv.ConvertNative(ctx, "scan.png", "")
```

Thay thành phần (dependency injection) — `Converter` chỉ phụ thuộc interface:

```go
conv, err := ocr2pdf.New(
	ocr2pdf.WithEngine(myEngine),       // OCR khác: PaddleOCR, cloud API...
	ocr2pdf.WithBuilder(myBuilder),     // cách dựng PDF khác
	ocr2pdf.WithImageLoader(myLoader),  // nguồn ảnh khác: S3, scanner...
	ocr2pdf.WithFontLoader(myFonts),
	ocr2pdf.WithDPI(200),               // ép DPI cho mọi trang
)
```

## Kiến trúc

```
ocr2pdf.go          facade: Converter + Option, điều phối pipeline
cmd/ocr2pdf/        CLI mẫu
ocr/                Engine (Tesseract), TSV parser, Finder/Runner
imaging/            Loader: FileLoader (png/jpg), PDFLoader (pdfcpu), AutoLoader
pdf/                Builder: Document (object/xref), ImageEncoder, fontResource, textLayer
fonts/              Font interface, TTF loader, subsetter
```

```
imaging.Loader ──[]Page{ảnh, DPI}──► ocr.Engine ──[]Line{từ, hộp}──► pdf.Builder ──► []byte PDF
```

| Interface | Trách nhiệm | Cài đặt mặc định |
|---|---|---|
| `ocr.Engine` | ảnh → `[]ocr.Line` | `ocr.Tesseract` |
| `ocr.Finder` / `ocr.Runner` / `ocr.Parser` | tìm binary / chạy lệnh / đọc TSV | `PathFinder`, `ExecRunner`, `TSVParser` |
| `ocr.LanguageReporter`, `ocr.PDFRenderer` | khả năng tuỳ chọn của engine | `Tesseract` |
| `imaging.Loader` | tệp → `[]imaging.Page` | `FileLoader`, `PDFLoader`, `AutoLoader` |
| `pdf.Builder` | `pdf.Input` → bytes | `pdf.NewBuilder()` |
| `pdf.ImageEncoder` | ảnh → dữ liệu nén + dictionary | `pdf.DefaultEncoder` |
| `fonts.Font` / `fonts.Loader` | glyph, độ rộng, unicode, subset | `fonts.TTFLoader` |

Thêm định dạng đầu vào = thêm một `imaging.Loader`, không sửa OCR/PDF.
Thêm engine OCR = implement `ocr.Engine`, truyền qua `WithEngine`.

## Lớp text ẩn hoạt động thế nào

Mỗi trang là ảnh XObject phủ kín trang, cộng các toán tử text vô hình (`3 Tr`) đặt đúng vị trí từng từ:

1. Tesseract xuất TSV → `ocr.Line` (gộp từ theo level 4 = dòng) + hộp từ theo pixel.
2. `Converter` đổi pixel → point bằng `scale = 72/DPI`, kích thước trang = `pixel × 72 / DPI`.
3. Font nhúng dạng **Type0 / CIDFontType2 / Identity-H**; CID = glyph id. Vì font bị subset
   nên `fonts.Font.Subset` trả về `remap` (glyph cũ → glyph mới) và **phải áp dụng remap ở cả ba chỗ**:
   `/W`, `ToUnicode`, và chuỗi hex trong content stream. Lệch một chỗ là copy ra sai ký tự.
4. Mỗi từ đặt bằng `Tz` để kéo giãn ngang vừa đúng bề rộng hộp nhận dạng; cả dòng dùng chung
   cỡ chữ + baseline để thứ tự copy đúng.
5. `FontDescriptor` (`Ascent`, `Descent`, `CapHeight`, `FontBBox`) phải theo đơn vị **1/1000 em**,
   không phải font unit — sai chỗ này làm hộp chọn text cao gấp đôi.

## Kiểm thử & CI

```powershell
go test ./...                    # toàn bộ (không cần tesseract)
go test ./ocr -run TestTSVParser -v
go test ./pdf -run TestBuildPageSizeFollowsDPI -v
go vet ./... ; gofmt -l .
golangci-lint run ./...          # cấu hình trong .golangci.yml
```

CI (`.github/workflows/ci.yml`) chạy build, `go vet`, `go test -race`, kiểm tra `gofmt`
và `golangci-lint` trên mỗi push vào `main` và mỗi pull request.

Test dùng font giả (`pdf/builder_test.go`) và TSV mẫu (`ocr/tsv_test.go`) nên chạy offline.
Kiểm tra end-to-end thủ công:

```powershell
.\ocr2pdf.exe -in data\scan200.pdf -lang vie
python -c "import pymupdf; d=pymupdf.open(r'data\scan200-searchable.pdf'); print(len(d), d[0].rect, len(d[0].get_text()))"
```

## Giới hạn

- PDF đầu vào phải là **bản scan/ảnh** (mỗi trang có ảnh). PDF chỉ có vector/text cần rasterize trước:
  `pdftoppm -r 200 in.pdf page` rồi đưa các file `page-*.png` vào.
- Chất lượng text phụ thuộc `traineddata` và độ nghiêng của bản scan; nên deskew trước nếu ảnh lệch nhiều.
- Trang đầu ra có kích thước theo ảnh + DPI, không giữ nguyên MediaBox gốc của PDF đầu vào.
