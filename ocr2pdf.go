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
	"image"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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
	Language string // ngôn ngữ OCR thực tế đã dùng (rỗng nếu engine tự quyết)
	Pages    int
	Lines    int
	Words    int
	Bytes    int
	Duration time.Duration
}

// String summarizes the result in one line, handy for CLIs and logs.
func (r Result) String() string {
	parts := []string{
		fmt.Sprintf("%s (%.1f KB)", r.Output, float64(r.Bytes)/1024),
		fmt.Sprintf("%s, %s, %s",
			count(r.Pages, "page"), count(r.Lines, "line"), count(r.Words, "word")),
	}
	if r.Language != "" {
		parts = append(parts, r.Language)
	}
	parts = append(parts, r.Duration.Round(time.Millisecond).String())
	return strings.Join(parts, " - ")
}

// count formats n with a singular or plural English noun.
func count(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
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

	// tạo engine theo ngôn ngữ; nil khi người dùng tự truyền engine
	newEngine func(lang string) (ocr.Engine, error)
}

// autoLanguage bật chế độ tự dò ngôn ngữ từ nội dung trang đầu.
const autoLanguage = "auto"

// maxLanguageCandidates giới hạn số lần OCR thử khi dò ngôn ngữ.
const maxLanguageCandidates = 4

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

// WithLanguage đặt ngôn ngữ OCR: "eng", "vie", "vie+eng"...
// Mặc định là "auto": chạy thử trang đầu bằng từng traineddata đã cài rồi
// chọn ngôn ngữ có confidence cao nhất (tốn thêm một lượt OCR cho trang đầu).
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
		language: autoLanguage,
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
		c.newEngine = c.tesseractEngine
		// Tạo thử một engine để báo lỗi sớm khi thiếu tesseract.
		if _, err := c.newEngine("eng"); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// tesseractEngine tạo engine Tesseract cho một ngôn ngữ cụ thể.
func (c *Converter) tesseractEngine(lang string) (ocr.Engine, error) {
	return ocr.NewTesseract(
		ocr.NewPathFinder(c.tesseractBinary),
		ocr.ExecRunner{},
		ocr.NewTSVParser(c.minConfidence),
		ocr.WithLanguage(lang),
		ocr.WithPSM(c.psm),
	)
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

	engine, language, firstPageLines, err := c.prepareEngine(ctx, pages)
	if err != nil {
		return nil, Result{}, err
	}

	out := pdf.Input{Font: font, Pages: make([]pdf.Page, 0, len(pages))}
	res := Result{Pages: len(pages), Language: language}

	for i, page := range pages {
		lines := firstPageLines
		if i > 0 || lines == nil {
			lines, err = engine.Recognize(ctx, page.Image)
			if err != nil {
				return nil, Result{}, fmt.Errorf("OCR trang %d: %w", i+1, err)
			}
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
// Engine cần file cục bộ nên URL phải được tải về trước.
func (c *Converter) ConvertNative(ctx context.Context, imagePath, outputPath string) (Result, error) {
	if imaging.IsRemote(imagePath) {
		return Result{}, errors.New("ConvertNative cần file cục bộ: hãy tải ảnh về trước, hoặc dùng Convert/Render")
	}
	engine, language, _, err := c.prepareEngine(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	renderer, ok := engine.(ocr.PDFRenderer)
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
	res := Result{Output: outputPath, Language: language, Pages: 1, Duration: time.Since(start)}
	if info, err := os.Stat(outputPath); err == nil {
		res.Bytes = int(info.Size())
	}
	return res, nil
}

// Languages trả về các traineddata đang cài (rỗng nếu engine không hỗ trợ).
func (c *Converter) Languages(ctx context.Context) []string {
	engine := c.engine
	if engine == nil {
		probe, err := c.newEngine("eng")
		if err != nil {
			return nil
		}
		engine = probe
	}
	reporter, ok := engine.(ocr.LanguageReporter)
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
// Với URL, tên file lấy từ đoạn cuối đường dẫn và ghi vào thư mục hiện tại.
func DefaultOutputPath(inputPath string) string {
	if imaging.IsRemote(inputPath) {
		return searchableName(remoteName(inputPath))
	}
	ext := strings.ToLower(filepath.Ext(inputPath))
	base := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
	if ext == ".pdf" {
		return base + "-searchable.pdf"
	}
	return base + ".pdf"
}

// remoteName lấy tên file từ URL; không có thì dùng "input".
func remoteName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "input"
	}
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		return "input"
	}
	return name
}

// searchableName thêm hậu tố -searchable cho tên file PDF.
func searchableName(name string) string {
	ext := strings.ToLower(path.Ext(name))
	base := strings.TrimSuffix(name, path.Ext(name))
	if ext == ".pdf" {
		return base + "-searchable.pdf"
	}
	return base + ".pdf"
}

// prepareEngine chọn engine và ngôn ngữ cho lần chạy này.
// Nếu đang ở chế độ auto, nó OCR thử trang đầu để dò ngôn ngữ và trả kèm kết quả
// của lần thắng, để Render không phải chạy lại trang đầu.
func (c *Converter) prepareEngine(ctx context.Context, pages []imaging.Page) (ocr.Engine, string, []ocr.Line, error) {
	if c.engine != nil {
		lang := c.language
		if isAuto(lang) {
			lang = ""
		}
		return c.engine, lang, nil, nil
	}

	if !isAuto(c.language) {
		engine, err := c.newEngine(c.language)
		return engine, c.language, nil, err
	}

	if len(pages) == 0 {
		engine, err := c.newEngine("eng")
		return engine, "eng", nil, err
	}

	lang, lines := c.detectLanguage(ctx, pages[0].Image)
	engine, err := c.newEngine(lang)
	return engine, lang, lines, err
}

// detectLanguage chạy thử OCR trang đầu bằng từng traineddata đã cài và chọn
// ngôn ngữ có confidence trung bình cao nhất.
func (c *Converter) detectLanguage(ctx context.Context, img image.Image) (string, []ocr.Line) {
	candidates := c.candidateLanguages(ctx)
	if len(candidates) == 0 {
		return "eng", nil
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	best, bestScore := "", -1.0
	var bestLines []ocr.Line
	for _, lang := range candidates {
		engine, err := c.newEngine(lang)
		if err != nil {
			continue
		}
		lines, err := engine.Recognize(ctx, img)
		if err != nil || len(lines) == 0 {
			continue
		}
		if score := meanConfidence(lines); score > bestScore {
			best, bestScore, bestLines = lang, score, lines
		}
	}
	if best == "" {
		return candidates[0], nil
	}
	return best, bestLines
}

// candidateLanguages liệt kê các traineddata đã cài (bỏ "osd") để đem ra thử.
func (c *Converter) candidateLanguages(ctx context.Context) []string {
	engine, err := c.newEngine("eng")
	if err != nil {
		return nil
	}
	reporter, ok := engine.(ocr.LanguageReporter)
	if !ok {
		return nil
	}
	langs, err := reporter.Languages(ctx)
	if err != nil {
		return nil
	}

	out := make([]string, 0, len(langs))
	for _, lang := range langs {
		if lang == "" || lang == "osd" {
			continue
		}
		out = append(out, lang)
	}
	if len(out) > maxLanguageCandidates {
		out = out[:maxLanguageCandidates]
	}
	return out
}

// meanConfidence tính confidence trung bình, có trọng số theo độ dài từ.
func meanConfidence(lines []ocr.Line) float64 {
	var weighted, total float64
	for _, ln := range lines {
		for _, w := range ln.Words {
			n := float64(utf8.RuneCountInString(w.Text))
			weighted += w.Conf * n
			total += n
		}
	}
	if total == 0 {
		return 0
	}
	return weighted / total
}

// isAuto cho biết ngôn ngữ có ở chế độ tự dò hay không.
func isAuto(lang string) bool {
	return lang == "" || strings.EqualFold(lang, autoLanguage)
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
