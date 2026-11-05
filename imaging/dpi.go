package imaging

import (
	"encoding/binary"
	"os"
)

// pngDPI đọc chunk pHYs của PNG. Trả 0 nếu không có.
func pngDPI(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 8 || string(data[1:4]) != "PNG" {
		return 0
	}
	for i := 8; i+8 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[i : i+4]))
		typ := string(data[i+4 : i+8])
		body := i + 8
		if body+length+4 > len(data) {
			return 0
		}
		if typ == "pHYs" && length >= 9 {
			ppuX := binary.BigEndian.Uint32(data[body : body+4])
			if data[body+8] == 1 && ppuX > 0 { // 1 = pixel/mét
				return float64(ppuX) * 0.0254
			}
			return 0
		}
		i = body + length + 4 // bỏ CRC
	}
	return 0
}

// jpegDPI đọc mật độ điểm ảnh trong APP0/JFIF của JPEG. Trả 0 nếu không có.
func jpegDPI(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 0
		}
		marker := data[i+1]
		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if length < 2 || i+2+length > len(data) {
			return 0
		}
		if marker == 0xE0 && length >= 14 { // APP0, JFIF
			units := data[i+11]
			xDensity := binary.BigEndian.Uint16(data[i+12 : i+14])
			if xDensity == 0 {
				return 0
			}
			switch units {
			case 1: // dpi
				return float64(xDensity)
			case 2: // dots/cm
				return float64(xDensity) * 2.54
			}
			return 0
		}
		if marker == 0xDA { // bắt đầu dữ liệu nén
			return 0
		}
		i += 2 + length
	}
	return 0
}

// detectDPI lấy DPI từ metadata của ảnh (PNG/JPEG), không có thì trả 0.
func detectDPI(path string) float64 {
	if d := pngDPI(path); d > 0 {
		return d
	}
	if d := jpegDPI(path); d > 0 {
		return d
	}
	return 0
}
