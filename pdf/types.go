// Package pdf dựng file PDF nhiều trang: ảnh gốc + lớp text ẩn để bôi đen/copy.
package pdf

import (
	"image"
	"strings"

	"github.com/TheronThayer9999/ocr2pdf/fonts"
)

// Encoding cho biết dữ liệu ảnh đã được nén sẵn theo định dạng nào.
type Encoding string

const (
	// EncodingRaw nghĩa là chỉ có pixel thô, builder tự nén.
	EncodingRaw Encoding = ""
	// EncodingJPEG nghĩa là dữ liệu đã là JPEG, chèn thẳng bằng DCTDecode.
	EncodingJPEG Encoding = "jpeg"
)

// Image là ảnh của một trang kèm DPI để quy đổi pixel -> point.
type Image struct {
	Data     image.Image
	DPI      float64
	Encoded  []byte   // dữ liệu nén sẵn (nếu có)
	Encoding Encoding // định dạng của Encoded
}

// Word là một từ cần đặt lên lớp text ẩn (toạ độ point, gốc trên-trái).
type Word struct {
	Text  string
	X     float64 // mép trái
	Width float64
}

// Line là một dòng text: mọi từ dùng chung baseline và cỡ chữ.
type Line struct {
	Top    float64 // mép trên so với đỉnh trang
	Height float64
	Words  []Word
}

// Text ghép các từ của dòng thành chuỗi.
func (l Line) Text() string {
	parts := make([]string, 0, len(l.Words))
	for _, w := range l.Words {
		parts = append(parts, w.Text)
	}
	return strings.Join(parts, " ")
}

// Page là một trang đầu ra: ảnh nền và các dòng text nhận dạng được.
type Page struct {
	Image Image
	Lines []Line
}

// Input là toàn bộ dữ liệu để dựng file PDF.
type Input struct {
	Pages []Page
	Font  fonts.Font
}

// Builder dựng nội dung file PDF từ Input.
type Builder interface {
	Build(in Input) ([]byte, error)
}
