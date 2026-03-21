// gen_pwa_icons writes PWA manifest assets: square PNG icons (144/192/512) and optional screenshots (wide + mobile).
// Run from repo root: go run ./cmd/tools/gen_pwa_icons
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func lerpColor(c1, c2 color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(lerp(float64(c1.R), float64(c2.R), t)),
		G: uint8(lerp(float64(c1.G), float64(c2.G), t)),
		B: uint8(lerp(float64(c1.B), float64(c2.B), t)),
		A: 255,
	}
}

// Brand gradient corners (matches favicon.svg spirit: cyan → deep blue).
var cTL = color.NRGBA{14, 165, 233, 255}  // #0EA5E9
var cBR = color.NRGBA{29, 78, 216, 255}   // #1D4ED8

func fillGradient(img *image.NRGBA) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tx := float64(x) / float64(w-1)
			ty := float64(y) / float64(h-1)
			t := (tx + ty) * 0.5
			img.SetNRGBA(x, y, lerpColor(cTL, cBR, t))
		}
	}
}

func drawSun(img *image.NRGBA, cx, cy, r int, sun1, sun2 color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dx, dy := x-cx, y-cy
			if d := dx*dx + dy*dy; d <= r*r {
				t := float64(d) / float64(r*r)
				img.SetNRGBA(x, y, lerpColor(sun1, sun2, t))
			}
		}
	}
}

func drawCloud(img *image.NRGBA) {
	w := img.Bounds().Dx()
	// white cloud blob (three overlapping circles)
	type circ struct{ cx, cy, r int }
	circs := []circ{
		{int(0.42 * float64(w)), int(0.58 * float64(w)), int(0.14 * float64(w))},
		{int(0.52 * float64(w)), int(0.55 * float64(w)), int(0.16 * float64(w))},
		{int(0.62 * float64(w)), int(0.58 * float64(w)), int(0.13 * float64(w))},
	}
	cloud := color.NRGBA{255, 255, 255, 245}
	for _, c := range circs {
		r2 := c.r * c.r
		minx, maxx := c.cx-c.r, c.cx+c.r
		miny, maxy := c.cy-c.r, c.cy+c.r
		for y := miny; y <= maxy; y++ {
			for x := minx; x <= maxx; x++ {
				if x < 0 || y < 0 || x >= w || y >= w {
					continue
				}
				dx, dy := x-c.cx, y-c.cy
				if dx*dx+dy*dy <= r2 {
					img.SetNRGBA(x, y, cloud)
				}
			}
		}
	}
}

func drawRainStrokes(img *image.NRGBA) {
	w := img.Bounds().Dx()
	drop := color.NRGBA{96, 165, 250, 255}
	th := int(math.Max(2, float64(w)*0.012))
	for _, off := range []struct{ x, y, dx, dy float64 }{
		{0.48, 0.72, -0.01, 0.04},
		{0.54, 0.74, -0.01, 0.04},
		{0.60, 0.72, -0.01, 0.04},
	} {
		x0 := int(off.x * float64(w))
		y0 := int(off.y * float64(w))
		x1 := int((off.x + off.dx) * float64(w))
		y1 := int((off.y + off.dy) * float64(w))
		line(img, x0, y0, x1, y1, drop, th)
	}
}

func line(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA, thick int) {
	steps := int(math.Max(math.Abs(float64(x1-x0)), math.Abs(float64(y1-y0)))) + 1
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := int(lerp(float64(x0), float64(x1), t))
		y := int(lerp(float64(y0), float64(y1), t))
		for dy := -thick; dy <= thick; dy++ {
			for dx := -thick; dx <= thick; dx++ {
				if dx*dx+dy*dy > thick*thick {
					continue
				}
				xx, yy := x+dx, y+dy
				if xx >= 0 && yy >= 0 && xx < img.Bounds().Dx() && yy < img.Bounds().Dy() {
					img.SetNRGBA(xx, yy, c)
				}
			}
		}
	}
}

func buildIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	fillGradient(img)

	sun1 := color.NRGBA{252, 211, 77, 255} // amber
	sun2 := color.NRGBA{249, 115, 22, 255} // orange
	sr := size / 6
	scx := size * 3 / 10
	scy := size * 11 / 40
	drawSun(img, scx, scy, sr, sun1, sun2)

	drawCloud(img)
	drawRainStrokes(img)
	return img
}

func fillRect(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	b := img.Bounds()
	for y := y0; y <= y1 && y < b.Max.Y; y++ {
		if y < b.Min.Y {
			continue
		}
		for x := x0; x <= x1 && x < b.Max.X; x++ {
			if x < b.Min.X {
				continue
			}
			img.SetNRGBA(x, y, c)
		}
	}
}

// Promo images for manifest "screenshots" (no text; safe‑area UI mock).
func buildPromoWide(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := (float64(x)/float64(w) + float64(y)/float64(h)) * 0.5
			img.SetNRGBA(x, y, lerpColor(color.NRGBA{15, 23, 42, 255}, color.NRGBA{30, 58, 138, 255}, t))
		}
	}
	fillRect(img, 0, 0, w-1, h/14, color.NRGBA{15, 23, 42, 220})
	cw, ch := w*55/100, h*50/100
	cx0 := (w - cw) / 2
	cy0 := (h - ch) / 2
	fillRect(img, cx0, cy0, cx0+cw, cy0+ch, color.NRGBA{30, 41, 59, 240})
	// mini icon in card
	mi := buildIcon(min(120, cw/3))
	mb := mi.Bounds()
	sx := cx0 + (cw-mb.Dx())/2
	sy := cy0 + ch/5
	for y := 0; y < mb.Dy(); y++ {
		for x := 0; x < mb.Dx(); x++ {
			_, _, _, a := mi.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			img.SetNRGBA(sx+x, sy+y, mi.NRGBAAt(x, y))
		}
	}
	return img
}

func buildPromoMobile(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := (float64(x)/float64(w) + float64(y)/float64(h)) * 0.5
			img.SetNRGBA(x, y, lerpColor(color.NRGBA{15, 23, 42, 255}, color.NRGBA{29, 78, 216, 255}, t))
		}
	}
	fillRect(img, 0, 0, w-1, h/12, color.NRGBA{15, 23, 42, 230})
	cw, ch := w*88/100, h*38/100
	cx0 := (w - cw) / 2
	cy0 := h * 22 / 100
	fillRect(img, cx0, cy0, cx0+cw, cy0+ch, color.NRGBA{30, 41, 59, 245})
	mi := buildIcon(min(96, cw/3))
	mb := mi.Bounds()
	sx := cx0 + (cw-mb.Dx())/2
	sy := cy0 + ch/6
	for y := 0; y < mb.Dy(); y++ {
		for x := 0; x < mb.Dx(); x++ {
			img.SetNRGBA(sx+x, sy+y, mi.NRGBAAt(x, y))
		}
	}
	return img
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func main() {
	for _, size := range []int{144, 192, 512} {
		img := buildIcon(size)
		path := fmt.Sprintf("static/icons/icon-%d.png", size)
		if err := writePNG(path, img); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Println("wrote", path)
	}

	if err := os.MkdirAll("static/screenshots", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir screenshots: %v\n", err)
		os.Exit(1)
	}
	for _, s := range []struct {
		path string
		w, h int
		fn   func(int, int) *image.NRGBA
	}{
		{"static/screenshots/wide.png", 1280, 720, buildPromoWide},
		{"static/screenshots/mobile.png", 390, 844, buildPromoMobile},
	} {
		img := s.fn(s.w, s.h)
		if err := writePNG(s.path, img); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", s.path, err)
			os.Exit(1)
		}
		fmt.Println("wrote", s.path)
	}
}
