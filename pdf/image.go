package pdf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// ImageEncoder biến Image thành dữ liệu nén và phần dictionary còn lại
// (Width/Height/Subtype do builder tự thêm).
type ImageEncoder interface {
	Encode(img Image) (data []byte, dict string, err error)
}

// DefaultEncoder chèn thẳng JPEG (DCTDecode); các ảnh khác được nén
// bằng zlib kèm filter kiểu PNG (Predictor 15) để giữ file nhỏ.
type DefaultEncoder struct{}

// Encode cài đặt ImageEncoder.
func (DefaultEncoder) Encode(img Image) ([]byte, string, error) {
	if img.Encoding == EncodingJPEG && len(img.Encoded) > 0 {
		space := "/DeviceRGB"
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(img.Encoded)); err == nil {
			if cfg.ColorModel == color.GrayModel {
				space = "/DeviceGray"
			}
		}
		return img.Encoded, "/ColorSpace " + space + " /BitsPerComponent 8 /Filter /DCTDecode", nil
	}
	return encodeFlate(img.Data)
}

// encodeFlate đưa ảnh về RGB nền trắng rồi nén bằng zlib + filter kiểu PNG.
func encodeFlate(img image.Image) ([]byte, string, error) {
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Over)

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, rgba); err == nil {
		if idat, w, ok := pngIDAT(pngBuf.Bytes()); ok && w == b.Dx() {
			dict := fmt.Sprintf(
				"/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /DecodeParms << /Predictor 15 /Colors 3 /BitsPerComponent 8 /Columns %d >>",
				w)
			return idat, dict, nil
		}
	}

	// Dự phòng: RGB thô + zlib.
	raw := make([]byte, 0, b.Dx()*b.Dy()*3)
	for y := 0; y < b.Dy(); y++ {
		row := rgba.Pix[y*rgba.Stride : y*rgba.Stride+b.Dx()*4]
		for x := 0; x < b.Dx(); x++ {
			raw = append(raw, row[x*4], row[x*4+1], row[x*4+2])
		}
	}

	var buf bytes.Buffer
	zw, err := zlib.NewWriterLevel(&buf, zlib.BestSpeed)
	if err != nil {
		return nil, "", err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, "", err
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode", nil
}

// pngIDAT lấy luồng IDAT của PNG (chỉ nhận ảnh RGB 8-bit, không interlace).
func pngIDAT(data []byte) ([]byte, int, bool) {
	if len(data) < 8 || string(data[1:4]) != "PNG" {
		return nil, 0, false
	}
	var idat []byte
	width := 0
	for i := 8; i+8 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[i : i+4]))
		typ := string(data[i+4 : i+8])
		body := i + 8
		if body+length+4 > len(data) {
			return nil, 0, false
		}
		switch typ {
		case "IHDR":
			if length < 13 {
				return nil, 0, false
			}
			width = int(binary.BigEndian.Uint32(data[body : body+4]))
			bitDepth, colorType, interlace := data[body+8], data[body+9], data[body+12]
			if bitDepth != 8 || colorType != 2 || interlace != 0 {
				return nil, 0, false
			}
		case "IDAT":
			idat = append(idat, data[body:body+length]...)
		}
		i = body + length + 4
	}
	if len(idat) == 0 || width == 0 {
		return nil, 0, false
	}
	return idat, width, true
}
