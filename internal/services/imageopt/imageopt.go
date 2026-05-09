package imageopt

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	_ "image/gif"

	"golang.org/x/image/draw"
)

const MaxDim = 1600

// Optimize reads src image bytes, resizes longest dimension to MaxDim if larger,
// then encodes back as JPEG (q=82). Saves to dstPath. Returns the final extension.
func Optimize(src io.Reader, dstPath string) (string, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return "", err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// Not a recognized raster (e.g., svg) - just write as-is
		return writeRaw(data, dstPath)
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return "", errors.New("invalid image dimensions")
	}
	scale := 1.0
	if w > MaxDim || h > MaxDim {
		if w > h {
			scale = float64(MaxDim) / float64(w)
		} else {
			scale = float64(MaxDim) / float64(h)
		}
	}
	dst := img
	if scale < 1.0 {
		nw := int(float64(w) * scale)
		nh := int(float64(h) * scale)
		rect := image.Rect(0, 0, nw, nh)
		out := image.NewRGBA(rect)
		draw.CatmullRom.Scale(out, rect, img, bounds, draw.Over, nil)
		dst = out
	}
	// Force JPEG output for size; switch to PNG only when source has alpha and is small
	jpegPath := changeExt(dstPath, ".jpg")
	f, err := os.Create(jpegPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := jpeg.Encode(f, dst, &jpeg.Options{Quality: 82}); err != nil {
		return "", err
	}
	return ".jpg", nil
}

func writeRaw(data []byte, dstPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return filepath.Ext(dstPath), nil
}

func changeExt(path, ext string) string {
	old := filepath.Ext(path)
	if old == "" {
		return path + ext
	}
	return path[:len(path)-len(old)] + ext
}

// ensure png import used
var _ = png.Decode
