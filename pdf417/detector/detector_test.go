package detector_test

import (
	"testing"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
	_ "github.com/fabiomarini/zxinggo/pdf417" // register PDF417 for Encode
	"github.com/fabiomarini/zxinggo/pdf417/detector"
)

// drawGuardBlob extrudes a start-pattern guard plus a short black codeword
// block (which bounds the guard's trailing white so findGuardPattern matches)
// from (x, topRow) down for height rows.
func drawGuardBlob(t *testing.T, bm *bitutil.BitMatrix, x, topRow, height int) {
	t.Helper()
	runs := []int{8, 1, 1, 1, 1, 1, 1, 3}
	for dy := range height {
		cx := x
		black := true
		for _, run := range runs {
			if black {
				for i := range run {
					bm.Set(cx+i, topRow+dy)
				}
			}
			cx += run
			black = !black
		}
		for i := range 20 {
			bm.Set(cx+i, topRow+dy)
		}
	}
}

// TestDetectSkipsShortGuardCandidate guards against the detector committing to
// the first guard-pattern match it finds. A short (sub-minimum-height) spurious
// start-pattern blob sits above a real symbol; the detector must skip the blob
// and report the real symbol with a valid start vertex, not lock onto the blob.
// Regression: the original committed to the first match, failed the height
// check, and returned a start-less (or empty) result on noisy pages.
func TestDetectSkipsShortGuardCandidate(t *testing.T) {
	sym, err := zxinggo.Encode("REGRESSION-TEST-PAYLOAD", zxinggo.FormatPDF417, 400, 120, nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	const ox, oy = 20, 120
	canvas := bitutil.NewBitMatrixWithSize(sym.Width()+2*ox, sym.Height()+oy+20)
	for y := range sym.Height() {
		for x := range sym.Width() {
			if sym.Get(x, y) {
				canvas.Set(ox+x, oy+y)
			}
		}
	}
	// Spurious short start-pattern blob near the top, same x as the real guard.
	drawGuardBlob(t, canvas, ox, 0, 5)

	res, err := detector.Detect(canvas, false, true)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(res.Points) == 0 {
		t.Fatal("detector found no symbol")
	}
	start := res.Points[0][0]
	if start == nil {
		t.Fatal("detector locked onto the spurious blob: start vertex is nil")
	}
	if start.Y < oy {
		t.Fatalf("detector locked onto the spurious blob: start y=%v, want >= %d", start.Y, oy)
	}
}
