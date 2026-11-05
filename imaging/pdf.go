package imaging

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFLoader trích ảnh từng trang của PDF (bản scan/in từ Word).
type PDFLoader struct {
	DefaultDPI float64 // dùng khi không tính được DPI từ khổ trang
}

// NewPDFLoader tạo loader cho đầu vào PDF.
func NewPDFLoader() *PDFLoader {
	return &PDFLoader{DefaultDPI: 200}
}

// Load trả về mỗi trang PDF thành một Page (ảnh lớn nhất trên trang).
func (l *PDFLoader) Load(ctx context.Context, path string) ([]Page, error) {
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	f, err := os.Open(path)
	if err != nil {
		return nil, openError(path, err)
	}
	defer f.Close()

	dims, err := api.PageDims(ctx, f, conf)
	if err != nil {
		return nil, fmt.Errorf("đọc khổ trang PDF %s: %w", path, err)
	}
	if len(dims) == 0 {
		return nil, fmt.Errorf("PDF %s không có trang nào", path)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	extracted, err := api.ExtractImagesRaw(ctx, f, nil, conf)
	if err != nil && len(extracted) == 0 {
		return nil, fmt.Errorf("trích ảnh từ PDF %s: %w", path, err)
	}

	pages := make([]Page, 0, len(dims))
	for i, dim := range dims {
		var images map[int]model.Image
		if i < len(extracted) {
			images = extracted[i]
		}
		page, err := l.page(i+1, dim, images)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// page chọn ảnh lớn nhất trên một trang và tính DPI từ khổ trang thật.
func (l *PDFLoader) page(pageNr int, dim types.Dim, images map[int]model.Image) (Page, error) {
	var best *Page

	for _, img := range images {
		if img.IsImgMask || img.Thumb {
			continue
		}
		data, err := io.ReadAll(img.Reader)
		if err != nil || len(data) == 0 {
			continue
		}
		decoded, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			continue // định dạng chưa hỗ trợ (CCITT/JBIG2...)
		}

		bounds := decoded.Bounds()
		if best != nil {
			bestBounds := best.Image.Bounds()
			if bounds.Dx()*bounds.Dy() <= bestBounds.Dx()*bestBounds.Dy() {
				continue
			}
		}

		dpi := l.DefaultDPI
		if dim.Width > 0 {
			dpi = float64(bounds.Dx()) / (dim.Width / 72)
		}
		if dpi <= 0 {
			dpi = 200
		}

		page := Page{Image: decoded, DPI: dpi}
		if img.FileType == "jpg" || img.FileType == "jpeg" {
			page.Encoded, page.Encoding = data, EncodingJPEG
		}
		best = &page
	}

	if best == nil {
		return Page{}, fmt.Errorf("trang %d của PDF không có ảnh: PDF này không phải bản scan (cần rasterize trước)", pageNr)
	}
	return *best, nil
}
