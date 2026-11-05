package pdf

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/TheronThayer9999/ocr2pdf/fonts"
)

// fontResource ghi các object font (Type0 + CIDFontType2) và giữ bảng ánh xạ glyph.
type fontResource struct {
	num   int // object của font Type0 dùng trong /Resources
	font  fonts.Font
	remap map[fonts.Glyph]fonts.Glyph
}

// newFontResource nhúng font (đã rút gọn nếu được) và trả về resource dùng cho trang.
func newFontResource(doc *Document, f fonts.Font, used map[fonts.Glyph]bool) *fontResource {
	program := f.Program()
	remap := identityRemap(used)
	if sub, m, err := f.Subset(used); err == nil && len(sub) > 0 {
		program, remap = sub, m
	}

	type0 := doc.Reserve()
	cidFont := doc.Reserve()
	descriptor := doc.Reserve()
	fontFile := doc.Reserve()
	toUnicode := doc.Reserve()

	doc.Set(fontFile, doc.Stream("/Length1 "+strconv.Itoa(len(program)), program))

	// Các metric trong FontDescriptor tính theo glyph space 1/1000 em.
	k := 1000.0 / float64(f.UnitsPerEm())
	doc.Set(descriptor, []byte(fmt.Sprintf(
		"<< /Type /FontDescriptor /FontName /%s /Flags 4 /FontBBox [0 %.0f 1000 1000] /ItalicAngle 0 /Ascent %.0f /Descent %.0f /CapHeight %.0f /StemV 80 /FontFile2 %d 0 R >>",
		f.Name(), float64(-f.UnitsPerEm()/5)*k, float64(f.Ascent())*k, float64(f.Descent())*k, float64(f.CapHeight())*k, fontFile)))

	doc.Set(cidFont, []byte(fmt.Sprintf(
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /DW 1000 /W %s /CIDToGIDMap /Identity >>",
		f.Name(), descriptor, widthsArray(f, used, remap))))

	doc.Set(type0, []byte(fmt.Sprintf(
		"<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>",
		f.Name(), cidFont, toUnicode)))

	doc.Set(toUnicode, doc.Stream("", toUnicodeCMap(f, used, remap)))

	return &fontResource{num: type0, font: f, remap: remap}
}

// glyph trả về CID thực tế trong font đã nhúng.
func (r *fontResource) glyph(g fonts.Glyph) fonts.Glyph {
	if mapped, ok := r.remap[g]; ok {
		return mapped
	}
	return g
}

func identityRemap(used map[fonts.Glyph]bool) map[fonts.Glyph]fonts.Glyph {
	remap := make(map[fonts.Glyph]fonts.Glyph, len(used))
	for g := range used {
		remap[g] = g
	}
	return remap
}

// sortedGlyphs sắp xếp glyph id để output ổn định giữa các lần chạy.
func sortedGlyphs(used map[fonts.Glyph]bool) []fonts.Glyph {
	out := make([]fonts.Glyph, 0, len(used))
	for g := range used {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// widthsArray tạo mảng /W của CIDFont.
func widthsArray(f fonts.Font, used map[fonts.Glyph]bool, remap map[fonts.Glyph]fonts.Glyph) string {
	var b bytes.Buffer
	b.WriteString("[")
	for _, g := range sortedGlyphs(used) {
		cid, ok := remap[g]
		if !ok {
			cid = g
		}
		fmt.Fprintf(&b, " %d [%d]", cid, f.Width1000(g))
	}
	b.WriteString(" ]")
	return b.String()
}

// toUnicodeCMap sinh bảng glyph -> ký tự để trình đọc PDF copy/search đúng.
func toUnicodeCMap(f fonts.Font, used map[fonts.Glyph]bool, remap map[fonts.Glyph]fonts.Glyph) []byte {
	type pair struct {
		cid fonts.Glyph
		r   rune
	}
	var pairs []pair
	for _, g := range sortedGlyphs(used) {
		r, ok := f.Unicode(g)
		if !ok || r == 0 {
			continue
		}
		cid, ok := remap[g]
		if !ok {
			cid = g
		}
		pairs = append(pairs, pair{cid, r})
	}

	var b bytes.Buffer
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	b.WriteString("/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	b.WriteString("/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n")
	b.WriteString("1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")

	const chunk = 100
	for i := 0; i < len(pairs); i += chunk {
		end := i + chunk
		if end > len(pairs) {
			end = len(pairs)
		}
		fmt.Fprintf(&b, "%d beginbfchar\n", end-i)
		for _, p := range pairs[i:end] {
			fmt.Fprintf(&b, "<%04X> <%s>\n", uint16(p.cid), utf16BE(p.r))
		}
		b.WriteString("endbfchar\n")
	}

	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return b.Bytes()
}

func utf16BE(r rune) string {
	if r <= 0xFFFF {
		return fmt.Sprintf("%04X", uint16(r))
	}
	r -= 0x10000
	hi := uint16(0xD800 + (r >> 10))
	lo := uint16(0xDC00 + (r & 0x3FF))
	return fmt.Sprintf("%04X%04X", hi, lo)
}
