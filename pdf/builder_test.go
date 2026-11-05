package pdf

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/TheronThayer9999/ocr2pdf/fonts"
)

// fakeFont là font giả để test builder mà không cần file TTF.
type fakeFont struct{}

func (fakeFont) Name() string    { return "Fake" }
func (fakeFont) UnitsPerEm() int { return 1000 }
func (fakeFont) Ascent() int     { return 750 }
func (fakeFont) Descent() int    { return -250 }
func (fakeFont) CapHeight() int  { return 700 }

func (fakeFont) Glyphs(text string) []fonts.Glyph {
	out := make([]fonts.Glyph, 0, len(text))
	for range text {
		out = append(out, 42)
	}
	return out
}

func (fakeFont) Width(fonts.Glyph) int     { return 500 }
func (fakeFont) Width1000(fonts.Glyph) int { return 500 }

func (fakeFont) TextWidth(glyphs []fonts.Glyph, size float64) float64 {
	return float64(len(glyphs)) * 500 / 1000 * size
}

func (fakeFont) Unicode(g fonts.Glyph) (rune, bool) { return 'A', true }
func (fakeFont) Program() []byte                    { return []byte("fake-ttf") }

func (fakeFont) Subset(map[fonts.Glyph]bool) ([]byte, map[fonts.Glyph]fonts.Glyph, error) {
	return nil, nil, errors.New("không hỗ trợ subset")
}

func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	return img
}

func testInput(pages int) Input {
	in := Input{Font: fakeFont{}}
	for i := 0; i < pages; i++ {
		in.Pages = append(in.Pages, Page{
			Image: Image{Data: testImage(100, 200), DPI: 100},
			Lines: []Line{{
				Top:    10,
				Height: 20,
				Words: []Word{
					{Text: "Hello", X: 5, Width: 40},
					{Text: "world", X: 50, Width: 45},
				},
			}},
		})
	}
	return in
}

func TestBuildCreatesValidStructure(t *testing.T) {
	data, err := NewBuilder().Build(testInput(2))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-1.7")) {
		t.Fatal("thiếu header PDF")
	}

	text := string(data)
	if !strings.Contains(text, "/Type /Pages /Kids [") || !strings.Contains(text, "/Count 2") {
		t.Error("cây trang không có 2 trang")
	}
	if strings.Count(text, "/Type /Page ") != 2 {
		t.Errorf("số object trang = %d, muốn 2", strings.Count(text, "/Type /Page "))
	}
	if !strings.Contains(text, "/Subtype /Image") {
		t.Error("thiếu ảnh XObject")
	}
	// Text ẩn: render mode 3 + glyph dạng hex.
	if !strings.Contains(text, "3 Tr") || !strings.Contains(text, "Tj") {
		t.Error("thiếu toán tử text ẩn")
	}
	// Font phải có ToUnicode và font nhúng.
	if !strings.Contains(text, "/ToUnicode") || !strings.Contains(text, "/FontFile2") {
		t.Error("thiếu ToUnicode hoặc font nhúng")
	}
}

func TestBuildXrefPointsToXrefTable(t *testing.T) {
	data, err := NewBuilder().Build(testInput(1))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	re := regexp.MustCompile(`startxref\s+(\d+)\s+%%EOF\s*$`)
	m := re.FindSubmatch(data)
	if m == nil {
		t.Fatal("không tìm thấy startxref")
	}
	offset, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatalf("startxref không phải số: %v", err)
	}
	if offset >= len(data) || !bytes.HasPrefix(data[offset:], []byte("xref")) {
		t.Errorf("startxref = %d không trỏ tới bảng xref", offset)
	}
}

func TestBuildPageSizeFollowsDPI(t *testing.T) {
	data, err := NewBuilder().Build(testInput(1))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// 100x200 px @ 100 dpi -> 72x144 point
	if !bytes.Contains(data, []byte("/MediaBox [0 0 72.00 144.00]")) {
		t.Error("MediaBox không đúng kích thước theo DPI")
	}
}

func TestBuildRejectsEmptyInput(t *testing.T) {
	if _, err := NewBuilder().Build(Input{Font: fakeFont{}}); err == nil {
		t.Fatal("muốn lỗi khi không có trang")
	}
	if _, err := NewBuilder().Build(Input{Pages: testInput(1).Pages}); err == nil {
		t.Fatal("muốn lỗi khi thiếu font")
	}
}

// TestBuildToMatchesBuild: đường streaming và đường in-memory phải cho ra CÙNG
// bytes — nếu lệch thì object/xref của một trong hai đường đang sai.
func TestBuildToMatchesBuild(t *testing.T) {
	in := testInput(3)
	want, err := NewBuilder().Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	sb, ok := NewBuilder().(StreamingBuilder)
	if !ok {
		t.Fatal("builder mặc định không hỗ trợ BuildTo")
	}
	var got bytes.Buffer
	if err := sb.BuildTo(&got, in); err != nil {
		t.Fatalf("BuildTo: %v", err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("BuildTo khác Build (%d vs %d bytes)", got.Len(), len(want))
	}
}

// TestBuildToLoadsImageOnlyWhenWritingPage khoá đúng mục tiêu của BuildTo: ảnh
// được nạp qua Open tại lúc ghi trang, không nạp sẵn cả tài liệu.
func TestBuildToLoadsImageOnlyWhenWritingPage(t *testing.T) {
	in := testInput(2)
	calls := 0
	for i := range in.Pages {
		in.Pages[i].Image = Image{
			DPI:    100,
			Width:  100,
			Height: 200,
			Open: func() (Image, error) {
				calls++
				return Image{Data: testImage(100, 200), DPI: 100}, nil
			},
		}
	}
	sb := NewBuilder().(StreamingBuilder)
	if err := sb.BuildTo(&bytes.Buffer{}, in); err != nil {
		t.Fatalf("BuildTo: %v", err)
	}
	if calls != 2 {
		t.Fatalf("Open gọi %d lần, muốn 2 (mỗi trang một lần)", calls)
	}
}

func TestBuildToPropagatesOpenError(t *testing.T) {
	in := testInput(1)
	in.Pages[0].Image = Image{DPI: 100, Width: 100, Height: 200, Open: func() (Image, error) {
		return Image{}, errors.New("mất ảnh")
	}}
	sb := NewBuilder().(StreamingBuilder)
	if err := sb.BuildTo(&bytes.Buffer{}, in); err == nil {
		t.Fatal("muốn lỗi khi Open hỏng")
	}
}
