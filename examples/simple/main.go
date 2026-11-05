// Command simple is the smallest possible use of the ocr2pdf module.
//
//	go run ./examples/simple data/scan200.pdf
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/TheronThayer9999/ocr2pdf"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: go run ./examples/simple <image-or-pdf>")
	}

	// Everything has a default: output name, DPI, font, Tesseract binary.
	// Add options when you need them, for example:
	//   ocr2pdf.WithLanguage("vie"), ocr2pdf.WithOutput("result.pdf")
	res, err := ocr2pdf.Convert(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res) // data/scan200-searchable.pdf (274.2 KB) - 1 page, 23 lines, 173 words - 1.0s
}
