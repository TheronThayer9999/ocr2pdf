// Package ocr định nghĩa domain của bước nhận dạng chữ và engine Tesseract.
package ocr

import (
	"context"
	"image"
)

// Word là một từ đã nhận dạng, kèm hộp bao theo pixel (gốc trên-trái).
type Word struct {
	Text  string
	Left  int
	Width int
	Conf  float64
}

// Line là một dòng text; các từ trong dòng dùng chung baseline và cỡ chữ.
type Line struct {
	Text   string
	Top    int
	Height int
	Words  []Word
}

// Count trả về số từ của dòng.
func (l Line) Count() int { return len(l.Words) }

// Engine nhận dạng chữ trong một ảnh.
type Engine interface {
	Recognize(ctx context.Context, img image.Image) ([]Line, error)
}

// LanguageReporter cho biết những traineddata đang có.
type LanguageReporter interface {
	Languages(ctx context.Context) ([]string, error)
}

// PDFRenderer là khả năng tuỳ chọn: engine tự xuất PDF thay vì dùng pdf.Builder.
type PDFRenderer interface {
	RenderPDF(ctx context.Context, imagePath, outputPath string) error
}
