package ocr

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"
	"time"
)

// Tesseract là Engine gọi CLI tesseract.exe.
type Tesseract struct {
	path    string
	lang    string
	oem     int
	psm     int
	timeout time.Duration
	runner  Runner
	parser  Parser
}

// Option cấu hình Tesseract.
type Option func(*Tesseract)

// WithLanguage đặt ngôn ngữ OCR, ví dụ "eng", "vie", "vie+eng".
func WithLanguage(lang string) Option {
	return func(t *Tesseract) { t.lang = lang }
}

// WithOEM đặt OCR Engine Mode (0..3).
func WithOEM(oem int) Option {
	return func(t *Tesseract) { t.oem = oem }
}

// WithPSM đặt Page Segmentation Mode (0..13).
func WithPSM(psm int) Option {
	return func(t *Tesseract) { t.psm = psm }
}

// WithTimeout đặt thời gian tối đa cho một lần gọi.
func WithTimeout(d time.Duration) Option {
	return func(t *Tesseract) { t.timeout = d }
}

// NewTesseract tạo engine; finder quyết định binary nào được dùng.
func NewTesseract(f Finder, r Runner, p Parser, opts ...Option) (*Tesseract, error) {
	if f == nil || r == nil {
		return nil, fmt.Errorf("cần finder và runner để tạo engine tesseract")
	}
	path, err := f.Find()
	if err != nil {
		return nil, err
	}
	if p == nil {
		p = NewTSVParser(0)
	}

	t := &Tesseract{
		path:    path,
		lang:    "eng",
		oem:     3,
		psm:     3,
		timeout: 2 * time.Minute,
		runner:  r,
		parser:  p,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t, nil
}

// Path trả về binary tesseract đang dùng.
func (t *Tesseract) Path() string { return t.path }

// Language trả về ngôn ngữ đang cấu hình.
func (t *Tesseract) Language() string { return t.lang }

// Recognize ghi ảnh ra file tạm rồi OCR (tesseract CLI chỉ nhận file).
func (t *Tesseract) Recognize(ctx context.Context, img image.Image) ([]Line, error) {
	path, cleanup, err := writeTempPNG(img)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return t.RecognizeFile(ctx, path)
}

// RecognizeFile OCR một file ảnh có sẵn.
func (t *Tesseract) RecognizeFile(ctx context.Context, imagePath string) ([]Line, error) {
	tsv, err := t.run(ctx, imagePath, "tsv")
	if err != nil {
		return nil, err
	}
	lines, err := t.parser.Parse(tsv)
	if err != nil {
		return nil, err
	}
	// Trang trắng trả danh sách rỗng chứ không phải lỗi: caller tự quyết định
	// (CLI báo 0 từ, tab Quét ghi 0 từ cho trang đó).
	return lines, nil
}

// Languages trả về danh sách traineddata đã cài.
func (t *Tesseract) Languages(ctx context.Context) ([]string, error) {
	ctx, cancel := t.withTimeout(ctx)
	defer cancel()

	out, err := t.runner.Run(ctx, t.path, "--list-langs")
	if err != nil {
		return nil, err
	}

	// Dòng đầu là header "List of available languages in ... (N):"
	var langs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.ContainsAny(line, " :\\/") {
			continue
		}
		langs = append(langs, line)
	}
	return langs, nil
}

// RenderPDF để tesseract tự xuất PDF có text layer (ảnh + chữ ẩn).
func (t *Tesseract) RenderPDF(ctx context.Context, imagePath, outputPath string) error {
	// Tesseract tự thêm đuôi .pdf.
	outputPath = strings.TrimSuffix(outputPath, ".pdf")
	ctx, cancel := t.withTimeout(ctx)
	defer cancel()

	args := []string{imagePath, outputPath, "-l", t.lang, "--oem", strconv.Itoa(t.oem), "--psm", strconv.Itoa(t.psm), "pdf"}
	_, err := t.runner.Run(ctx, t.path, args...)
	return err
}

// run gọi tesseract ở chế độ "stdout" với định dạng đầu ra cho trước.
func (t *Tesseract) run(ctx context.Context, imagePath, format string) (string, error) {
	ctx, cancel := t.withTimeout(ctx)
	defer cancel()

	args := []string{
		imagePath,
		"stdout",
		"-l", t.lang,
		"--oem", strconv.Itoa(t.oem),
		"--psm", strconv.Itoa(t.psm),
		format,
	}
	return t.runner.Run(ctx, t.path, args...)
}

func (t *Tesseract) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if t.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, t.timeout)
}

// writeTempPNG ghi ảnh ra file PNG tạm và trả về hàm dọn dẹp.
func writeTempPNG(img image.Image) (string, func(), error) {
	f, err := os.CreateTemp("", "ocr2pdf-*.png")
	if err != nil {
		return "", func() {}, err
	}
	name := f.Name()
	cleanup := func() {
		f.Close()
		os.Remove(name)
	}
	if err := png.Encode(f, img); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", func() {}, err
	}
	return name, cleanup, nil
}
