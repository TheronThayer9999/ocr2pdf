package imaging

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.White)
		}
	}

	var buf strings.Builder
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return []byte(buf.String())
}

func TestIsRemote(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"https://example.com/scan.pdf", true},
		{"HTTP://example.com/scan.png", true},
		{"data/scan200.pdf", false},
		{`C:\scans\page.png`, false},
		{"ftp://example.com/x.pdf", false},
	} {
		if got := IsRemote(tc.in); got != tc.want {
			t.Errorf("IsRemote(%q) = %v, muốn %v", tc.in, got, tc.want)
		}
	}
}

func TestRemoteLoaderDownloadsImage(t *testing.T) {
	data := testPNG(t, 40, 20)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	pages, err := NewAutoLoader().Load(context.Background(), server.URL+"/scan.png")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("số trang = %d, muốn 1", len(pages))
	}
	if got := pages[0].Image.Bounds().Dx(); got != 40 {
		t.Errorf("chiều rộng = %d, muốn 40", got)
	}
	if pages[0].DPI <= 0 {
		t.Errorf("DPI = %v, muốn > 0", pages[0].DPI)
	}
}

func TestRemoteLoaderUsesContentTypeWithoutExtension(t *testing.T) {
	data := testPNG(t, 30, 10)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png; charset=binary")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	pages, err := NewAutoLoader().Load(context.Background(), server.URL+"/download")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := pages[0].Image.Bounds().Dy(); got != 10 {
		t.Errorf("chiều cao = %d, muốn 10", got)
	}
}

func TestRemoteLoaderReportsHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := NewAutoLoader().Load(context.Background(), server.URL+"/missing.png")
	if err == nil {
		t.Fatal("muốn lỗi khi máy chủ trả 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("lỗi = %v, muốn có mã 404", err)
	}
}

func TestRemoteLoaderEnforcesSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNG(t, 200, 200))
	}))
	defer server.Close()

	loader := NewRemoteLoader(NewAutoLoader())
	loader.MaxBytes = 128

	_, err := loader.Load(context.Background(), server.URL+"/big.png")
	if err == nil || !strings.Contains(err.Error(), "giới hạn") {
		t.Fatalf("lỗi = %v, muốn lỗi vượt giới hạn", err)
	}
}
