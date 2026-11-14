// Package pattern implements the run-length pattern-row / PatternView
// machinery ported from zxing-cpp (core/src/Pattern.h, oned/ODRowReader.h).
//
// A pattern row is a []int of run lengths that always starts with the white
// run in front of the first bar, followed by alternating bar/space widths
// starting with the first bar. A View is a window of Size() counters over
// such a row; At(0) is the current bar and At(-1) the space in front of it.
package pattern

import "math"

// View is a window over a pattern row (ported from zxing-cpp PatternView).
type View struct {
	base []int
	i    int // index of the view's first element in base
	size int
}

// NewView returns a view of the whole row, positioned at the first bar
// (skipping the leading white run).
func NewView(row []int) View {
	if len(row) == 0 {
		return View{}
	}
	return View{base: row, i: 1, size: len(row) - 1}
}

// At returns the counter n positions ahead of the view start (n may be
// negative to inspect the space in front; out-of-range reads yield 0).
func (v View) At(n int) int {
	j := v.i + n
	if j < 0 || j >= len(v.base) {
		return 0
	}
	return v.base[j]
}

// Size returns the number of counters in the view.
func (v View) Size() int { return v.size }

// Row returns the full pattern row underlying the view.
func (v View) Row() []int { return v.base }

// Sum sums n counters from the view start (n == 0: the whole view).
func (v View) Sum(n int) int {
	if n == 0 {
		n = v.size
	}
	if n < 0 {
		n = 0
	}
	s := 0
	for _, c := range v.base[v.i:min(v.i+n, len(v.base))] {
		s += c
	}
	return s
}

// Index is the number of bars/spaces from the first bar to the view start.
func (v View) Index() int { return v.i - 1 }

// PixelsInFront returns the pixel offset of the view start in the row.
func (v View) PixelsInFront() int {
	s := 0
	for _, c := range v.base[:min(v.i, len(v.base))] {
		s += c
	}
	return s
}

// PixelsTillEnd returns the pixel offset of the view end minus one.
func (v View) PixelsTillEnd() int {
	s := 0
	for _, c := range v.base[:min(v.i+v.size, len(v.base))] {
		s += c
	}
	return s - 1
}

// IsAtFirstBar reports whether the view starts at the first bar.
func (v View) IsAtFirstBar() bool { return v.i == 1 }

// IsAtLastBar reports whether the view ends at the last bar.
func (v View) IsAtLastBar() bool { return v.i+v.size == len(v.base)-1 }

// IsValid reports whether a window of n counters fits in the row.
func (v View) IsValid(n int) bool {
	return v.base != nil && v.i >= 0 && v.i+n <= len(v.base)
}

// IsValidSize reports whether the view's own size fits in the row.
func (v View) IsValidSize() bool { return v.IsValid(v.size) }

// SpaceInFront returns the white run in front of the view (MaxInt32 at the
// first bar).
func (v View) SpaceInFront() int {
	if v.IsAtFirstBar() {
		return math.MaxInt32
	}
	return v.At(-1)
}

// SubView returns a nested view at the given offset and size (0: to the end
// of this view, negative: shrink from the end).
func (v View) SubView(offset, size int) View {
	if size == 0 {
		size = v.size - offset
	} else if size < 0 {
		size = v.size - offset + size
	}
	if size < 0 {
		size = 0
	}
	return View{base: v.base, i: v.i + offset, size: size}
}

// Shift moves the view n counters forward; it returns false once the view
// runs past the end of the row (the move still happens, as in zxing-cpp).
func (v *View) Shift(n int) bool {
	if v.base == nil {
		return false
	}
	v.i += n
	return v.i+v.size <= len(v.base)
}

// SkipPair advances by two counters (one bar and one space).
func (v *View) SkipPair() bool { return v.Shift(2) }

// SkipSymbol advances by the view's own size.
func (v *View) SkipSymbol() bool { return v.Shift(v.size) }

// SkipSingle advances by one counter and requires the skipped run to be at
// most maxWidth pixels.
func (v *View) SkipSingle(maxWidth int) bool {
	return v.Shift(1) && v.At(-1) <= maxWidth
}

// Extend grows the view to the end of the row.
func (v *View) Extend() {
	v.size = len(v.base) - v.i
	if v.size < 0 {
		v.size = 0
	}
}

// IsPattern matches view against a fixed pattern (sum SUM over N runs).
// It returns the estimated module size on success, 0 on mismatch. Ported
// from zxing-cpp Pattern.h IsPattern (non-end-to-end path).
func IsPattern(view View, pat []int, spaceInPixel int, minQuietZone float64, moduleSizeRef float64) float64 {
	n := len(pat)
	sum := 0
	for _, p := range pat {
		sum += p
	}
	width := view.Sum(n)
	if sum > n && width < sum {
		return 0
	}
	moduleSize := float64(width) / float64(sum)
	if minQuietZone != 0 && float64(spaceInPixel) < minQuietZone*moduleSize-1 {
		return 0
	}
	if moduleSizeRef == 0 {
		moduleSizeRef = moduleSize
	}
	// The 0.5 offset desensitizes quantization for small module sizes.
	threshold := moduleSizeRef*0.5 + 0.5
	for x := range n {
		d := float64(view.At(x)) - float64(pat[x])*moduleSizeRef
		if d < 0 {
			d = -d
		}
		if d > threshold {
			return 0
		}
	}
	return moduleSize
}

// IsRightGuard matches a right-guard pattern at the view, honoring the
// quiet zone behind it (infinite when the view ends at the last bar).
func IsRightGuard(view View, pat []int, minQuietZone float64, moduleSizeRef float64) bool {
	if view.Size() != len(pat) || !view.IsValidSize() {
		return false
	}
	space := math.MaxInt32
	if !view.IsAtLastBar() {
		space = view.At(view.Size())
	}
	return IsPattern(view, pat, space, minQuietZone, moduleSizeRef) != 0
}

// FindLeftGuard scans for a window matching the guard pattern with the
// required quiet zone in front, starting at the current view position.
func FindLeftGuard(view View, minSize int, pat []int, minQuietZone float64) View {
	n := len(pat)
	if minSize < n {
		minSize = n
	}
	if view.Size() < minSize {
		return View{}
	}
	window := view.SubView(0, n)
	if window.IsAtFirstBar() && IsPattern(window, pat, math.MaxInt32, minQuietZone, 0) != 0 {
		return window
	}
	end := view.i + view.Size() - minSize
	for window.i < end {
		if IsPattern(window, pat, window.At(-1), minQuietZone, 0) != 0 {
			return window
		}
		if !window.Shift(2) {
			break
		}
	}
	return View{}
}

// PatternMatchVariance returns the ratio of total variance to total width;
// +Inf if any individual counter deviates more than
// maxIndividualVariance * unitBarWidth. float32 arithmetic matches the
// Java/zxing-cpp float semantics.
func PatternMatchVariance(counters []int, pat []int, maxIndividualVariance float64) float64 {
	total, patternLength := 0, 0
	for i := range pat {
		total += counters[i]
		patternLength += pat[i]
	}
	if total < patternLength {
		return math.Inf(1)
	}
	unitBarWidth := float32(total) / float32(patternLength)
	maxIndVar := float32(maxIndividualVariance) * unitBarWidth
	var totalVariance float32
	for i := range pat {
		counter := float32(counters[i])
		scaled := float32(pat[i]) * unitBarWidth
		variance := counter - scaled
		if variance < 0 {
			variance = -variance
		}
		if variance > maxIndVar {
			return math.Inf(1)
		}
		totalVariance += variance
	}
	return float64(totalVariance / float32(total))
}

// DecodeDigit returns the index of the best-matching pattern, or -1.
// Ported from zxing-cpp RowReader::DecodeDigit.
func DecodeDigit(view View, patterns [][]int, maxAvgVariance, maxIndividualVariance float64, requireUnambiguous bool) int {
	var counters [16]int
	bestVariance := maxAvgVariance
	bestMatch := -1
	for i, pat := range patterns {
		for j := range pat {
			counters[j] = view.At(j)
		}
		v := PatternMatchVariance(counters[:len(pat)], pat, maxIndividualVariance)
		if v < bestVariance {
			bestVariance = v
			bestMatch = i
		} else if requireUnambiguous && v == bestVariance {
			bestMatch = -1
		}
	}
	return bestMatch
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
