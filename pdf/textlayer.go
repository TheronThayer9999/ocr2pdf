package pdf

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

// textLayer sinh lớp text ẩn (render mode 3) khớp vị trí từng từ trên ảnh.
type textLayer struct {
	font       *fontResource
	pageHeight float64
}

func newTextLayer(f *fontResource, pageHeight float64) *textLayer {
	return &textLayer{font: f, pageHeight: pageHeight}
}

// Render trả về các toán tử vẽ text cho toàn bộ dòng của một trang.
func (t *textLayer) Render(lines []Line) []byte {
	var b bytes.Buffer

	for _, ln := range lines {
		if len(ln.Words) == 0 || ln.Height <= 0 {
			continue
		}

		// Cả dòng dùng chung cỡ chữ + baseline để copy ra đúng thứ tự.
		size := ln.Height * 0.78
		baseline := ln.Top + ln.Height
		if hasDescender(ln.Text()) {
			baseline = ln.Top + ln.Height*0.8
		}
		y := t.pageHeight - baseline

		for _, w := range ln.Words {
			glyphs := t.font.font.Glyphs(w.Text)
			if len(glyphs) == 0 {
				continue
			}

			// Kéo giãn/co chữ theo chiều ngang để khớp bề rộng hộp nhận dạng.
			tz := 100.0
			if natural := t.font.font.TextWidth(glyphs, size); natural > 0 {
				tz = clamp(100*w.Width/natural, 5, 500)
			}

			cids := make([]byte, 0, len(glyphs)*2)
			for _, g := range glyphs {
				cid := uint16(t.font.glyph(g))
				cids = append(cids, byte(cid>>8), byte(cid))
			}

			fmt.Fprintf(&b, "BT\n/F1 %s Tf\n%s Tz\n3 Tr\n1 0 0 1 %s %s Tm\n<%s> Tj\nET\n",
				num(size), num(tz), num(w.X), num(y), hex.EncodeToString(cids))
		}
	}
	return b.Bytes()
}

// hasDescender cho biết chuỗi có ký tự thò xuống dưới baseline không.
func hasDescender(s string) bool {
	const descenders = "gjpqy,;()[]{}|/\\"
	return strings.ContainsAny(strings.ToLower(s), descenders)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
