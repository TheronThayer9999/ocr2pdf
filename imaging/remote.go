package imaging

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// DefaultMaxDownload caps how many bytes a remote input may have.
const DefaultMaxDownload = 128 << 20 // 128 MB

// RemoteLoader downloads http(s) inputs to a temp file and delegates to Local,
// which expects a real path. Anything that is not a URL is passed straight through.
type RemoteLoader struct {
	Local    Loader
	Client   *http.Client
	MaxBytes int64
}

// NewRemoteLoader wraps a local loader with URL support.
func NewRemoteLoader(local Loader) *RemoteLoader {
	return &RemoteLoader{
		Local:    local,
		Client:   &http.Client{Timeout: 2 * time.Minute},
		MaxBytes: DefaultMaxDownload,
	}
}

// IsRemote reports whether inputPath is an http(s) URL.
func IsRemote(inputPath string) bool {
	lower := strings.ToLower(inputPath)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// Load downloads the URL when needed, then loads it like a local file.
func (l *RemoteLoader) Load(ctx context.Context, inputPath string) ([]Page, error) {
	if !IsRemote(inputPath) {
		return l.Local.Load(ctx, inputPath)
	}

	path, cleanup, err := l.download(ctx, inputPath)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	return l.Local.Load(ctx, path)
}

// download streams the URL into a temp file named after the remote file's extension.
func (l *RemoteLoader) download(ctx context.Context, rawURL string) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", func() {}, fmt.Errorf("tải %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "ocr2pdf")

	resp, err := l.Client.Do(req)
	if err != nil {
		return "", func() {}, fmt.Errorf("tải %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", func() {}, fmt.Errorf("tải %s: máy chủ trả về %s", rawURL, resp.Status)
	}

	ext := extensionFor(resp, rawURL)
	f, err := os.CreateTemp("", "ocr2pdf-download-*"+ext)
	if err != nil {
		return "", func() {}, fmt.Errorf("tạo file tạm: %w", err)
	}

	cleanup := func() {
		f.Close()
		os.Remove(f.Name())
	}

	max := l.MaxBytes
	if max <= 0 {
		max = DefaultMaxDownload
	}
	written, err := io.Copy(f, io.LimitReader(resp.Body, max+1))
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("tải %s: %w", rawURL, err)
	}
	if written > max {
		cleanup()
		return "", func() {}, fmt.Errorf("tải %s: file lớn hơn giới hạn %d byte", rawURL, max)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("ghi file tạm: %w", err)
	}

	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

// extensionFor prefers the URL path, then the Content-Type header.
func extensionFor(resp *http.Response, rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		switch ext := strings.ToLower(path.Ext(u.Path)); ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".pdf":
			return ext
		}
	}

	switch strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "application/pdf":
		return ".pdf"
	}

	// Unknown: let AutoLoader sniff the content.
	return ".bin"
}
