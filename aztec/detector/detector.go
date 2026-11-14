// Package detector implements Aztec barcode detection in binary images.
// This is a faithful Go port of the zxing-cpp Aztec detector
// (core/src/aztec/AZDetector.cpp).
package detector

import (
	"math/bits"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/bmcursor"
	"github.com/fabiomarini/zxinggo/internal/concentric"
	"github.com/fabiomarini/zxinggo/internal/localgrid"
	"github.com/fabiomarini/zxinggo/internal/pattern"
	"github.com/fabiomarini/zxinggo/reedsolomon"
)

// DetectorResult encapsulates the result of detecting an Aztec barcode.
type DetectorResult struct {
	Bits            *bitutil.BitMatrix
	Points          []zxinggo.ResultPoint
	Compact         bool
	NbDataBlocks    int
	NbLayers        int
	ErrorsCorrected int
}

// getPatternRow run-length encodes the given row of the binarized image into
// a pattern row (zxing-cpp GetPatternRow(const BitMatrix&, r, pr, false)):
// the result starts with the white run in front of the first bar (0 if the
// row starts with a bar) and ends with a white run.
func getPatternRow(image *bitutil.BitMatrix, y int, buf []int) []int {
	w := image.Width()
	buf = buf[:0]
	if w == 0 {
		return buf
	}
	prev := image.Get(0, y)
	if prev {
		buf = append(buf, 0) // first value is number of white pixels, here 0
	}
	run := 1
	for x := 1; x < w; x++ {
		v := image.Get(x, y)
		if v == prev {
			run++
		} else {
			buf = append(buf, run)
			run = 1
			prev = v
		}
	}
	buf = append(buf, run)
	if prev {
		buf = append(buf, 0)
	}
	return buf
}

// isAztecCenterPattern finds min/max of all subsequent black/white pairs and
// checks that they are 'close together' (zxing-cpp IsAztecCenterPattern).
func isAztecCenterPattern(view pattern.View) bool {
	m := view.At(0) + view.At(1)
	M := m
	for i := 1; i < view.Size()-1; i++ {
		v := view.At(i) + view.At(i+1)
		if v < m {
			m = v
		} else if v > M {
			M = v
		}
	}
	return M <= m*4/3+1 &&
		view.At(-1) >= view.At(view.Size()/2)-2 &&
		view.At(view.Size()) >= view.At(view.Size()/2)-2
}

// findAztecCenterPattern is a specialized version of FindLeftGuard to find
// the '1,1,1,1,1,1,1' pattern of a compact Aztec center pattern (zxing-cpp
// FindAztecCenterPattern).
func findAztecCenterPattern(view pattern.View) pattern.View {
	const minSize = 8 // Aztec runes
	window := view.SubView(0, 7)
	for window.Index() < view.Index()+view.Size()-minSize {
		if isAztecCenterPattern(window) {
			return window
		}
		window.SkipPair()
	}
	return pattern.View{}
}

// checkSymmetricAztecCenterPattern verifies the symmetric 7-ring pattern
// around the cursor position along its direction (zxing-cpp
// CheckSymmetricAztecCenterPattern). It returns the pattern spread or 0.
func checkSymmetricAztecCenterPattern(cur *bmcursor.CursorI, rng int, updatePosition bool) int {
	rng *= 2 // tilted symbols may have a larger vertical than horizontal range

	curFwd := bmcursor.NewEdgeCounter(*cur)
	curBwd := bmcursor.NewEdgeCounter(cur.TurnedBack())

	centerFwd := curFwd.StepToNextEdge(rng / 7)
	if centerFwd == 0 {
		return 0
	}
	centerBwd := curBwd.StepToNextEdge(rng / 7)
	if centerBwd == 0 {
		return 0
	}
	center := centerFwd + centerBwd - 1 // -1 because the starting pixel is counted twice
	if center > rng/7 || center < rng/(4*7) {
		return 0
	}

	spread := center
	m, M := 0, 0
	for _, c := range [2]*bmcursor.EdgeCounter{&curFwd, &curBwd} {
		lastS := center
		for range 3 {
			s := c.StepToNextEdge(rng - spread)
			if s == 0 {
				return 0
			}
			v := s + lastS
			if m == 0 {
				m, M = v, v
			} else if v < m {
				m = v
			} else if v > M {
				M = v
			}
			if M > m*4/3+1 {
				return 0
			}
			spread += s
			lastS = s
		}
	}

	if updatePosition {
		cur.Step(centerFwd - centerBwd)
	}

	return spread
}

// locateAztecCenter checks the center pattern in 4 directions and returns
// the refined center (zxing-cpp LocateAztecCenter).
func locateAztecCenter(image *bitutil.BitMatrix, center bmcursor.PointF, spreadH int) (concentric.ConcentricPattern, bool) {
	cur := bmcursor.NewCursorI(image, bmcursor.PointIF(center), bmcursor.PointI{})
	minSpread, maxSpread := spreadH, 0
	for _, d := range [4]bmcursor.PointI{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 1, Y: -1}} {
		cur.SetDirection(d)
		spread := checkSymmetricAztecCenterPattern(&cur, spreadH, d.X == 0)
		if spread == 0 {
			return concentric.ConcentricPattern{}, false
		}
		if spread < minSpread {
			minSpread = spread
		}
		if spread > maxSpread {
			maxSpread = spread
		}
	}

	return concentric.ConcentricPattern{P: bmcursor.CenteredI(cur.P), Size: float64(maxSpread+minSpread) / 2}, true
}

// findBoundingBox finds the bounding box of the set bits, ignoring results
// smaller than minSize (zxing-cpp BitMatrix::findBoundingBox).
func findBoundingBox(image *bitutil.BitMatrix, minSize int) (left, top, width, height int, ok bool) {
	tl := image.TopLeftOnBit()
	br := image.BottomRightOnBit()
	if tl == nil || br == nil || br[1]-tl[1]+1 < minSize {
		return 0, 0, 0, 0, false
	}
	left, top, right, bottom := tl[0], tl[1], br[0], br[1]

	for y := top; y <= bottom; y++ {
		for x := 0; x < left; x++ {
			if image.Get(x, y) {
				left = x
				break
			}
		}
		for x := image.Width() - 1; x > right; x-- {
			if image.Get(x, y) {
				right = x
				break
			}
		}
	}

	width = right - left + 1
	height = bottom - top + 1
	return left, top, width, height, width >= minSize && height >= minSize
}

// findPureFinderPattern locates the finder pattern of a pure Aztec symbol
// via its bounding box (zxing-cpp FindPureFinderPattern).
func findPureFinderPattern(image *bitutil.BitMatrix) []concentric.ConcentricPattern {
	// The smallest possible Aztec symbol is an Aztec Rune which is 11x11.
	// The Aztec Runes 68 and 223 have none of their bits set on the bottom
	// row, so the smallest dimension is 10.
	left, top, width, height, ok := findBoundingBox(image, 10)
	if !ok {
		return nil
	}

	tryLocate := func(l, t, size int) (concentric.ConcentricPattern, bool) {
		return concentric.LocateConcentricPattern(image,
			bmcursor.PointF{X: float64(l + size/2), Y: float64(t + size/2)}, size/2, true)
	}

	// Symbols can have a blank row or column at the edge
	if width == height && width >= 11 {
		if p, ok := tryLocate(left, top, width); ok {
			return []concentric.ConcentricPattern{p}
		}
	} else if width < height && width >= height*10/11 && image.Width() >= height {
		if p, ok := tryLocate(left, top, height); ok {
			return []concentric.ConcentricPattern{p}
		}
		if p, ok := tryLocate(left-(height-width), top, height); ok {
			return []concentric.ConcentricPattern{p}
		}
	} else if height < width && height >= width*10/11 && image.Height() >= width {
		if p, ok := tryLocate(left, top, width); ok {
			return []concentric.ConcentricPattern{p}
		}
		if p, ok := tryLocate(left, top-(width-height), width); ok {
			return []concentric.ConcentricPattern{p}
		}
	}

	return nil
}

// findFinderPatterns scans the binarized image rows for Aztec center
// pattern candidates (zxing-cpp FindFinderPatterns, PatternRow variant).
func findFinderPatterns(image *bitutil.BitMatrix, tryHarder bool) []concentric.ConcentricPattern {
	var res []concentric.ConcentricPattern

	h := image.Height()
	skip, margin := clampInt(h/2/100, 1, 5), h/4
	if tryHarder {
		skip, margin = 1, 5
	}

	var rowBuf []int
	for y := margin; y < h-margin; y += skip {
		row := getPatternRow(image, y, rowBuf)
		next := pattern.NewView(row)
		next.Shift(1) // the center pattern we are looking for starts with white and is 7 wide (compact code)

		for {
			next = findAztecCenterPattern(next)
			if !next.IsValidSize() {
				break
			}

			p := bmcursor.PointF{X: float64(next.PixelsInFront()+next.At(0)+next.At(1)+next.At(2)) + float64(next.At(3))/2.0, Y: float64(y) + 0.5}

			// make sure p is not 'inside' an already found pattern area
			found := false
			for i := len(res) - 1; i >= 0; i-- {
				old := res[i]
				// search from back to front, stop once we are out of range
				// due to the y-coordinate
				if p.Y-old.P.Y > old.Size/2 {
					break
				}
				if bmcursor.DistanceF(p, old.P) < old.Size/2 {
					found = true
					break
				}
			}

			if !found {
				if pat, ok := locateAztecCenter(image, p, next.Sum(0)); ok {
					res = append(res, pat)
				}
			}

			next.SkipPair()
			next.Extend()
		}
	}

	return res
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// findRotation matches the 12 orientation bits against the expected pattern
// in 4 rotations, allowing at most 2 wrong bits (zxing-cpp FindRotation;
// ISO/IEC 24778:2008(E) 14.3.3 says 3 but that is wrong).
func findRotation(b uint32, mirror bool) int {
	mask := uint32(0b111_011_100_000)
	if mirror {
		mask = 0b111_000_001_110
	}
	for i := range 4 {
		if bits.OnesCount32(mask^b) <= 2 {
			return i
		}
		b = ((b << 3) & 0xfff) | ((b >> 9) & 0b111) // left shift/rotate
	}
	return -1
}

// sampleOrientationBits reads 4*3=12 bits from the 4 corners of the finder
// pattern at radius (zxing-cpp SampleOrientationBits).
func sampleOrientationBits(image *bitutil.BitMatrix, mod2Pix localgrid.PerspectiveTransform, radius int) uint32 {
	var b uint32
	for _, d := range [4]bmcursor.PointI{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}} {
		corner := bmcursor.PointI{X: radius * d.X, Y: radius * d.Y}
		cornerL := bmcursor.PointI{X: corner.X, Y: corner.Y - d.Y}
		cornerR := bmcursor.PointI{X: corner.X - d.X, Y: corner.Y}
		if d.X != d.Y {
			cornerL, cornerR = cornerR, cornerL
		}
		for _, ps := range [3]bmcursor.PointI{cornerL, corner, cornerR} {
			p := mod2Pix.Apply(bmcursor.PointFI(ps))
			if !bmcursor.IsInF(image, p) {
				return 0
			}
			b <<= 1
			if image.Get(int(p.X), int(p.Y)) {
				b |= 1
			}
		}
	}
	return b
}

// sampleModeMessage reads the bits between the corner bits along the 4
// edges and error corrects them (zxing-cpp ModeMessage). It returns the mode
// message, whether it is a Rune and the number of corrected codewords, or
// modeMessage == -1 on failure.
func sampleModeMessage(image *bitutil.BitMatrix, mod2Pix localgrid.PerspectiveTransform, radius int) (modeMessage int, isRune bool, ec int) {
	compact := radius == 5

	var b uint64
	for _, d := range [4]bmcursor.PointI{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}} {
		corner := bmcursor.PointI{X: radius * d.X, Y: radius * d.Y}
		next := bmcursor.PointI{X: 0, Y: -d.Y}
		if d.X == d.Y {
			next = bmcursor.PointI{X: -d.X, Y: 0}
		}
		for i := 2; i <= 2*radius-2; i++ {
			if !compact && i == 7 {
				continue // skip the timing pattern
			}
			p := mod2Pix.Apply(bmcursor.PointFI(bmcursor.PointI{X: corner.X + i*next.X, Y: corner.Y + i*next.Y}))
			if !bmcursor.IsInF(image, p) {
				return -1, false, 0
			}
			b <<= 1
			if image.Get(int(p.X), int(p.Y)) {
				b |= 1
			}
		}
	}

	// error correct bits
	numCodewords, numDataCodewords := 10, 4
	if compact {
		numCodewords, numDataCodewords = 7, 2
	}
	numECCodewords := numCodewords - numDataCodewords

	words := make([]int, numCodewords)
	for i := numCodewords - 1; i >= 0; i-- {
		words[i] = int(b & 0xF)
		b >>= 4
	}

	dec := reedsolomon.NewDecoder(reedsolomon.AztecParam)
	ok := false
	ec, err := dec.Decode(words, numECCodewords)
	if err == nil {
		ok = true
	}

	if !ok && compact {
		// Is this a Rune?
		for i := range words {
			words[i] ^= 0b1010
		}
		ec, err = dec.Decode(words, numECCodewords)
		if err == nil {
			ok = true
			isRune = true
		}
	}

	if !ok {
		return -1, false, 0
	}

	modeMessage = 0
	for i := range numDataCodewords {
		modeMessage = (modeMessage << 4) + words[i]
	}
	return modeMessage, isRune, ec
}

// extractParameters reads layers and data block count from the mode message
// (zxing-cpp ExtractParameters; the readerInit flag is not used by this
// decoder and is dropped).
func extractParameters(modeMessage int, compact bool) (nbLayers, nbDataBlocks int) {
	if compact {
		// 8 bits:  2 bits layers and 6 bits data blocks
		nbLayers = (modeMessage >> 6) + 1
		if nbLayers == 1 && modeMessage&0x20 != 0 { // ISO/IEC 24778:2008 Section 9 MSB artificially set
			modeMessage &= ^0x20
		}
		nbDataBlocks = (modeMessage & 0x3F) + 1
	} else {
		// 16 bits:  5 bits layers and 11 bits data blocks
		nbLayers = (modeMessage >> 11) + 1
		if nbLayers <= 22 && modeMessage&0x400 != 0 { // ISO/IEC 24778:2008 Section 9 MSB artificially set
			modeMessage &= ^0x400
		}
		nbDataBlocks = (modeMessage & 0x7FF) + 1
	}
	return nbLayers, nbDataBlocks
}

// Detect locates Aztec barcodes in the given binarized image and returns
// all detection candidates (zxing-cpp Detect(image, isPure, tryHarder,
// maxSymbols, standard, runes) with maxSymbols unbounded, standard == true
// and runes == false: Aztec Runes are skipped because this decoder does not
// support them).
func Detect(image *bitutil.BitMatrix, isPure, tryHarder bool) []DetectorResult {
	var res []DetectorResult

	var fps []concentric.ConcentricPattern
	if isPure {
		fps = findPureFinderPattern(image)
	} else {
		fps = findFinderPatterns(image, tryHarder)
	}

	for _, fp := range fps {
		fpQuad, ok := concentric.FindConcentricPatternCorners(image, fp.P, int(fp.Size), 3)
		if !ok {
			continue
		}

		srcQuad := localgrid.CenteredSquare(7)
		mod2Pix := localgrid.NewPerspectiveTransform(srcQuad, fpQuad)
		if !mod2Pix.IsValid() {
			continue
		}

		var radius, mirror, rotate int // radius: 5 or 7 (compact vs. full)
		modeMessage := -1
		var isRune bool
		var ec int

		// 24778:2008(E) 14.3.3 reads: the 12 orientation bits at the corners
		// are bitwise compared against the specified pattern in each of four
		// possible orientations and their four mirror inverse orientations.
		// Unfortunately, the "9 of 12 bits" tolerance seems to be wrong, so
		// the complete set of mode message bits is used to help determine
		// the orientation of the symbol.
		parseModeMessage := func(sq localgrid.Quad, fq localgrid.Quad) bool {
			for radius = 5; radius <= 7; radius += 2 {
				b := sampleOrientationBits(image, localgrid.NewPerspectiveTransform(sq, fq), radius)
				if b == 0 {
					continue
				}
				for mirror = 0; mirror <= 1; mirror++ {
					rotate = findRotation(b, mirror == 1)
					if rotate == -1 {
						continue
					}
					modeMessage, isRune, ec = sampleModeMessage(image,
						localgrid.NewPerspectiveTransform(sq, localgrid.RotatedCorners(fq, rotate, mirror == 1)), radius)
					if modeMessage != -1 {
						return true
					}
				}
			}
			return false
		}

		if !parseModeMessage(srcQuad, fpQuad) || radius == 7 {
			// improve precision of sample grid by extrapolating from the
			// outer square of white pixels (5 edges away from center)
			if fpQuad5, ok := concentric.FindConcentricPatternCorners(image, fp.P, int(fp.Size*5/3), 5); ok {
				if parseModeMessage(localgrid.CenteredSquare(11), fpQuad5) && radius == 7 {
					srcQuad = localgrid.CenteredSquare(11)
					fpQuad = fpQuad5
				}
				if modeMessage == -1 {
					continue
				}
			}
		}

		// zxing-cpp: if ((!standard && !isRune) || (!runes && isRune)) continue;
		// standard == true, runes == false here.
		if isRune {
			continue
		}

		nbLayers, nbDataBlocks := extractParameters(modeMessage, radius == 5)

		dim := 4*nbLayers + 11
		if radius != 5 {
			dim = 4*nbLayers + 2*((2*nbLayers+6)/15) + 15
		}

		center := bmcursor.PointF{X: float64(dim) / 2, Y: float64(dim) / 2}
		srcQuad = localgrid.MoveQuad(srcQuad, center)
		fpQuad = localgrid.RotatedCorners(fpQuad, rotate, mirror == 1)
		mod2Pix = localgrid.NewPerspectiveTransform(srcQuad, fpQuad)

		var bits *bitutil.BitMatrix
		var quad [4]bmcursor.PointI
		var ok2 bool
		if !isPure && dim >= 35 {
			// for symbols with timing patterns, find those starting from the
			// center and successively move outward
			bits, quad, ok2 = sampleGridRefined(image, dim, center, mod2Pix)
		} else {
			bits, quad, ok2 = localgrid.SampleGrid(image, dim, dim, mod2Pix)
		}
		if !ok2 {
			continue
		}

		res = append(res, DetectorResult{
			Bits:            bits,
			Points:          quadToPoints(quad),
			Compact:         radius == 5,
			NbDataBlocks:    nbDataBlocks,
			NbLayers:        nbLayers,
			ErrorsCorrected: ec,
		})
	}

	return res
}

// quadToPoints converts the projected symbol corners to result points.
func quadToPoints(quad [4]bmcursor.PointI) []zxinggo.ResultPoint {
	pts := make([]zxinggo.ResultPoint, 4)
	for i, p := range quad {
		pts[i] = zxinggo.ResultPoint{X: float64(p.X), Y: float64(p.Y)}
	}
	return pts
}

// sampleGridRefined samples the symbol grid, refining the projection with
// the alignment (timing) pattern crosses found in the image (zxing-cpp
// AZDetector.cpp alignment pattern path).
func sampleGridRefined(image *bitutil.BitMatrix, dim int, center bmcursor.PointF, mod2Pix localgrid.PerspectiveTransform) (*bitutil.BitMatrix, [4]bmcursor.PointI, bool) {
	R := (dim / 2) / 16
	firstTimingPattern := dim/2 - R*16

	var apM []int // alignment pattern positions in modules
	for i := firstTimingPattern; i < dim; i += 16 {
		apM = append(apM, i)
	}
	n := len(apM)
	apP := make([][]localgrid.APPoint, n) // found/guessed positions in pixels
	for y := range n {
		apP[y] = make([]localgrid.APPoint, n)
	}
	apP[n/2][n/2] = localgrid.APPoint{P: mod2Pix.Apply(center), OK: true} // center point

	for r := R - 1; r >= 0; r-- {
		srcQuad := localgrid.MoveQuad(localgrid.CenteredSquare(32*(R-r)), center)
		idxs := [4]bmcursor.PointI{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}}
		var dstQuad localgrid.Quad
		grid := localgrid.New(image, mod2Pix, bmcursor.PointI{X: dim, Y: dim})
		for i := range 4 {
			pi := bmcursor.PointI{X: (R-r)*idxs[i].X + n/2, Y: (R-r)*idxs[i].Y + n/2}
			grid.At(bmcursor.PointIF(srcQuad[i]), bmcursor.PointI{})
			if p, ok := grid.FindTimingPatternCross(true, 4); ok {
				apP[pi.Y][pi.X] = localgrid.APPoint{P: p, OK: true}
			}
			if apP[pi.Y][pi.X].OK {
				dstQuad[i] = apP[pi.Y][pi.X].P
			} else {
				dstQuad[i] = mod2Pix.Apply(srcQuad[i])
			}
		}
		mod2Pix = localgrid.NewPerspectiveTransform(srcQuad, dstQuad)
	}

	// find the remaining (non-corner) alignment patterns
	grid := localgrid.New(image, mod2Pix, bmcursor.PointI{X: dim, Y: dim})
	for y := range n {
		for x := range n {
			if !apP[y][x].OK {
				grid.At(bmcursor.PointI{X: apM[x], Y: apM[y]}, bmcursor.PointI{})
				if p, ok := grid.FindTimingPatternCross(true, 4); ok {
					apP[y][x] = localgrid.APPoint{P: p, OK: true}
				}
			}
		}
	}

	return localgrid.SampleGridAlignment(image, dim, dim, mod2Pix, apP, apM, apM)
}
