package ocr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Finder xác định vị trí binary tesseract.
type Finder interface {
	Find() (string, error)
}

// PathFinder tìm tesseract theo thứ tự: đường dẫn chỉ định -> biến môi trường -> PATH -> vị trí cài mặc định.
type PathFinder struct {
	Explicit string // đường dẫn người dùng truyền vào (có thể rỗng)
}

// NewPathFinder tạo finder với đường dẫn tuỳ chọn.
func NewPathFinder(explicit string) *PathFinder {
	return &PathFinder{Explicit: explicit}
}

// Find trả về đường dẫn binary tesseract đầu tiên tìm thấy.
func (f *PathFinder) Find() (string, error) {
	if f.Explicit != "" {
		if _, err := os.Stat(f.Explicit); err != nil {
			return "", fmt.Errorf("không tìm thấy tesseract tại %q: %w", f.Explicit, err)
		}
		return f.Explicit, nil
	}
	if p := os.Getenv("TESSERACT_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("tesseract"); err == nil {
		return p, nil
	}
	for _, c := range defaultCandidates() {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("không tìm thấy tesseract, hãy cài đặt hoặc truyền đường dẫn binary")
}

func defaultCandidates() []string {
	var out []string
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		out = append(out, filepath.Join(pf, "Tesseract-OCR", "tesseract.exe"))
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		out = append(out, filepath.Join(la, "Programs", "Tesseract-OCR", "tesseract.exe"))
	}
	return append(out, "/usr/bin/tesseract", "/usr/local/bin/tesseract")
}
