// Package ocr2pdf chuyển ảnh hoặc PDF scan (ảnh) thành PDF có lớp text ẩn,
// cho phép bôi đen và copy nội dung như văn bản thật.
//
// Kiến trúc: Converter điều phối 3 thành phần có thể thay thế —
// imaging.Loader (đọc trang đầu vào), ocr.Engine (nhận dạng chữ) và pdf.Builder (dựng PDF).
//
//	c, err := ocr2pdf.New(ocr2pdf.WithLanguage("vie"))
//	if err != nil { ... }
//	res, err := c.Convert(context.Background(), "scan.pdf", "scan-searchable.pdf")
package ocr2pdf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheronThayer9999/ocr2pdf/fonts"
	"github.com/TheronThayer9999/ocr2pdf/imaging"
	"github.com/TheronThayer9999/ocr2pdf/ocr"
	"github.com/TheronThayer9999/ocr2pdf/pdf"
)

// ErrNativeUnsupported trả về khi engine không hỗ trợ tự xuất PDF.
var ErrNativeUnsupported = errors.New("engine hiện tại không hỗ trợ xuất PDF trực tiếp")

// Result mô tả kết quả một lần chuyển đổi.
type Result struct {
	Output   string
	Pages    int
	Lines    int
	Words    int
	Bytes    int
	Duration time.Duration
}

// String tóm tắt kết quả trong một dòng, tiện cho CLI và log.
func (r Result) String() string {
	return fmt.Sprintf("%s (%.1f KB) - %d trang, %d dòng, %d từ - %s",
		r.Output, float64(r.Bytes)/1024, r.Pages, r.Lines, r.Words,
		r.Duration.Round(time.Millisecond))
}

// Converter điều phối toàn bộ pipeline chuyển đổi.
type Converter struct {
	images   imaging.Loader
	engine   ocr.Engine
	builder  pdf.Builder
	fonts    fonts.Loader
	fontPath string
	output   string
	dpi      float64

	// cấu hình dùng khi tự tạo engine Tesseract
	tesseractBinary string
	language        string
	psm             int
	minConfidence   float64
}

// Option cấu hình Converter.
type Option func(*Converter)

// WithImageLoader thay bộ đọc đầu vào (mặc định tự nhận dạng ảnh/PDF).
func WithImageLoader(l imaging.Loader) Option {
	return func(c *Converter) { c.images = l }
}

// WithEngine thay engine OCR (mặc định là Tesseract).
func WithEngine(e ocr.Engine) Option {
	return func(c *Converter) { c.engine = e }
}

// WithBuilder thay bộ dựng PDF.
func WithBuilder(b pdf.Builder) Option {
	return func(c *Converter) { c.builder = b }
}

// WithFontLoader thay bộ nạp font.
func WithFontLoader(l fonts.Loader) Option {
	return func(c *Converter) { c.fonts = l }
}

// WithFont chỉ định file TTF cho lớp text ẩn (mặc định lấy font hệ thống).
func WithFont(path string) Option {
	return func(c *Converter) { c.fontPath = path }
}

// WithOutput đặt file PDF đầu ra; bỏ trống thì suy ra từ tên file vào
// (x.png -> x.pdf, x.pdf -> x-searchable.pdf).
func WithOutput(path string) Option {
	return func(c *Converter) { c.output = path }
}

// WithDPI ép DPI cho mọi trang (0 = dùng DPI đọc được từ tệp).
func WithDPI(dpi float64) Option {
	return func(c *Converter) { c.dpi = dpi }
}

// WithTesseractBinary chỉ định đường dẫn tesseract.
func WithTesseractBinary(path string) Option {
	return func(c *Converter) { c.tesseractBinary = path }
}

// WithLanguage đặt ngôn ngữ OCR, ví dụ "eng", "vie", "vie+eng".
func WithLanguage(lang string) Option {
	return func(c *Converter) { c.language = lang }
}

// WithPSM đặt page segmentation mode của tesseract (0..13).
func WithPSM(psm int) Option {
	return func(c *Converter) { c.psm = psm }
}

// WithMinConfidence bỏ qua những từ có độ tin cậy thấp hơn ngưỡng.
func WithMinConfidence(v float64) Option {
	return func(c *Converter) { c.minConfidence = v }
}

// New tạo Converter với các thành phần mặc định, cho phép ghi đè qua Option.
func New(opts ...Option) (*Converter, error) {
	c := &Converter{
		language: "eng",
		psm:      3,
	}
	for _, opt := range opts {
		opt(c)
	}

	if c.images == nil {
		c.images = imaging.NewAutoLoader()
	}
	if c.builder == nil {
		c.builder = pdf.NewBuilder()
	}
	if c.fonts == nil {
		c.fonts = fonts.NewTTFLoader()
	}
	if c.engine == nil {
		engine, err := ocr.NewTesseract(
			ocr.NewPathFinder(c.tesseractBinary),
			ocr.ExecRunner{},
			ocr.NewTSVParser(c.minConfidence),
			ocr.WithLanguage(c.language),
			ocr.WithPSM(c.psm),
		)
		if err != nil {
			return nil, err
		}
		c.engine = engine
	}
	return c, nil
}

// Convert đọc inputPath (ảnh hoặc PDF) và ghi PDF kết quả ra outputPath.
// Nếu outputPath rỗng, dùng WithOutput hoặc tự suy ra tên từ inputPath.
func (c *Converter) Convert(ctx context.Context, inputPath, outputPath string) (Result, error) {
	if outputPath == "" {
		outputPath = c.output
	}
	if outputPath == "" {
		outputPath = DefaultOutputPath(inputPath)
	}

	data, res, err := c.Render(ctx, inputPath)
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return Result{}, fmt.Errorf("ghi %s: %w", outputPath, err)
	}
	res.Output = outputPath
	return res, nil
}

// Render trả về nội dung PDF trong bộ nhớ (dùng khi không muốn ghi ra file).
func (c *Converter) Render(ctx context.Context, inputPath string) ([]byte, Result, error) {
	start := time.Now()

	pages, err := c.images.Load(ctx, inputPath)
	if err != nil {
		return nil, Result{}, err
	}

	font, err := c.fonts.Load(c.fontPath)
	if err != nil {
		return nil, Result{}, err
	}

	out := pdf.Input{Font: font, Pages: make([]pdf.Page, 0, len(pages))}
	res := Result{Pages: len(pages)}

	for _, page := range pages {
		lines, err := c.engine.Recognize(ctx, page.Image)
		if err != nil {
			return nil, Result{}, fmt.Errorf("OCR trang %d: %w", len(out.Pages)+1, err)
		}
		dpi := page.DPI
		if c.dpi > 0 {
			dpi = c.dpi
		}
		if dpi <= 0 {
			dpi = 200
		}
		out.Pages = append(out.Pages, pdf.Page{
			Image: pdf.Image{
				Data:     page.Image,
				DPI:      dpi,
				Encoded:  page.Encoded,
				Encoding: pdf.Encoding(page.Encoding),
			},
			Lines: toPDFLines(lines, 72/dpi),
		})
		res.Lines += len(lines)
		for _, ln := range lines {
			res.Words += ln.Count()
		}
	}

	data, err := c.builder.Build(out)
	if err != nil {
		return nil, Result{}, err
	}
	res.Bytes = len(data)
	res.Duration = time.Since(start)
	return data, res, nil
}

// ConvertNative để engine tự xuất PDF (nhanh, không cần font, chỉ hỗ trợ ảnh).
func (c *Converter) ConvertNative(ctx context.Context, imagePath, outputPath string) (Result, error) {
	renderer, ok := c.engine.(ocr.PDFRenderer)
	if !ok {
		return Result{}, ErrNativeUnsupported
	}
	if outputPath == "" {
		outputPath = c.output
	}
	if outputPath == "" {
		outputPath = DefaultOutputPath(imagePath)
	}
	start := time.Now()
	if err := renderer.RenderPDF(ctx, imagePath, outputPath); err != nil {
		return Result{}, err
	}
	res := Result{Output: outputPath, Pages: 1, Duration: time.Since(start)}
	if info, err := os.Stat(outputPath); err == nil {
		res.Bytes = int(info.Size())
	}
	return res, nil
}

// Languages trả về các traineddata đang cài (rỗng nếu engine không hỗ trợ).
func (c *Converter) Languages(ctx context.Context) []string {
	reporter, ok := c.engine.(ocr.LanguageReporter)
	if !ok {
		return nil
	}
	langs, err := reporter.Languages(ctx)
	if err != nil {
		return nil
	}
	return langs
}

// Convert là cách dùng ngắn nhất — mọi tham số đều có mặc định:
//
//	res, err := ocr2pdf.Convert("scan.pdf")                                    // -> scan-searchable.pdf
//	res, err := ocr2pdf.Convert("scan.pdf", ocr2pdf.WithLanguage("vie"))
//	res, err := ocr2pdf.Convert("scan.png", ocr2pdf.WithOutput("out.pdf"))
func Convert(inputPath string, opts ...Option) (Result, error) {
	return ConvertContext(context.Background(), inputPath, opts...)
}

// ConvertContext như Convert nhưng nhận context để huỷ giữa chừng.
func ConvertContext(ctx context.Context, inputPath string, opts ...Option) (Result, error) {
	conv, err := New(opts...)
	if err != nil {
		return Result{}, err
	}
	return conv.Convert(ctx, inputPath, "")
}

// ConvertNative để engine tự xuất PDF: nhanh hơn, không nhúng font riêng.
// Hiện chỉ Tesseract hỗ trợ và chỉ nhận đầu vào là ảnh.
func ConvertNative(inputPath string, opts ...Option) (Result, error) {
	conv, err := New(opts...)
	if err != nil {
		return Result{}, err
	}
	return conv.ConvertNative(context.Background(), inputPath, "")
}

// Languages trả về các traineddata đang cài, dùng để kiểm tra trước khi chạy.
func Languages(ctx context.Context, opts ...Option) []string {
	conv, err := New(opts...)
	if err != nil {
		return nil
	}
	return conv.Languages(ctx)
}

// DefaultOutputPath đặt tên file kết quả, tránh ghi đè lên PDF đầu vào:
// x.png -> x.pdf, x.pdf -> x-searchable.pdf.
func DefaultOutputPath(inputPath string) string {
	ext := strings.ToLower(filepath.Ext(inputPath))
	base := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
	if ext == ".pdf" {
		return base + "-searchable.pdf"
	}
	return base + ".pdf"
}

// toPDFLines chuyển toạ độ pixel của OCR sang point (scale = 72/dpi), giữ nguyên thứ tự đọc.
func toPDFLines(lines []ocr.Line, scale float64) []pdf.Line {
	out := make([]pdf.Line, 0, len(lines))
	for _, ln := range lines {
		pl := pdf.Line{Top: float64(ln.Top) * scale, Height: float64(ln.Height) * scale}
		for _, w := range ln.Words {
			pl.Words = append(pl.Words, pdf.Word{
				Text:  w.Text,
				X:     float64(w.Left) * scale,
				Width: float64(w.Width) * scale,
			})
		}
		out = append(out, pl)
	}
	return out
}
