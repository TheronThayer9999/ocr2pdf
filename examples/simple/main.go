// Command simple is the smallest possible use of the ocr2pdf module.
//
// Run it from the repository root; the path is relative to your current directory:
//
//	go run ./examples/simple examples/simple/demo.png
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/TheronThayer9999/ocr2pdf"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run ./examples/simple <image-or-pdf>\n" +
			"example: go run ./examples/simple examples/simple/demo.png")
	}

	// Everything has a default: output name, DPI, font, Tesseract binary.
	// Add options when you need them, for example:
	//   ocr2pdf.WithLanguage("vie"), ocr2pdf.WithOutput("result.pdf")
	res, err := ocr2pdf.Convert(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res) // examples/simple/demo.pdf (134.9 KB) - 1 page, 30 lines, 312 words - 607ms
}
