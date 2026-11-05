// Package pdf dựng file PDF nhiều trang: ảnh gốc + lớp text ẩn để bôi đen/copy.
package pdf

import (
	"image"
	"io"
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

	// Width/Height là kích thước pixel của ảnh. Bắt buộc khi Data nil (ảnh JPEG
	// nhúng thẳng chỉ cần header); 0 thì builder lấy từ Data.Bounds().
	Width, Height int

	// Open nạp nội dung ảnh theo yêu cầu. Khi khác nil, builder gọi Open() thay
	// cho Data/Encoded — nhờ đó dựng được tài liệu 300 trang mà RAM chỉ giữ một
	// ảnh tại một thời điểm. Image trả về phải có Data hoặc Encoded (+ Width/
	// Height) và không được đặt Open.
	Open func() (Image, error)
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

// StreamingBuilder là khả năng tuỳ chọn: dựng thẳng ra io.Writer thay vì giữ
// toàn bộ tệp trong RAM. Dùng cho tài liệu dài — ảnh trang được nạp qua
// Image.Open lần lượt và giải phóng ngay sau khi ghi xong object của trang.
type StreamingBuilder interface {
	BuildTo(w io.Writer, in Input) error
}
