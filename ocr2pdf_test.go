package ocr2pdf

import (
	"testing"

	"github.com/TheronThayer9999/ocr2pdf/ocr"
)

func TestDefaultOutputPath(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"data/scan200.pdf", "data/scan200-searchable.pdf"},
		{`data\image_one.png`, `data\image_one.pdf`},
		{"https://example.com/files/scan.pdf", "scan-searchable.pdf"},
		{"https://example.com/files/photo.jpg", "photo.pdf"},
		{"https://example.com/download", "download.pdf"},
		{"https://example.com/", "input.pdf"},
	} {
		if got := DefaultOutputPath(tc.in); got != tc.want {
			t.Errorf("DefaultOutputPath(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
	}
}

func TestIsAuto(t *testing.T) {
	for _, lang := range []string{"", "auto", "AUTO", "Auto"} {
		if !isAuto(lang) {
			t.Errorf("isAuto(%q) = false, muốn true", lang)
		}
	}
	for _, lang := range []string{"eng", "vie", "vie+eng"} {
		if isAuto(lang) {
			t.Errorf("isAuto(%q) = true, muốn false", lang)
		}
	}
}

func TestMeanConfidenceWeighsByWordLength(t *testing.T) {
	lines := []ocr.Line{
		{Words: []ocr.Word{{Text: "a", Conf: 100}}},
		{Words: []ocr.Word{{Text: "abcd", Conf: 50}}},
	}

	// (100*1 + 50*4) / 5 = 60
	if got := meanConfidence(lines); got != 60 {
		t.Errorf("meanConfidence = %v, muốn 60", got)
	}
	if got := meanConfidence(nil); got != 0 {
		t.Errorf("meanConfidence(nil) = %v, muốn 0", got)
	}
}
