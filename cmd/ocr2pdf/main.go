// Command ocr2pdf là CLI mẫu cho package ocr2pdf.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TheronThayer9999/ocr2pdf"
)

func main() {
	in := flag.String("in", "", "ảnh (png/jpg) hoặc PDF đầu vào")
	out := flag.String("out", "", "PDF đầu ra (mặc định: x.png -> x.pdf, x.pdf -> x-searchable.pdf)")
	lang := flag.String("lang", "eng", "ngôn ngữ tesseract: eng, vie, vie+eng")
	psm := flag.Int("psm", 3, "page segmentation mode của tesseract")
	dpi := flag.Float64("dpi", 0, "ép DPI (0 = tự đọc từ tệp / khổ trang PDF)")
	conf := flag.Float64("conf", 30, "bỏ qua từ có confidence thấp hơn ngưỡng")
	tess := flag.String("tesseract", "", "đường dẫn tesseract.exe (mặc định: tự tìm)")
	font := flag.String("font", "", "file TTF cho lớp text ẩn (mặc định: font hệ thống)")
	native := flag.Bool("native", false, "để tesseract tự xuất PDF (chỉ với ảnh)")
	flag.Parse()

	if *in == "" {
		flag.Usage()
		os.Exit(2)
	}

	opts := []ocr2pdf.Option{
		ocr2pdf.WithLanguage(*lang),
		ocr2pdf.WithPSM(*psm),
		ocr2pdf.WithDPI(*dpi),
		ocr2pdf.WithMinConfidence(*conf),
		ocr2pdf.WithTesseractBinary(*tess),
		ocr2pdf.WithFont(*font),
	}

	if *out != "" {
		opts = append(opts, ocr2pdf.WithOutput(*out))
	}

	warnMissingLanguage(*lang, opts)

	run := ocr2pdf.Convert
	if *native {
		run = ocr2pdf.ConvertNative
	}
	res, err := run(*in, opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lỗi:", err)
		os.Exit(1)
	}
	fmt.Println(res)
}

// warnMissingLanguage nhắc khi traineddata của ngôn ngữ chưa được cài.
func warnMissingLanguage(lang string, opts []ocr2pdf.Option) {
	langs := ocr2pdf.Languages(context.Background(), opts...)
	if len(langs) == 0 {
		return
	}
	have := make(map[string]bool, len(langs))
	for _, l := range langs {
		have[l] = true
	}
	var missing []string
	for _, want := range strings.Split(lang, "+") {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "cảnh báo: chưa có traineddata cho %q (đang có: %s)\n",
			strings.Join(missing, ", "), strings.Join(langs, ", "))
	}
}
