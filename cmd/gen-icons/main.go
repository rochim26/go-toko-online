// gen-icons creates flat PNG icon assets for the PWA + favicon.
// Run locally: go run ./cmd/gen-icons static/img
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

var (
	bg = color.RGBA{0x0f, 0x17, 0x2a, 0xff} // navy
	fg = color.RGBA{0xff, 0xff, 0xff, 0xff}
)

func main() {
	out := "static/img"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, sz := range []int{192, 512, 180, 32} {
		path := filepath.Join(out, fmt.Sprintf("icon-%d.png", sz))
		if sz == 180 {
			path = filepath.Join(out, "apple-touch-icon.png")
		}
		if sz == 32 {
			path = filepath.Join(out, "favicon-32.png")
		}
		if err := writeIcon(path, sz, "M"); err != nil {
			log.Fatal(err)
		}
		fmt.Println("wrote", path)
	}
	// favicon.ico - reuse 32x32 PNG renamed (browsers accept PNG inside .ico)
	src, _ := os.ReadFile(filepath.Join(out, "favicon-32.png"))
	if err := os.WriteFile(filepath.Join(out, "favicon.ico"), src, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote", filepath.Join(out, "favicon.ico"))
}

func writeIcon(path string, size int, label string) error {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// Fill with bg
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	// Draw a centered "M" using upscaled basic font (since basicfont is fixed 7x13)
	// Render to a small canvas then upscale.
	small := image.NewRGBA(image.Rect(0, 0, 13, 13))
	draw.Draw(small, small.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	d := &font.Drawer{
		Dst:  small,
		Src:  &image.Uniform{fg},
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(3), Y: fixed.I(11)},
	}
	d.DrawString(label)
	// Scale up to size
	r := image.Rect(size/4, size/4, size*3/4, size*3/4)
	draw.NearestNeighbor.Scale(img, r, small, small.Bounds(), draw.Over, nil)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
