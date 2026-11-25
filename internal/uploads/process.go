// Package uploads stores re-encoded images and attaches them to posts.
package uploads

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	_ "image/gif" // decoder
	"image/jpeg"
	"image/png"
	"io"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // decoder
)

const (
	maxPixels = 40_000_000
	maxSide   = 2560
	jpegQ     = 85
)

var (
	ErrNotImage      = errors.New("not a supported image")
	ErrTooManyPixels = errors.New("image has too many pixels")
)

// Process validates an image, downsizes it to ≤ 2560 px and re-encodes it,
// which drops all metadata. PNG/GIF become PNG; JPEG/WebP become JPEG.
func Process(r io.Reader) (data []byte, ext string, w, h int, err error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, "", 0, 0, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", 0, 0, ErrNotImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, "", 0, 0, ErrNotImage
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, "", 0, 0, ErrTooManyPixels
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", 0, 0, ErrNotImage
	}
	w, h = cfg.Width, cfg.Height
	img := src
	if w > maxSide || h > maxSide {
		if w >= h {
			h, w = h*maxSide/w, maxSide
		} else {
			w, h = w*maxSide/h, maxSide
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		img = dst
	}
	var out bytes.Buffer
	switch format {
	case "png", "gif":
		ext = "png"
		err = png.Encode(&out, img)
	default:
		ext = "jpg"
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: jpegQ})
	}
	if err != nil {
		return nil, "", 0, 0, err
	}
	return out.Bytes(), ext, w, h, nil
}
