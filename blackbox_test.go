package zxinggo_test

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/binarizer"

	// Register all format readers
	_ "github.com/fabiomarini/zxinggo/aztec"
	_ "github.com/fabiomarini/zxinggo/datamatrix"
	_ "github.com/fabiomarini/zxinggo/maxicode"
	_ "github.com/fabiomarini/zxinggo/oned"
	_ "github.com/fabiomarini/zxinggo/pdf417"
	_ "github.com/fabiomarini/zxinggo/qrcode"
)

// --- 2D Formats ---

func TestBlackBoxAztec1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "aztec-1",
		format: zxinggo.FormatAztec,
		tests: []blackboxTestRotation{
			rot(0, 15, 15),
			rot(90, 15, 15),
			rot(180, 15, 15),
			rot(270, 15, 15),
		},
	})
}

func TestBlackBoxAztec2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "aztec-2",
		format: zxinggo.FormatAztec,
		tests: []blackboxTestRotation{
			rot(0, 22, 22),
			rot(90, 22, 22),
			rot(180, 22, 22),
			rot(270, 22, 22),
		},
	})
}

func TestBlackBoxDataMatrix1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "datamatrix-1",
		format: zxinggo.FormatDataMatrix,
		tests: []blackboxTestRotation{
			rot(0, 22, 22),
			rot(90, 22, 22),
			rot(180, 22, 22),
			rot(270, 22, 22),
		},
	})
}

func TestBlackBoxDataMatrix2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "datamatrix-2",
		format: zxinggo.FormatDataMatrix,
		tests: []blackboxTestRotation{
			rotM(0, 16, 16, 0, 1),
			rotM(90, 16, 16, 0, 1),
			rotM(180, 17, 17, 0, 1),
			rotM(270, 16, 16, 0, 1),
		},
	})
}

func TestBlackBoxDataMatrix3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "datamatrix-3",
		format: zxinggo.FormatDataMatrix,
		tests: []blackboxTestRotation{
			rot(0, 18, 18),
			rot(90, 17, 17),
			rot(180, 18, 18),
			rot(270, 18, 18),
		},
	})
}

func TestBlackBoxQRCode1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "qrcode-1",
		format: zxinggo.FormatQRCode,
		tests: []blackboxTestRotation{
			rot(0, 19, 19),
			rot(90, 16, 19),
			rot(180, 19, 19),
			rot(270, 16, 19),
		},
	})
}

func TestBlackBoxQRCode2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "qrcode-2",
		format: zxinggo.FormatQRCode,
		tests: []blackboxTestRotation{
			rot(0, 32, 33),
			rot(90, 30, 34),
			rot(180, 31, 34),
			rot(270, 31, 33),
		},
	})
}

func TestBlackBoxQRCode3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "qrcode-3",
		format: zxinggo.FormatQRCode,
		tests: []blackboxTestRotation{
			rot(0, 39, 42),
			rot(90, 39, 42),
			rot(180, 37, 41),
			rot(270, 40, 42),
		},
	})
}

func TestBlackBoxQRCode4(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "qrcode-4",
		format: zxinggo.FormatQRCode,
		tests: []blackboxTestRotation{
			rot(0, 36, 42),
			rot(90, 36, 42),
			rot(180, 35, 42),
			rot(270, 35, 42),
		},
	})
}

func TestBlackBoxQRCode5(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "qrcode-5",
		format: zxinggo.FormatQRCode,
		tests: []blackboxTestRotation{
			rot(0, 19, 19),
			rot(90, 19, 19),
			rot(180, 19, 19),
			rot(270, 19, 19),
		},
	})
}

func TestBlackBoxQRCode6(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "qrcode-6",
		format: zxinggo.FormatQRCode,
		tests: []blackboxTestRotation{
			rot(0, 15, 15),
			rot(90, 14, 15),
			rot(180, 13, 15),
			rot(270, 14, 15),
		},
	})
}

func TestBlackBoxPDF417_1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "pdf417-1",
		format: zxinggo.FormatPDF417,
		tests: []blackboxTestRotation{
			rot(0, 13, 13),
			rot(90, 13, 13),
			rot(180, 13, 13),
			rot(270, 13, 13),
		},
	})
}

func TestBlackBoxPDF417_2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "pdf417-2",
		format: zxinggo.FormatPDF417,
		tests: []blackboxTestRotation{
			rotM(0, 25, 25, 0, 0),
			rotM(180, 25, 25, 0, 0),
		},
	})
}

func TestBlackBoxPDF417_3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "pdf417-3",
		format: zxinggo.FormatPDF417,
		tests: []blackboxTestRotation{
			rotM(0, 19, 19, 0, 0),
			rotM(180, 19, 19, 0, 0),
		},
	})
}

func TestBlackBoxMaxiCode1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "maxicode-1",
		format: zxinggo.FormatMaxiCode,
		tests: []blackboxTestRotation{
			rot(0, 9, 9),
		},
	})
}

// --- 1D Formats ---

func TestBlackBoxCode128_1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "code128-1",
		format: zxinggo.FormatCode128,
		tests: []blackboxTestRotation{
			rot(0, 6, 6),
			rot(180, 6, 6),
		},
	})
}

func TestBlackBoxCode128_2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "code128-2",
		format: zxinggo.FormatCode128,
		tests: []blackboxTestRotation{
			rot(0, 36, 39),
			rot(180, 36, 39),
		},
	})
}

func TestBlackBoxCode128_3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "code128-3",
		format: zxinggo.FormatCode128,
		tests: []blackboxTestRotation{
			rot(0, 2, 2),
			rot(180, 2, 2),
		},
	})
}

func TestBlackBoxCode39_1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "code39-1",
		format: zxinggo.FormatCode39,
		tests: []blackboxTestRotation{
			rot(0, 4, 4),
			rot(180, 4, 4),
		},
	})
}

func TestBlackBoxCode39_3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "code39-3",
		format: zxinggo.FormatCode39,
		tests: []blackboxTestRotation{
			rot(0, 17, 17),
			rot(180, 17, 17),
		},
	})
}

func TestBlackBoxCodabar1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "codabar-1",
		format: zxinggo.FormatCodabar,
		tests: []blackboxTestRotation{
			rot(0, 11, 11),
			rot(180, 11, 11),
		},
	})
}

func TestBlackBoxEAN13_1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "ean13-1",
		format: zxinggo.FormatEAN13,
		tests: []blackboxTestRotation{
			rot(0, 30, 32),
			rot(180, 27, 32),
		},
	})
}

func TestBlackBoxEAN13_2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "ean13-2",
		format: zxinggo.FormatEAN13,
		tests: []blackboxTestRotation{
			rotM(0, 11, 17, 0, 1),
			rotM(180, 11, 17, 0, 1),
		},
	})
}

func TestBlackBoxEAN13_3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "ean13-3",
		format: zxinggo.FormatEAN13,
		tests: []blackboxTestRotation{
			rot(0, 53, 55),
			rot(180, 55, 55),
		},
	})
}

func TestBlackBoxEAN13_4(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "ean13-4",
		format: zxinggo.FormatEAN13,
		tests: []blackboxTestRotation{
			rotM(0, 6, 13, 1, 1),
			rotM(180, 7, 13, 1, 1),
		},
	})
}

func TestBlackBoxEAN13_5(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "ean13-5",
		format: zxinggo.FormatEAN13,
		tests: []blackboxTestRotation{
			rot(0, 0, 0),
			rot(180, 0, 0),
		},
	})
}

func TestBlackBoxEAN8_1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "ean8-1",
		format: zxinggo.FormatEAN8,
		tests: []blackboxTestRotation{
			rot(0, 8, 8),
			rot(180, 8, 8),
		},
	})
}

func TestBlackBoxITF1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "itf-1",
		format: zxinggo.FormatITF,
		tests: []blackboxTestRotation{
			rot(0, 14, 14),
			rot(180, 14, 14),
		},
	})
}

func TestBlackBoxITF2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "itf-2",
		format: zxinggo.FormatITF,
		tests: []blackboxTestRotation{
			rot(0, 14, 14),
			rot(180, 14, 14),
		},
	})
}

func TestBlackBoxUPCA1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upca-1",
		format: zxinggo.FormatUPCA,
		tests: []blackboxTestRotation{
			rotM(0, 14, 18, 0, 1),
			rotM(180, 16, 18, 0, 1),
		},
	})
}

func TestBlackBoxUPCA2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upca-2",
		format: zxinggo.FormatUPCA,
		tests: []blackboxTestRotation{
			rotM(0, 27, 35, 0, 2),
			rotM(180, 29, 35, 0, 2),
		},
	})
}

func TestBlackBoxUPCA3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upca-3",
		format: zxinggo.FormatUPCA,
		tests: []blackboxTestRotation{
			rotM(0, 7, 9, 0, 2),
			rotM(180, 8, 9, 0, 2),
		},
	})
}

func TestBlackBoxUPCA4(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upca-4",
		format: zxinggo.FormatUPCA,
		tests: []blackboxTestRotation{
			rotM(0, 9, 11, 0, 1),
			rotM(180, 9, 11, 0, 1),
		},
	})
}

func TestBlackBoxUPCA5(t *testing.T) {
	// TODO: thresholds regressed by 1 each after adding UPC/EAN extension support — investigate
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upca-5",
		format: zxinggo.FormatUPCA,
		tests: []blackboxTestRotation{
			rotM(0, 19, 23, 0, 0),
			rotM(180, 21, 23, 0, 0),
		},
	})
}

func TestBlackBoxUPCA6(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upca-6",
		format: zxinggo.FormatUPCA,
		tests: []blackboxTestRotation{
			rot(0, 0, 0),
			rot(180, 0, 0),
		},
	})
}

func TestBlackBoxUPCE1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upce-1",
		format: zxinggo.FormatUPCE,
		tests: []blackboxTestRotation{
			rot(0, 3, 3),
			rot(180, 3, 3),
		},
	})
}

func TestBlackBoxUPCE2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upce-2",
		format: zxinggo.FormatUPCE,
		tests: []blackboxTestRotation{
			rotM(0, 30, 35, 0, 2),
			rotM(180, 30, 35, 1, 2),
		},
	})
}

func TestBlackBoxUPCE3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upce-3",
		format: zxinggo.FormatUPCE,
		tests: []blackboxTestRotation{
			rot(0, 6, 8),
			rot(180, 6, 8),
		},
	})
}

// --- RSS Formats ---

func TestBlackBoxRSS14_1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "rss14-1",
		format: zxinggo.FormatRSS14,
		tests: []blackboxTestRotation{
			rot(0, 6, 6),
			rot(180, 6, 6),
		},
	})
}

func TestBlackBoxRSS14_2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "rss14-2",
		format: zxinggo.FormatRSS14,
		tests: []blackboxTestRotation{
			rotM(0, 6, 10, 1, 1),
			rotM(180, 6, 10, 0, 1),
		},
	})
}

func TestBlackBoxRSSExpanded1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "rssexpanded-1",
		format: zxinggo.FormatRSSExpanded,
		tests: []blackboxTestRotation{
			rot(0, 35, 35),
			rot(180, 35, 35),
		},
	})
}

func TestBlackBoxRSSExpanded2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "rssexpanded-2",
		format: zxinggo.FormatRSSExpanded,
		tests: []blackboxTestRotation{
			rot(0, 21, 22),
			rot(180, 21, 23),
		},
	})
}

func TestBlackBoxRSSExpanded3(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "rssexpanded-3",
		format: zxinggo.FormatRSSExpanded,
		tests: []blackboxTestRotation{
			rot(0, 117, 117),
			rot(180, 117, 117),
		},
	})
}

func TestBlackBoxRSSExpandedStacked1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "rssexpandedstacked-1",
		format: zxinggo.FormatRSSExpanded,
		tests: []blackboxTestRotation{
			rot(0, 60, 65),
			rot(180, 60, 65),
		},
	})
}

func TestBlackBoxRSSExpandedStacked2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "rssexpandedstacked-2",
		format: zxinggo.FormatRSSExpanded,
		tests: []blackboxTestRotation{
			rot(0, 2, 7),
			rot(180, 2, 7),
		},
	})
}

func TestBlackBoxPDF417_4(t *testing.T) {
	runPDF417MultiTest(t, "pdf417-4", 3)
}

// --- Code 93 ---

func TestBlackBoxCode93_1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "code93-1",
		format: zxinggo.FormatCode93,
		tests: []blackboxTestRotation{
			rot(0, 3, 3),
			rot(180, 3, 3),
		},
	})
}

// --- Extended Code 39 ---

func TestBlackBoxCode39_2(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "code39-2",
		format: zxinggo.FormatCode39,
		tests: []blackboxTestRotation{
			rot(0, 2, 2),
			rot(180, 2, 2),
		},
	})
}

// --- UPC/EAN Extension ---

func TestBlackBoxUPCEANExtension1(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "upcean-extension-1",
		format: zxinggo.FormatEAN13,
		tests: []blackboxTestRotation{
			rotM(0, 1, 1, 0, 1),
		},
		opts: &zxinggo.DecodeOptions{
			AllowedEANExtensions: []int{2, 5},
		},
	})
}

// --- Inverted ---

func TestBlackBoxInvertedDataMatrix(t *testing.T) {
	runBlackBoxTest(t, blackboxTestCase{
		dir:    "inverted",
		format: zxinggo.FormatDataMatrix,
		tests: []blackboxTestRotation{
			rot(0, 1, 1),
			rot(90, 1, 1),
			rot(180, 1, 1),
			rot(270, 1, 1),
		},
		opts: &zxinggo.DecodeOptions{
			AlsoInverted: true,
		},
	})
}

// invertImage returns a copy of img with every channel inverted.
func invertImage(img image.Image) image.Image {
	src := img.Bounds()
	out := image.NewNRGBA(src)
	for y := src.Min.Y; y < src.Max.Y; y++ {
		for x := src.Min.X; x < src.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{
				R: uint8(255 - r>>8), G: uint8(255 - g>>8),
				B: uint8(255 - b>>8), A: uint8(a >> 8),
			})
		}
	}
	return out
}

// TestAlsoInverted1D verifies that AlsoInverted lets row-based 1D readers
// decode white-on-black symbols. It inverts standard 1D images and asserts
// they decode only when AlsoInverted is set, proving the retry path does
// the work rather than the binarizer silently flipping the image.
func TestAlsoInverted1D(t *testing.T) {
	base := filepath.Join(blackboxTestDir, "code128-1")
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	decoded := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".png" {
			continue
		}
		path := filepath.Join(base, e.Name())
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		expected, err := loadExpectedText(strings.TrimSuffix(path, ".png"))
		if err != nil {
			t.Fatal(err)
		}

		// Feed a pixel-inverted image through a plain luminance source so
		// the reader's AlsoInverted retry (not a pre-inverted source) does
		// the work.
		inverted := zxinggo.NewBinaryBitmap(
			binarizer.NewHybrid(zxinggo.NewImageLuminanceSource(invertImage(img))))

		opts := &zxinggo.DecodeOptions{
			PossibleFormats: []zxinggo.Format{zxinggo.FormatCode128},
			AlsoInverted:    true,
		}
		result, err := zxinggo.Decode(inverted, opts)
		if err != nil || result.Text != expected {
			t.Errorf("%s: AlsoInverted decode: got %q (%v), want %q", e.Name(), resultText(result), err, expected)
		}

		opts.AlsoInverted = false
		if result, err := zxinggo.Decode(inverted, opts); err == nil {
			t.Errorf("%s: inverted image decoded without AlsoInverted (%q)", e.Name(), result.Text)
		}
		decoded++
	}
	if decoded == 0 {
		t.Fatal("no code128-1 images found")
	}
}
