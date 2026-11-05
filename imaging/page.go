// Package imaging nạp ảnh các trang từ file ảnh hoặc từ PDF nhiều trang.
package imaging

import (
	"context"
	"fmt"
	"image"
)

// Encoding cho biết dữ liệu ảnh đã được nén sẵn ở định dạng nào.
type Encoding string

const (
	// EncodingRaw nghĩa là chỉ có pixel thô.
	EncodingRaw Encoding = ""
	// EncodingJPEG nghĩa là dữ liệu đã là JPEG (chèn thẳng vào PDF được).
	EncodingJPEG Encoding = "jpeg"
)

// Page là một trang đầu vào: ảnh, DPI và dữ liệu nén gốc nếu có.
type Page struct {
	Image    image.Image
	DPI      float64
	Encoded  []byte
	Encoding Encoding
}

// Loader nạp toàn bộ trang của một tệp đầu vào (ảnh hoặc PDF).
type Loader interface {
	Load(ctx context.Context, path string) ([]Page, error)
}

// openError thêm ngữ cảnh cho lỗi mở tệp.
func openError(path string, err error) error {
	return fmt.Errorf("mở %s: %w", path, err)
}
