# AGENTS.md

## Commands

- Build CLI: `go build -o ocr2pdf.exe ./cmd/ocr2pdf`
- Run: `.\ocr2pdf.exe -in data\scan200.pdf -lang vie` (tự đặt tên `...-searchable.pdf`)
- Tests (không cần Tesseract): `go test ./...`
- Một test: `go test ./ocr -run TestTSVParser -v` · `go test ./pdf -run TestBuild -v`
- Trước khi xong: `go vet ./...`, `gofmt -l .` (phải rỗng), `golangci-lint run ./...`

## Môi trường (máy này)

- Tesseract **không có trong PATH**; binary ở `C:\Program Files\Tesseract-OCR\tesseract.exe`
  (v5.4, langs: `eng`, `vie`, `osd`). Code tự dò; override bằng `TESSERACT_PATH` hoặc `-tesseract`.
- **Đừng sửa file nguồn bằng PowerShell** (`Get-Content | Set-Content`): PS 5.1 ghi lại encoding và
  biến tiếng Việt trong comment/string thành mojibake. Dùng tool edit/write.
- Có sẵn để kiểm chứng: Python + `pymupdf` (đọc text/rect ảnh của PDF), `PIL`; `pdfcpu` (Go) để validate.
  Không có poppler/pdftoppm.
- `go.mod` ghim `go 1.26.0` vì `pdfcpu` và `x/image` yêu cầu 1.26; đừng nâng lên 1.27.
  golangci-lint 2.14 build bằng go1.26 nên trên máy Go 1.27 nó panic
  `file requires newer Go version go1.27`; chạy bằng `$env:GOTOOLCHAIN="go1.26.5"; golangci-lint run ./...`.
- Input mẫu: `data/image_one.png`, `data/scan200.pdf` (A4 200 dpi).

## Kiến trúc — giữ đúng ranh giới

- Pipeline: `imaging.Loader` → `ocr.Engine` → `pdf.Builder`; `Converter` (package gốc) **chỉ** phụ thuộc interface.
- Các package public có chủ đích (module để người khác import): `ocr/`, `imaging/`, `pdf/`, `fonts/`. Không dùng `internal/`.
- Thêm định dạng đầu vào = thêm `imaging.Loader`; thêm OCR engine = implement `ocr.Engine` (truyền qua `WithEngine`).
- Module path `github.com/TheronThayer9999/ocr2pdf` trùng với repo GitHub; fork sang tài khoản khác thì đổi `module` trong `go.mod` + rewrite import.

## Bất biến của lớp PDF (dễ làm hỏng)

- Font nhúng là Type0 / CIDFontType2 / Identity-H, **CID = glyph id**. `fonts.Font.Subset` trả `remap`
  (glyph cũ → mới) và phải áp dụng ở **cả 3 chỗ**: mảng `/W`, CMap `/ToUnicode`, và chuỗi hex trong content stream.
- `FontDescriptor` (`Ascent`, `Descent`, `CapHeight`, `FontBBox`) tính theo **1/1000 em**, không phải font unit.
- Kích thước trang = `pixel × 72 / DPI`. DPI lấy từ PNG `pHYs` / JPEG JFIF, không có thì 200;
  với PDF thì `imageWidth / (pageWidthPoint / 72)`.
- Text ẩn dùng `3 Tr`; mỗi dòng chung cỡ chữ + baseline, mỗi từ dùng `Tz` để khớp bề rộng hộp OCR.
- Ảnh JPEG phải chèn thẳng (`/Filter /DCTDecode`), không giải mã rồi nén lại; ảnh khác đi đường
  PNG-predictor (`/Predictor 15`) trong `pdf/image.go`.

## Giới hạn đã biết

- PDF đầu vào phải là bản scan/ảnh (đọc bằng pdfcpu `ExtractImagesRaw`). PDF vector/text trả lỗi yêu cầu rasterize trước.
- `-lang eng` (mặc định) làm sai dấu tiếng Việt (`trur©c`); `vie` đã cài, dùng khi tài liệu có dấu.
- Trang ra có kích thước theo ảnh + DPI, không giữ MediaBox gốc của PDF đầu vào.
