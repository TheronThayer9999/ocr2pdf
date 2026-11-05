package pdf

import (
	"bytes"
	"fmt"
)

// Document quản lý danh sách object gián tiếp và bảng xref của một file PDF.
type Document struct {
	objects [][]byte
}

// NewDocument tạo document rỗng.
func NewDocument() *Document {
	return &Document{}
}

// Reserve giữ chỗ trước cho một object để có thể tham chiếu chéo.
func (d *Document) Reserve() int {
	d.objects = append(d.objects, nil)
	return len(d.objects)
}

// Set gán nội dung cho object đã reserve.
func (d *Document) Set(num int, body []byte) {
	d.objects[num-1] = body
}

// AddStream thêm một object dạng stream và trả về số hiệu object.
func (d *Document) AddStream(dict string, data []byte) int {
	num := d.Reserve()
	d.Set(num, d.Stream(dict, data))
	return num
}

// Stream tạo nội dung object dạng stream (không tự thêm vào document).
func (d *Document) Stream(dict string, data []byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<< %s /Length %d >>\nstream\n", dict, len(data))
	b.Write(data)
	b.WriteString("\nendstream")
	return b.Bytes()
}

// Write ráp các object thành file hoàn chỉnh; root là object của /Catalog.
func (d *Document) Write(root int) []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")

	offsets := make([]int, len(d.objects))
	for i, body := range d.objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(body)
		out.WriteString("\nendobj\n")
	}

	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(d.objects)+1)
	out.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(d.objects)+1, root, xref)
	return out.Bytes()
}

// num định dạng số thực cho nội dung PDF.
func num(v float64) string {
	return fmt.Sprintf("%.2f", v)
}
