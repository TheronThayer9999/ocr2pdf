package ocr

import (
	"strconv"
	"strings"
)

// Parser chuyển output TSV của tesseract thành danh sách dòng.
type Parser interface {
	Parse(tsv string) ([]Line, error)
}

// TSVParser đọc TSV với các level: 4 = dòng, 5 = từ.
type TSVParser struct {
	MinConfidence float64 // bỏ qua từ có confidence thấp hơn
}

// NewTSVParser tạo parser với ngưỡng confidence cho trước.
func NewTSVParser(minConfidence float64) *TSVParser {
	return &TSVParser{MinConfidence: minConfidence}
}

// Parse đọc toàn bộ TSV và gom từ vào dòng tương ứng.
func (p *TSVParser) Parse(tsv string) ([]Line, error) {
	var lines []Line
	var cur *Line

	flush := func() {
		if cur != nil && len(cur.Words) > 0 {
			lines = append(lines, *cur)
		}
		cur = nil
	}

	for _, row := range strings.Split(strings.ReplaceAll(tsv, "\r\n", "\n"), "\n") {
		// 12 cột: level page block par line word left top width height conf text
		fields := strings.SplitN(row, "\t", 12)
		if len(fields) < 12 {
			continue
		}

		switch fields[0] {
		case "4": // bắt đầu một dòng mới
			flush()
			top, errTop := strconv.Atoi(fields[7])
			height, errHeight := strconv.Atoi(fields[9])
			if errTop != nil || errHeight != nil || height <= 0 {
				continue
			}
			cur = &Line{Top: top, Height: height}

		case "5": // một từ
			word, ok := p.parseWord(fields)
			if !ok {
				continue
			}
			if cur == nil {
				// Tesseract không xuất level 4 (một số psm) -> tự tạo dòng từ hộp của từ.
				cur = &Line{Top: atoi(fields[7]), Height: atoi(fields[9])}
			}
			cur.Words = append(cur.Words, word)
			if cur.Text == "" {
				cur.Text = word.Text
			} else {
				cur.Text += " " + word.Text
			}
			cur.grow(atoi(fields[7]), atoi(fields[9]))
		}
	}
	flush()
	return lines, nil
}

func (p *TSVParser) parseWord(fields []string) (Word, bool) {
	text := strings.TrimSpace(fields[11])
	if text == "" {
		return Word{}, false
	}
	conf, err := strconv.ParseFloat(fields[10], 64)
	if err != nil || conf < p.MinConfidence {
		return Word{}, false
	}
	left, errLeft := strconv.Atoi(fields[6])
	top, errTop := strconv.Atoi(fields[7])
	width, errWidth := strconv.Atoi(fields[8])
	height, errHeight := strconv.Atoi(fields[9])
	if errLeft != nil || errTop != nil || errWidth != nil || errHeight != nil {
		return Word{}, false
	}
	if width <= 0 || height <= 0 {
		return Word{}, false
	}
	return Word{Text: text, Left: left, Top: top, Width: width, Height: height, Conf: conf}, true
}

// grow nới hộp dòng để bao trọn một từ (dùng khi dòng được tạo từ chính các từ).
func (l *Line) grow(wordTop, wordHeight int) {
	if wordTop < l.Top {
		l.Height += l.Top - wordTop
		l.Top = wordTop
	}
	if bottom := wordTop + wordHeight; bottom > l.Top+l.Height {
		l.Height = bottom - l.Top
	}
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
