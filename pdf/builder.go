package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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

// Build dựng file PDF trong bộ nhớ. Tài liệu dài nên dùng BuildTo để RAM chỉ
// giữ một ảnh trang tại một thời điểm.
func (b *builder) Build(in Input) ([]byte, error) {
	var out bytes.Buffer
	if err := b.BuildTo(&out, in); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// BuildTo dựng PDF thẳng ra w: mỗi ảnh trang chỉ được nạp (qua Image.Open) đúng
// lúc ghi object của trang đó rồi giải phóng, nên bộ nhớ không tăng theo số
// trang. Font vẫn phải nằm trong bộ nhớ vì mọi trang dùng chung.
func (b *builder) BuildTo(w io.Writer, in Input) error {
	if len(in.Pages) == 0 {
		return errors.New("pdf: không có trang nào để dựng")
	}
	if in.Font == nil {
		return errors.New("pdf: cần font cho lớp text ẩn")
	}

	doc := NewDocument()
	catalog := doc.Reserve()
	pagesObj := doc.Reserve()
	fontRes := newFontResource(doc, in.Font, usedGlyphs(in))

	firstPageObj := len(doc.objects) + 1
	doc.Set(pagesObj, []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(pageRefs(firstPageObj, len(in.Pages)), " "), len(in.Pages))))
	doc.Set(catalog, []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesObj)))

	pw := &pdfWriter{w: w}
	if err := pw.header(); err != nil {
		return err
	}
	for i, body := range doc.objects {
		if err := pw.object(i+1, body); err != nil {
			return err
		}
	}
	for i, page := range in.Pages {
		if err := b.writePage(pw, fontRes, page, pagesObj, firstPageObj+3*i, i+1); err != nil {
			return err
		}
	}
	return pw.finish(catalog)
}

// pageRefs trả danh sách tham chiếu object của từng trang. Mỗi trang chiếm 3
// object liên tiếp (ảnh, content, page), nên số object trang tính được trước
// khi ghi bất cứ thứ gì.
func pageRefs(firstPageObj, pages int) []string {
	kids := make([]string, pages)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", firstPageObj+3*i+2)
	}
	return kids
}

// renderedPage là phần đã nén/xếp chỗ của một trang, đủ để ghi ra cả đường
// in-memory lẫn streaming.
type renderedPage struct {
	imgData []byte
	imgDict string
	content []byte
	width   float64
	height  float64
}

// renderPage nạp ảnh (nếu có Image.Open), nén và dựng lớp text của một trang.
func (b *builder) renderPage(fontRes *fontResource, page Page, pageNr int) (renderedPage, error) {
	img, err := resolveImage(page.Image)
	if err != nil {
		return renderedPage{}, fmt.Errorf("pdf: nạp ảnh trang %d: %w", pageNr, err)
	}
	imgData, imgDict, err := b.encoder.Encode(img)
	if err != nil {
		return renderedPage{}, fmt.Errorf("pdf: nén ảnh trang %d: %w", pageNr, err)
	}
	wPx, hPx, err := imageSize(img)
	if err != nil {
		return renderedPage{}, fmt.Errorf("pdf: ảnh trang %d: %w", pageNr, err)
	}
	dpi := img.DPI
	if dpi <= 0 {
		dpi = 96
	}
	pageW := float64(wPx) * 72 / dpi
	pageH := float64(hPx) * 72 / dpi

	var content bytes.Buffer
	fmt.Fprintf(&content, "q\n%s 0 0 %s 0 0 cm\n/Im0 Do\nQ\n", num(pageW), num(pageH))
	content.Write(newTextLayer(fontRes, pageH).Render(page.Lines))

	return renderedPage{
		imgData: imgData,
		imgDict: fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d %s", wPx, hPx, imgDict),
		content: content.Bytes(),
		width:   pageW,
		height:  pageH,
	}, nil
}

// writePage ghi 3 object của một trang: ảnh XObject, content stream, page.
func (b *builder) writePage(pw *pdfWriter, fontRes *fontResource, page Page, parentObj, firstObj, pageNr int) error {
	rp, err := b.renderPage(fontRes, page, pageNr)
	if err != nil {
		return err
	}
	if err := pw.stream(firstObj, rp.imgDict, rp.imgData); err != nil {
		return err
	}
	if err := pw.stream(firstObj+1, "", rp.content); err != nil {
		return err
	}
	return pw.object(firstObj+2, pageDict(parentObj, rp.width, rp.height, firstObj, fontRes.num, firstObj+1))
}

// imageSize dùng Width/Height khi caller đã khai (đường JPEG passthrough không
// có Data), còn không thì lấy từ chính ảnh đã giải mã.
func imageSize(img Image) (int, int, error) {
	if img.Width > 0 && img.Height > 0 {
		return img.Width, img.Height, nil
	}
	if img.Data == nil {
		return 0, 0, errors.New("thiếu Data và Width/Height")
	}
	bounds := img.Data.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}

// resolveImage gọi Image.Open (nếu có) và điền các giá trị mặc định từ ảnh gốc.
func resolveImage(img Image) (Image, error) {
	if img.Open == nil {
		return img, nil
	}
	resolved, err := img.Open()
	if err != nil {
		return Image{}, err
	}
	if resolved.DPI <= 0 {
		resolved.DPI = img.DPI
	}
	if resolved.Width <= 0 {
		resolved.Width = img.Width
	}
	if resolved.Height <= 0 {
		resolved.Height = img.Height
	}
	resolved.Open = nil
	return resolved, nil
}

// pageDict dựng dictionary của object trang; parentObj là object /Pages.
func pageDict(parentObj int, pageW, pageH float64, imgObj, fontObj, contentObj int) []byte {
	return []byte(fmt.Sprintf(
		"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources << /XObject << /Im0 %d 0 R >> /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>",
		parentObj, num(pageW), num(pageH), imgObj, fontObj, contentObj))
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
