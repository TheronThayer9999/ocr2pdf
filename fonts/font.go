// Package fonts nạp font TTF cho lớp text ẩn trong PDF và tạo bản font rút gọn.
package fonts

// Glyph là glyph id trong font (với Identity-H thì cũng chính là CID).
type Glyph uint16

// Font là những gì tầng PDF cần ở một font: độ rộng, glyph, unicode, chương trình font.
type Font interface {
	Name() string
	UnitsPerEm() int
	Ascent() int    // theo font unit
	Descent() int   // theo font unit, thường âm
	CapHeight() int // theo font unit

	// Glyphs chuyển chuỗi thành glyph id; ký tự không có trong font bị bỏ qua.
	Glyphs(text string) []Glyph
	// Width trả về advance của glyph theo font unit.
	Width(g Glyph) int
	// Width1000 trả về advance theo đơn vị 1/1000 em (dùng cho mảng /W).
	Width1000(g Glyph) int
	// TextWidth trả về bề rộng chuỗi (point) tại cỡ chữ size.
	TextWidth(glyphs []Glyph, size float64) float64
	// Unicode trả về ký tự ứng với glyph (phục vụ bảng ToUnicode).
	Unicode(g Glyph) (rune, bool)

	// Program trả về nội dung TTF đầy đủ.
	Program() []byte
	// Subset tạo font chỉ chứa các glyph cần dùng, kèm bảng ánh xạ glyph cũ -> mới.
	Subset(used map[Glyph]bool) (program []byte, remap map[Glyph]Glyph, err error)
}

// Loader nạp font từ đường dẫn (rỗng = tự tìm font hệ thống).
type Loader interface {
	Load(path string) (Font, error)
}
