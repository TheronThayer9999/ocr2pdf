package fonts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// ttfFont cài đặt Font bằng golang.org/x/image/font/sfnt.
type ttfFont struct {
	name       string
	program    []byte
	unitsPerEm int
	ascent     int
	descent    int
	capHeight  int

	sfnt   *sfnt.Font
	buf    sfnt.Buffer
	widths map[Glyph]int
	uni    map[Glyph]rune
}

// TTFLoader nạp font TTF, tự dò font hệ thống khi không chỉ định đường dẫn.
type TTFLoader struct {
	candidates []string // danh sách ưu tiên, rỗng = mặc định của hệ điều hành
}

// NewTTFLoader tạo loader dùng danh sách font mặc định.
func NewTTFLoader() *TTFLoader {
	return &TTFLoader{}
}

// WithCandidates thay danh sách font sẽ thử (theo thứ tự).
func (l *TTFLoader) WithCandidates(paths ...string) *TTFLoader {
	l.candidates = paths
	return l
}

// Load nạp font; nếu path rỗng thì thử lần lượt các font mặc định.
func (l *TTFLoader) Load(path string) (Font, error) {
	candidates := l.candidates
	switch {
	case path != "":
		candidates = []string{path}
	case len(candidates) == 0:
		candidates = systemCandidates()
	}

	var lastErr error
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			lastErr = err
			continue
		}
		f, err := newTTFFont(p, data)
		if err != nil {
			lastErr = err
			continue
		}
		return f, nil
	}
	if lastErr == nil {
		lastErr = errors.New("không có font nào để thử")
	}
	return nil, fmt.Errorf("không nạp được font TTF nào (dùng WithFont để chỉ định): %w", lastErr)
}

func systemCandidates() []string {
	var out []string
	if p := os.Getenv("OCR2PDF_FONT"); p != "" {
		out = append(out, p)
	}
	if win := os.Getenv("WINDIR"); win != "" {
		out = append(out,
			filepath.Join(win, "Fonts", "arial.ttf"),
			filepath.Join(win, "Fonts", "segoeui.ttf"),
			filepath.Join(win, "Fonts", "tahoma.ttf"),
		)
	}
	return append(out,
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/System/Library/Fonts/Supplemental/Arial.ttf",
	)
}

func newTTFFont(path string, data []byte) (*ttfFont, error) {
	sf, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font %s: %w", path, err)
	}
	upem := int(sf.UnitsPerEm())
	if upem <= 0 {
		return nil, fmt.Errorf("font %s có UnitsPerEm không hợp lệ", path)
	}

	f := &ttfFont{
		name:       "OCRFont",
		program:    data,
		unitsPerEm: upem,
		ascent:     upem * 3 / 4,
		descent:    -upem / 4,
		capHeight:  upem * 7 / 10,
		sfnt:       sf,
		widths:     make(map[Glyph]int),
		uni:        make(map[Glyph]rune),
	}

	// Đo tại ppem = UnitsPerEm nên giá trị trả về (26.6) chia 64 ra font unit.
	if m, err := sf.Metrics(&f.buf, fixed.I(upem), font.HintingNone); err == nil {
		f.ascent = int(m.Ascent) / 64
		f.descent = int(m.Descent) / 64
		if m.CapHeight > 0 {
			f.capHeight = int(m.CapHeight) / 64
		}
	}

	f.buildUnicodeMap()
	return f, nil
}

// buildUnicodeMap quét BMP để dựng bảng glyph -> rune (dùng cho ToUnicode).
func (f *ttfFont) buildUnicodeMap() {
	for r := rune(0); r < 0xFFFF; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		gid, err := f.sfnt.GlyphIndex(&f.buf, r)
		if err != nil || gid == 0 {
			continue
		}
		if _, ok := f.uni[Glyph(gid)]; !ok {
			f.uni[Glyph(gid)] = r
		}
	}
}

func (f *ttfFont) Name() string    { return f.name }
func (f *ttfFont) UnitsPerEm() int { return f.unitsPerEm }
func (f *ttfFont) Ascent() int     { return f.ascent }
func (f *ttfFont) Descent() int    { return f.descent }
func (f *ttfFont) CapHeight() int  { return f.capHeight }
func (f *ttfFont) Program() []byte { return f.program }

func (f *ttfFont) Glyphs(text string) []Glyph {
	out := make([]Glyph, 0, len(text))
	for _, r := range text {
		gid, err := f.sfnt.GlyphIndex(&f.buf, r)
		if err != nil || gid == 0 {
			continue
		}
		out = append(out, Glyph(gid))
	}
	return out
}

func (f *ttfFont) Width(g Glyph) int {
	if w, ok := f.widths[g]; ok {
		return w
	}
	w := f.unitsPerEm / 2
	if adv, err := f.sfnt.GlyphAdvance(&f.buf, sfnt.GlyphIndex(g), fixed.I(f.unitsPerEm), font.HintingNone); err == nil {
		w = int(adv) / 64
	}
	f.widths[g] = w
	return w
}

func (f *ttfFont) Width1000(g Glyph) int {
	return f.Width(g) * 1000 / f.unitsPerEm
}

func (f *ttfFont) TextWidth(glyphs []Glyph, size float64) float64 {
	total := 0
	for _, g := range glyphs {
		total += f.Width(g)
	}
	return float64(total) / float64(f.unitsPerEm) * size
}

func (f *ttfFont) Unicode(g Glyph) (rune, bool) {
	r, ok := f.uni[g]
	return r, ok
}

// Subset tạo font rút gọn chỉ chứa glyph cần dùng.
func (f *ttfFont) Subset(used map[Glyph]bool) ([]byte, map[Glyph]Glyph, error) {
	return subsetProgram(f.program, used)
}
