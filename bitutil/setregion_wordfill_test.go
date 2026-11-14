package bitutil

import (
	"math/rand"
	"testing"
)

func TestSetRegionWordFillMatchesPerBit(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 200; trial++ {
		w, h := rng.Intn(130)+1, rng.Intn(40)+1
		got := NewBitMatrixWithSize(w, h)
		want := NewBitMatrixWithSize(w, h)
		for op := 0; op < 20; op++ {
			lw, lh := rng.Intn(w)+1, rng.Intn(h)+1
			left, top := rng.Intn(w-lw+1), rng.Intn(h-lh+1)
			if err := got.SetRegion(left, top, lw, lh); err != nil {
				t.Fatal(err)
			}
			for y := top; y < top+lh; y++ {
				for x := left; x < left+lw; x++ {
					want.Set(x, y)
				}
			}
		}
		if got.String() != want.String() {
			t.Fatalf("trial %d: word-fill differs from per-bit at %dx%d", trial, w, h)
		}
	}
}
