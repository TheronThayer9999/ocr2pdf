package pdf

import (
	"fmt"
	"io"
)

// pdfWriter ghi object PDF tuần tự ra io.Writer và tự dựng bảng xref. Nó KHÔNG
// giữ nội dung object trong RAM — đây là phần làm cho BuildTo có RAM O(1 trang).
type pdfWriter struct {
	w       io.Writer
	pos     int64
	offsets []int64
}

// header ghi magic + comment nhị phân để công cụ nhận ra file PDF.
func (p *pdfWriter) header() error {
	return p.write([]byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"))
}

// object ghi một object gián tiếp. Các object phải được ghi theo đúng thứ tự
// 1,2,3… vì bảng xref ở cuối đánh số theo thứ tự xuất hiện.
func (p *pdfWriter) object(num int, parts ...[]byte) error {
	if num != len(p.offsets)+1 {
		return fmt.Errorf("pdf: ghi object %d sai thứ tự (đang chờ %d)", num, len(p.offsets)+1)
	}
	p.offsets = append(p.offsets, p.pos)
	if err := p.writef("%d 0 obj\n", num); err != nil {
		return err
	}
	for _, part := range parts {
		if err := p.write(part); err != nil {
			return err
		}
	}
	return p.write([]byte("\nendobj\n"))
}

// stream ghi một object dạng stream; /Length phải là số byte của riêng data.
func (p *pdfWriter) stream(num int, dict string, data []byte) error {
	head := []byte(fmt.Sprintf("<< %s /Length %d >>\nstream\n", dict, len(data)))
	return p.object(num, head, data, []byte("\nendstream"))
}

// finish ghi bảng xref + trailer. root là object của /Catalog.
func (p *pdfWriter) finish(root int) error {
	xref := p.pos
	if err := p.writef("xref\n0 %d\n", len(p.offsets)+1); err != nil {
		return err
	}
	if err := p.write([]byte("0000000000 65535 f \n")); err != nil {
		return err
	}
	for _, off := range p.offsets {
		if err := p.writef("%010d 00000 n \n", off); err != nil {
			return err
		}
	}
	return p.writef("trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(p.offsets)+1, root, xref)
}

func (p *pdfWriter) writef(format string, args ...any) error {
	return p.write([]byte(fmt.Sprintf(format, args...)))
}

func (p *pdfWriter) write(b []byte) error {
	n, err := p.w.Write(b)
	p.pos += int64(n)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
