package imaging

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

// FileLoader nạp một file ảnh (png/jpg/gif) thành một trang.
type FileLoader struct {
	DefaultDPI float64 // dùng khi ảnh không có metadata DPI
}

// NewFileLoader tạo loader với DPI mặc định 200 (phù hợp bản scan).
func NewFileLoader() *FileLoader {
	return &FileLoader{DefaultDPI: 200}
}

// Load đọc ảnh và xác định DPI từ metadata nếu có.
func (l *FileLoader) Load(ctx context.Context, path string) ([]Page, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, openError(path, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("giải mã ảnh %s: %w", path, err)
	}

	dpi := detectDPI(path)
	if dpi <= 0 {
		dpi = l.DefaultDPI
	}
	if dpi <= 0 {
		dpi = 200
	}
	return []Page{{Image: img, DPI: dpi}}, nil
}
