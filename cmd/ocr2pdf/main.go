// Command ocr2pdf is a reference CLI for the ocr2pdf package.
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
	in := flag.String("in", "", "input image (png/jpg) or PDF")
	out := flag.String("out", "", "output PDF (default: x.png -> x.pdf, x.pdf -> x-searchable.pdf)")
	lang := flag.String("lang", "auto", "tesseract language: auto (detect), eng, vie, vie+eng")
	psm := flag.Int("psm", 3, "tesseract page segmentation mode")
	dpi := flag.Float64("dpi", 0, "force a DPI (0 = read from the file / PDF page size)")
	conf := flag.Float64("conf", 30, "drop words below this confidence")
	tess := flag.String("tesseract", "", "path to tesseract.exe (default: auto-discovered)")
	font := flag.String("font", "", "TTF file for the text layer (default: system font)")
	native := flag.Bool("native", false, "let tesseract write the PDF itself (images only)")
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
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(res)
}

// warnMissingLanguage warns when the traineddata for a language is not installed.
func warnMissingLanguage(lang string, opts []ocr2pdf.Option) {
	if lang == "" || strings.EqualFold(lang, "auto") {
		return
	}

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
		fmt.Fprintf(os.Stderr, "warning: no traineddata for %q (installed: %s)\n",
			strings.Join(missing, ", "), strings.Join(langs, ", "))
	}
}
