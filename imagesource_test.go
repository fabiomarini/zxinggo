package zxinggo_test

import (
	"image"
	"image/color"
	"testing"

	zxinggo "github.com/fabiomarini/zxinggo"
)

// genericImage hides the concrete image type so NewImageLuminanceSource takes
// its generic At()-based fallback path, serving as the reference for the
// Pix/stride fast paths.
type genericImage struct{ image.Image }

func TestLuminanceFastPathsMatchGeneric(t *testing.T) {
	for _, tc := range []struct {
		name string
		img  image.Image
	}{
		{"Gray", mkGrayForLuma()},
		{"RGBA", mkRGBAForLuma()},
		{"NRGBA", mkNRGBAForLuma()},
		{"Gray16", mkGray16ForLuma()},
		{"RGBA64", mkRGBA64ForLuma()},
		{"NRGBA64", mkNRGBA64ForLuma()},
		{"Paletted", mkPalettedForLuma()},
		{"YCbCr444", mkYCbCrForLuma(image.YCbCrSubsampleRatio444)},
		{"YCbCr422", mkYCbCrForLuma(image.YCbCrSubsampleRatio422)},
		{"YCbCr420", mkYCbCrForLuma(image.YCbCrSubsampleRatio420)},
		{"YCbCr411", mkYCbCrForLuma(image.YCbCrSubsampleRatio411)},
	} {
		fast := zxinggo.NewImageLuminanceSource(tc.img).Matrix()
		ref := zxinggo.NewImageLuminanceSource(genericImage{tc.img}).Matrix()
		if len(fast) != len(ref) {
			t.Fatalf("%s: length %d != %d", tc.name, len(fast), len(ref))
		}
		for i := range fast {
			if fast[i] != ref[i] {
				t.Fatalf("%s: pixel %d: fast=%d generic=%d", tc.name, i, fast[i], ref[i])
			}
		}
	}
}

func mkGrayForLuma() image.Image {
	m := image.NewGray(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			m.SetGray(x, y, color.Gray{Y: uint8(x * 30)})
		}
	}
	return m
}

func mkRGBAForLuma() image.Image {
	m := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			m.Set(x, y, color.RGBA{uint8(x * 30), uint8(y * 30), uint8((x + y) * 15), 255})
		}
	}
	m.Set(0, 0, color.RGBA{0, 0, 0, 0}) // fully transparent -> white
	return m
}

func mkNRGBAForLuma() image.Image {
	m := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			m.Set(x, y, color.NRGBA{uint8(x * 30), uint8(y * 30), uint8((x + y) * 15), byte(x * 30)})
		}
	}
	return m
}

func mkGray16ForLuma() image.Image {
	m := image.NewGray16(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			m.SetGray16(x, y, color.Gray16{Y: uint16(x * 800)})
		}
	}
	return m
}

func mkRGBA64ForLuma() image.Image {
	m := image.NewRGBA64(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			m.Set(x, y, color.RGBA64{uint16(x * 8000), uint16(y * 8000), uint16(x * 4000), 65535})
		}
	}
	return m
}

func mkNRGBA64ForLuma() image.Image {
	m := image.NewNRGBA64(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			m.Set(x, y, color.NRGBA64{uint16(x * 8000), uint16(y * 8000), uint16(x * 4000), uint16(x * 8000)})
		}
	}
	return m
}

func mkPalettedForLuma() image.Image {
	pal := color.Palette{
		color.NRGBA{0, 0, 0, 0}, // fully transparent -> white
		color.Black,
		color.White,
		color.NRGBA{200, 30, 90, 255},
		color.Gray{Y: 123},
		color.NRGBA{10, 200, 40, 128}, // partial alpha premultiplication
	}
	m := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	for y := range 8 {
		for x := range 8 {
			m.SetColorIndex(x, y, byte((x*3+y)%len(pal)))
		}
	}
	return m
}

func mkYCbCrForLuma(ratio image.YCbCrSubsampleRatio) image.Image {
	w, h := 9, 7 // odd sizes exercise subsample rounding
	m := image.NewYCbCr(image.Rect(0, 0, w, h), ratio)
	for i := range m.Y {
		m.Y[i] = uint8(i * 7)
	}
	for i := range m.Cb {
		m.Cb[i] = uint8(30 + i*11)
	}
	for i := range m.Cr {
		m.Cr[i] = uint8(250 - i*13)
	}
	return m
}
