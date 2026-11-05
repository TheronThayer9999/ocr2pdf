// Package src holds the HTTP handlers of the ocr2pdf demo API.
package src

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TheronThayer9999/ocr2pdf"
)

// MaxUpload caps the size of a single upload.
const MaxUpload = 32 << 20 // 32 MB

// supported maps the file extensions the imaging loaders can decode.
var supported = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".pdf": true,
}

// API holds the shared configuration of the handlers.
type API struct {
	timeout time.Duration
}

// New creates the API.
func New() *API {
	return &API{timeout: 5 * time.Minute}
}

// Register mounts every route on the router.
func (a *API) Register(r gin.IRouter) {
	r.GET("/health", a.health)
	r.GET("/api/languages", a.languages)
	r.POST("/api/convert", a.convert)
}

// health is used by load balancers and docker healthchecks.
func (a *API) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// languages lists the Tesseract traineddata installed on the server.
func (a *API) languages(c *gin.Context) {
	langs := ocr2pdf.Languages(c.Request.Context())
	if langs == nil {
		langs = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"languages": langs})
}

// convert accepts a multipart upload (or a remote URL) and streams back the searchable PDF.
//
//	curl -F "file=@scan.pdf" -F "lang=vie" http://localhost:8080/api/convert -o out.pdf
//	curl -F "url=https://example.com/scan.pdf" http://localhost:8080/api/convert -o out.pdf
func (a *API) convert(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUpload)

	file, fileErr := c.FormFile("file")
	rawURL := strings.TrimSpace(c.PostForm("url"))
	if fileErr != nil && rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": `missing multipart field "file" or "url"`})
		return
	}

	opts, err := optionsFrom(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conv, err := ocr2pdf.New(opts...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), a.timeout)
	defer cancel()

	inputPath, name, cleanup, err := a.input(file, fileErr, rawURL)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnsupportedType) {
			status = http.StatusUnsupportedMediaType
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	defer cleanup()

	data, res, err := conv.Render(ctx, inputPath)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Header("X-Pdf-Language", res.Language)
	c.Header("X-Pdf-Pages", strconv.Itoa(res.Pages))
	c.Header("X-Pdf-Words", strconv.Itoa(res.Words))
	c.Data(http.StatusOK, "application/pdf", data)
}

// errUnsupportedType marks uploads whose extension the loaders cannot decode.
var errUnsupportedType = errors.New("unsupported file type")

// input resolves the request into a path the loaders understand: either a temp
// file holding the upload, or the remote URL itself (the loader downloads it).
func (a *API) input(file *multipart.FileHeader, fileErr error, rawURL string) (string, string, func(), error) {
	if fileErr != nil {
		name := ocr2pdf.DefaultOutputPath(rawURL)
		return rawURL, name, func() {}, nil
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !supported[ext] {
		return "", "", func() {}, fmt.Errorf(
			"%w %q (allowed: png, jpg, gif, pdf)", errUnsupportedType, ext)
	}

	name := strings.TrimSuffix(filepath.Base(file.Filename), filepath.Ext(file.Filename)) + "-searchable.pdf"
	path, cleanup, err := saveTemp(file)
	if err != nil {
		return "", "", func() {}, err
	}
	return path, name, cleanup, nil
}

// optionsFrom reads the optional form fields and turns them into converter options.
func optionsFrom(c *gin.Context) ([]ocr2pdf.Option, error) {
	var opts []ocr2pdf.Option

	if lang := strings.TrimSpace(c.PostForm("lang")); lang != "" && !strings.EqualFold(lang, "auto") {
		opts = append(opts, ocr2pdf.WithLanguage(lang))
	}
	if raw := strings.TrimSpace(c.PostForm("psm")); raw != "" {
		psm, err := strconv.Atoi(raw)
		if err != nil || psm < 0 || psm > 13 {
			return nil, fmt.Errorf("psm must be an integer between 0 and 13")
		}
		opts = append(opts, ocr2pdf.WithPSM(psm))
	}
	if raw := strings.TrimSpace(c.PostForm("dpi")); raw != "" {
		dpi, err := strconv.ParseFloat(raw, 64)
		if err != nil || dpi < 0 {
			return nil, fmt.Errorf("dpi must be a positive number")
		}
		opts = append(opts, ocr2pdf.WithDPI(dpi))
	}
	if raw := strings.TrimSpace(c.PostForm("conf")); raw != "" {
		conf, err := strconv.ParseFloat(raw, 64)
		if err != nil || conf < 0 || conf > 100 {
			return nil, fmt.Errorf("conf must be a number between 0 and 100")
		}
		opts = append(opts, ocr2pdf.WithMinConfidence(conf))
	}

	return opts, nil
}

// saveTemp copies the upload to a temp file with the original extension and
// returns the path plus a cleanup function.
func saveTemp(file *multipart.FileHeader) (string, func(), error) {
	src, err := file.Open()
	if err != nil {
		return "", func() {}, fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	f, err := os.CreateTemp("", "ocr2pdf-*"+strings.ToLower(filepath.Ext(file.Filename)))
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp file: %w", err)
	}

	cleanup := func() {
		f.Close()
		os.Remove(f.Name())
	}

	if _, err := io.Copy(f, src); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("close temp file: %w", err)
	}

	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
