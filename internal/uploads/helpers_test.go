package uploads_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func solid(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	return img
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, solid(w, h), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// jpegWithExif splices an APP1 Exif segment right after the SOI marker.
func jpegWithExif(t *testing.T) []byte {
	src := jpegBytes(t, 40, 30)
	payload := append([]byte("Exif\x00\x00"), bytes.Repeat([]byte("GPS-SECRET"), 8)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, src[:2]...)
	out = append(out, seg...)
	return append(out, src[2:]...)
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, solid(w, h)); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func gifBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.Encode(&b, solid(10, 10), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// hugePNG is a 1×1 PNG whose IHDR claims w×h (CRC fixed), a few hundred bytes on disk.
func hugePNG(t *testing.T, w, h uint32) []byte {
	b := pngBytes(t, 1, 1)
	// signature(8) + length(4) + "IHDR"(4) → width at 16, height at 20, CRC after 13 data bytes at 29
	binary.BigEndian.PutUint32(b[16:], w)
	binary.BigEndian.PutUint32(b[20:], h)
	binary.BigEndian.PutUint32(b[29:], crc32.ChecksumIEEE(b[12:29]))
	return b
}
