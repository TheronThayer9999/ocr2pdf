package imaging

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// AutoLoader chọn loader theo định dạng tệp (ảnh hoặc PDF).
type AutoLoader struct {
	Image *FileLoader
	PDF   *PDFLoader
}

// NewAutoLoader tạo loader tự nhận dạng đầu vào.
func NewAutoLoader() *AutoLoader {
	return &AutoLoader{Image: NewFileLoader(), PDF: NewPDFLoader()}
}

// Load điều phối sang loader phù hợp.
func (l *AutoLoader) Load(ctx context.Context, path string) ([]Page, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return l.PDF.Load(ctx, path)
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".tif", ".tiff":
		return l.Image.Load(ctx, path)
	}

	// Không rõ phần mở rộng: đoán theo nội dung.
	if isPDF(path) {
		return l.PDF.Load(ctx, path)
	}
	return l.Image.Load(ctx, path)
}

func isPDF(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	head := make([]byte, 5)
	if _, err := f.Read(head); err != nil {
		return false
	}
	return string(head) == "%PDF-"
}
