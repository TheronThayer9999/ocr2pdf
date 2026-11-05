package fonts

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// subsetProgram tạo bản TTF chỉ chứa glyph 0 và các glyph cần dùng.
// Trả về chương trình font mới và bảng ánh xạ glyph cũ -> mới.
func subsetProgram(data []byte, used map[Glyph]bool) ([]byte, map[Glyph]Glyph, error) {
	tables, err := parseSFNT(data)
	if err != nil {
		return nil, nil, err
	}

	head, maxp := tables["head"], tables["maxp"]
	hhea, hmtx := tables["hhea"], tables["hmtx"]
	loca, glyf := tables["loca"], tables["glyf"]
	if head == nil || maxp == nil || hhea == nil || hmtx == nil || loca == nil || glyf == nil {
		return nil, nil, fmt.Errorf("font thiếu bảng bắt buộc (head/maxp/hhea/hmtx/loca/glyf)")
	}

	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:6]))
	if numGlyphs == 0 {
		return nil, nil, fmt.Errorf("font không có glyph")
	}
	numHMetrics := int(binary.BigEndian.Uint16(hhea[34:36]))
	if numHMetrics == 0 {
		return nil, nil, fmt.Errorf("font có numberOfHMetrics = 0")
	}

	offsets, err := locaOffsets(loca, numGlyphs, binary.BigEndian.Uint16(head[50:52]) == 0)
	if err != nil {
		return nil, nil, err
	}

	glyphData := func(g int) []byte {
		if g < 0 || g >= numGlyphs {
			return nil
		}
		start, end := int(offsets[g]), int(offsets[g+1])
		if start > end || end > len(glyf) {
			return nil
		}
		return glyf[start:end]
	}
	advance := func(g int) uint16 {
		if g < numHMetrics {
			return binary.BigEndian.Uint16(hmtx[g*4 : g*4+2])
		}
		return binary.BigEndian.Uint16(hmtx[(numHMetrics-1)*4 : (numHMetrics-1)*4+2])
	}
	lsb := func(g int) int16 {
		if g < numHMetrics {
			return int16(binary.BigEndian.Uint16(hmtx[g*4+2 : g*4+4]))
		}
		idx := numHMetrics*4 + (g-numHMetrics)*2
		if idx+2 <= len(hmtx) {
			return int16(binary.BigEndian.Uint16(hmtx[idx : idx+2]))
		}
		return 0
	}

	keep := keepGlyphs(used, numGlyphs, glyphData)
	order := make([]int, 0, len(keep))
	for g := range keep {
		order = append(order, g)
	}
	sort.Ints(order)

	remap := make(map[Glyph]Glyph, len(order))
	newIndex := make(map[int]int, len(order))
	for i, g := range order {
		remap[Glyph(g)] = Glyph(i)
		newIndex[g] = i
	}

	// glyf + loca mới (loca luôn dùng dạng long: 4 byte mỗi entry).
	var newGlyf []byte
	newLoca := make([]byte, (len(order)+1)*4)
	for i, g := range order {
		binary.BigEndian.PutUint32(newLoca[i*4:i*4+4], uint32(len(newGlyf)))
		gd := glyphData(g)
		if len(gd) > 0 {
			if int16(binary.BigEndian.Uint16(gd[0:2])) < 0 {
				gd = remapComposite(gd, newIndex)
			}
			newGlyf = append(newGlyf, gd...)
		}
		for len(newGlyf)%4 != 0 {
			newGlyf = append(newGlyf, 0)
		}
	}
	binary.BigEndian.PutUint32(newLoca[len(order)*4:], uint32(len(newGlyf)))

	newHmtx := make([]byte, len(order)*4)
	for i, g := range order {
		binary.BigEndian.PutUint16(newHmtx[i*4:i*4+2], advance(g))
		binary.BigEndian.PutUint16(newHmtx[i*4+2:i*4+4], uint16(lsb(g)))
	}

	newHead := append([]byte(nil), head...)
	binary.BigEndian.PutUint32(newHead[8:12], 0)  // checkSumAdjustment, tính sau
	binary.BigEndian.PutUint16(newHead[50:52], 1) // indexToLocFormat = long

	newHhea := append([]byte(nil), hhea...)
	binary.BigEndian.PutUint16(newHhea[34:36], uint16(len(order)))

	newMaxp := append([]byte(nil), maxp...)
	binary.BigEndian.PutUint16(newMaxp[4:6], uint16(len(order)))

	// post phiên bản 3.0: không chứa tên glyph.
	newPost := make([]byte, 32)
	binary.BigEndian.PutUint32(newPost[0:4], 0x00030000)

	out := []sfntTable{
		{"glyf", newGlyf},
		{"head", newHead},
		{"hhea", newHhea},
		{"hmtx", newHmtx},
		{"loca", newLoca},
		{"maxp", newMaxp},
		{"post", newPost},
	}
	// Giữ bảng hinting nếu có (không tham chiếu glyph id).
	for _, tag := range []string{"cvt ", "fpgm", "prep"} {
		if t, ok := tables[tag]; ok {
			out = append(out, sfntTable{tag, t})
		}
	}

	return assembleSFNT(out), remap, nil
}

// keepGlyphs gồm glyph 0, glyph cần dùng và mọi glyph thành phần của glyph ghép.
func keepGlyphs(used map[Glyph]bool, numGlyphs int, glyphData func(int) []byte) map[int]bool {
	keep := map[int]bool{0: true}
	for g := range used {
		if int(g) < numGlyphs {
			keep[int(g)] = true
		}
	}
	for {
		added := false
		for g := range keep {
			gd := glyphData(g)
			if len(gd) < 10 || int16(binary.BigEndian.Uint16(gd[0:2])) >= 0 {
				continue // glyph đơn giản, không tham chiếu glyph khác
			}
			for _, cg := range compositeGlyphs(gd) {
				if cg < numGlyphs && !keep[cg] {
					keep[cg] = true
					added = true
				}
			}
		}
		if !added {
			return keep
		}
	}
}

func locaOffsets(loca []byte, numGlyphs int, short bool) ([]uint32, error) {
	offsets := make([]uint32, numGlyphs+1)
	for i := 0; i <= numGlyphs; i++ {
		if short {
			if (i+1)*2 > len(loca) {
				return nil, fmt.Errorf("bảng loca ngắn")
			}
			offsets[i] = uint32(binary.BigEndian.Uint16(loca[i*2:i*2+2])) * 2
			continue
		}
		if (i+1)*4 > len(loca) {
			return nil, fmt.Errorf("bảng loca ngắn")
		}
		offsets[i] = binary.BigEndian.Uint32(loca[i*4 : i*4+4])
	}
	return offsets, nil
}

// compositeGlyphs trả về các glyph thành phần của một glyph ghép.
func compositeGlyphs(gd []byte) []int {
	var out []int
	p := 10
	for p+4 <= len(gd) {
		flags := binary.BigEndian.Uint16(gd[p : p+2])
		out = append(out, int(binary.BigEndian.Uint16(gd[p+2:p+4])))
		p += 4
		if flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
			p += 4
		} else {
			p += 2
		}
		switch {
		case flags&0x0008 != 0: // WE_HAVE_A_SCALE
			p += 2
		case flags&0x0040 != 0: // WE_HAVE_AN_X_AND_Y_SCALE
			p += 4
		case flags&0x0080 != 0: // WE_HAVE_A_TWO_BY_TWO
			p += 8
		}
		if flags&0x0020 == 0 { // MORE_COMPONENTS
			break
		}
	}
	return out
}

// remapComposite đổi glyph id của các thành phần theo bảng ánh xạ mới.
func remapComposite(gd []byte, newIndex map[int]int) []byte {
	out := append([]byte(nil), gd...)
	p := 10
	for p+4 <= len(out) {
		flags := binary.BigEndian.Uint16(out[p : p+2])
		cg := int(binary.BigEndian.Uint16(out[p+2 : p+4]))
		if ni, ok := newIndex[cg]; ok {
			binary.BigEndian.PutUint16(out[p+2:p+4], uint16(ni))
		}
		p += 4
		if flags&0x0001 != 0 {
			p += 4
		} else {
			p += 2
		}
		switch {
		case flags&0x0008 != 0:
			p += 2
		case flags&0x0040 != 0:
			p += 4
		case flags&0x0080 != 0:
			p += 8
		}
		if flags&0x0020 == 0 {
			break
		}
	}
	return out
}

type sfntTable struct {
	tag  string
	data []byte
}

// parseSFNT đọc table directory của file TTF/OTF.
func parseSFNT(data []byte) (map[string][]byte, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("font quá ngắn")
	}
	n := int(binary.BigEndian.Uint16(data[4:6]))
	out := make(map[string][]byte, n)
	for i := 0; i < n; i++ {
		rec := 12 + i*16
		if rec+16 > len(data) {
			return nil, fmt.Errorf("table directory không hợp lệ")
		}
		tag := string(data[rec : rec+4])
		offset := int(binary.BigEndian.Uint32(data[rec+8 : rec+12]))
		length := int(binary.BigEndian.Uint32(data[rec+12 : rec+16]))
		if offset < 0 || length < 0 || offset+length > len(data) {
			return nil, fmt.Errorf("bảng %q nằm ngoài file", tag)
		}
		out[tag] = data[offset : offset+length]
	}
	return out, nil
}

// assembleSFNT ráp các bảng thành file TTF hoàn chỉnh (kèm checksum).
func assembleSFNT(tables []sfntTable) []byte {
	sort.Slice(tables, func(i, j int) bool { return tables[i].tag < tables[j].tag })

	n := len(tables)
	pow, entrySelector := 1, 0
	for pow*2 <= n {
		pow *= 2
		entrySelector++
	}
	header := 12 + n*16

	offsets := make([]int, n)
	pos := header
	for i, t := range tables {
		offsets[i] = pos
		pos += len(t.data)
		if pad := (4 - pos%4) % 4; pad > 0 {
			pos += pad
		}
	}

	out := make([]byte, pos)
	binary.BigEndian.PutUint32(out[0:4], 0x00010000)
	binary.BigEndian.PutUint16(out[4:6], uint16(n))
	binary.BigEndian.PutUint16(out[6:8], uint16(pow*16))
	binary.BigEndian.PutUint16(out[8:10], uint16(entrySelector))
	binary.BigEndian.PutUint16(out[10:12], uint16(n*16-pow*16))

	for i, t := range tables {
		rec := 12 + i*16
		copy(out[rec:rec+4], t.tag)
		binary.BigEndian.PutUint32(out[rec+4:rec+8], tableChecksum(t.data))
		binary.BigEndian.PutUint32(out[rec+8:rec+12], uint32(offsets[i]))
		binary.BigEndian.PutUint32(out[rec+12:rec+16], uint32(len(t.data)))
		copy(out[offsets[i]:], t.data)
	}

	// checkSumAdjustment = 0xB1B0AFBA - checksum(toàn file với head.checkSumAdjustment = 0)
	for i, t := range tables {
		if t.tag == "head" {
			adj := uint32(0xB1B0AFBA) - tableChecksum(out)
			binary.BigEndian.PutUint32(out[offsets[i]+8:offsets[i]+12], adj)
			break
		}
	}
	return out
}

func tableChecksum(data []byte) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		var v uint32
		for j := 0; j < 4; j++ {
			v <<= 8
			if i+j < len(data) {
				v |= uint32(data[i+j])
			}
		}
		sum += v
	}
	return sum
}
