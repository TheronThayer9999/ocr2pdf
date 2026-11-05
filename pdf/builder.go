package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/TheronThayer9999/ocr2pdf/fonts"
)

type builder struct {
	encoder ImageEncoder
}

// NewBuilder tạo Builder mặc định: JPEG chèn thẳng, ảnh khác nén bằng zlib.
func NewBuilder() Builder {
	return &builder{encoder: DefaultEncoder{}}
}

// NewBuilderWithEncoder cho phép thay bộ nén ảnh (ví dụ dùng JPEG cho ảnh lớn).
func NewBuilderWithEncoder(e ImageEncoder) Builder {
	if e == nil {
		e = DefaultEncoder{}
	}
	return &builder{encoder: e}
}

// Build dựng file PDF từ danh sách trang.
func (b *builder) Build(in Input) ([]byte, error) {
	if len(in.Pages) == 0 {
		return nil, errors.New("pdf: không có trang nào để dựng")
	}
	if in.Font == nil {
		return nil, errors.New("pdf: cần font cho lớp text ẩn")
	}

	doc := NewDocument()
	catalog := doc.Reserve()
	pagesObj := doc.Reserve()
	fontRes := newFontResource(doc, in.Font, usedGlyphs(in))

	kids := make([]string, 0, len(in.Pages))
	for i, page := range in.Pages {
		pageNum, err := b.writePage(doc, pagesObj, fontRes, page, i+1)
		if err != nil {
			return nil, err
		}
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNum))
	}

	doc.Set(pagesObj, []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), len(kids))))
	doc.Set(catalog, []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesObj)))

	return doc.Write(catalog), nil
}

// writePage ghi ảnh + lớp text của một trang và trả về số hiệu object trang.
func (b *builder) writePage(doc *Document, pagesObj int, fontRes *fontResource, page Page, pageNr int) (int, error) {
	if page.Image.Data == nil {
		return 0, fmt.Errorf("pdf: trang %d thiếu ảnh", pageNr)
	}

	imgData, imgDict, err := b.encoder.Encode(page.Image)
	if err != nil {
		return 0, fmt.Errorf("pdf: nén ảnh trang %d: %w", pageNr, err)
	}

	bounds := page.Image.Data.Bounds()
	dpi := page.Image.DPI
	if dpi <= 0 {
		dpi = 96
	}
	pageW := float64(bounds.Dx()) * 72 / dpi
	pageH := float64(bounds.Dy()) * 72 / dpi

	imgObj := doc.AddStream(fmt.Sprintf(
		"/Type /XObject /Subtype /Image /Width %d /Height %d %s",
		bounds.Dx(), bounds.Dy(), imgDict), imgData)

	var content bytes.Buffer
	fmt.Fprintf(&content, "q\n%s 0 0 %s 0 0 cm\n/Im0 Do\nQ\n", num(pageW), num(pageH))
	content.Write(newTextLayer(fontRes, pageH).Render(page.Lines))
	contentObj := doc.AddStream("", content.Bytes())

	pageNum := doc.Reserve()
	doc.Set(pageNum, []byte(fmt.Sprintf(
		"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources << /XObject << /Im0 %d 0 R >> /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>",
		pagesObj, num(pageW), num(pageH), imgObj, fontRes.num, contentObj)))
	return pageNum, nil
}

// usedGlyphs gom toàn bộ glyph cần dùng trong tất cả các trang.
func usedGlyphs(in Input) map[fonts.Glyph]bool {
	used := make(map[fonts.Glyph]bool)
	for _, page := range in.Pages {
		for _, ln := range page.Lines {
			for _, w := range ln.Words {
				for _, g := range in.Font.Glyphs(w.Text) {
					used[g] = true
				}
			}
		}
	}
	return used
}
