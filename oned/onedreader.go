// Package oned implements one-dimensional barcode reading and writing.
package oned

import (
	"math"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/binarizer"
	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/pattern"
)

// RowDecoder decodes a single row of a 1D barcode.
type RowDecoder interface {
	// DecodeRow attempts to decode a barcode from a single row.
	DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error)
}

// PatternRowDecoder decodes from run-length pattern counters, ported from
// zxing-cpp's RowReader::decodePattern. The view is positioned at the first
// bar; implementations may advance it and are expected to return quickly.
type PatternRowDecoder interface {
	DecodePattern(rowNumber int, view pattern.View, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error)
}

// DecodeOneD decodes a 1D barcode from an image, ported from zxing-cpp
// ODReader::DoDecode: rows are scanned middle-out as run-length pattern
// rows (global-histogram black point with sharpening), each forward and
// reversed; then the same is repeated over columns (90° rotation).
func DecodeOneD(image *zxinggo.BinaryBitmap, decoder RowDecoder, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	tryHarder := opts != nil && opts.TryHarder
	scanner := binarizer.NewRowScanner(image.Binarizer().LuminanceSource())
	prd, isPattern := decoder.(PatternRowDecoder)

	for _, rotate := range [2]bool{false, true} {
		if rotate {
			if rst, ok := decoder.(zxinggo.Reader); ok {
				rst.Reset()
			}
		}
		if res := scanOrientation(image, scanner, decoder, prd, isPattern, tryHarder, rotate, opts); res != nil {
			return res, nil
		}
	}
	return nil, zxinggo.ErrNotFound
}

// scanOrientation runs one middle-out scan pass, over rows (rotate=false)
// or bottom-to-top columns (rotate=true, zxing-cpp's rotated(90) view).
func scanOrientation(image *zxinggo.BinaryBitmap, scanner *binarizer.RowScanner, decoder RowDecoder, prd PatternRowDecoder, isPattern, tryHarder, rotate bool, opts *zxinggo.DecodeOptions) *zxinggo.Result {
	nLines, lineLen := image.Height(), image.Width()
	if rotate {
		nLines, lineLen = lineLen, nLines
	}
	rowStep := nLines >> 5
	if tryHarder {
		rowStep = nLines >> 8
	}
	if rowStep < 1 {
		rowStep = 1
	}
	maxLines := 15
	if tryHarder {
		maxLines = nLines
	}
	middle := nLines / 2

	var row *bitutil.BitArray
	for x := range maxLines {
		steps := (x + 1) / 2
		rowNumber := middle
		if x&0x01 == 0 {
			rowNumber += rowStep * steps
		} else {
			rowNumber -= rowStep * steps
		}
		if rowNumber < 0 || rowNumber >= nLines {
			break
		}

		var counters []int
		var err error
		if rotate {
			counters, err = scanner.Column(rowNumber)
		} else {
			counters, err = scanner.Row(rowNumber)
		}
		if err != nil {
			continue
		}

		for attempt := range 2 {
			if attempt == 1 {
				reverseCounters(counters)
			}
			var res *zxinggo.Result
			if isPattern {
				view := pattern.NewView(counters)
				for {
					res, err = prd.DecodePattern(rowNumber, view, opts)
					if err == nil && res != nil {
						break
					}
					// zxing-cpp: retry from the next bar while TryHarder.
					if !tryHarder || !view.Shift(2-view.Index()%2) {
						break
					}
					view.Extend()
					if view.Size() == 0 {
						break
					}
				}
			} else {
				row = countersToBitArray(counters, row)
				res, err = decoder.DecodeRow(rowNumber, row, opts)
			}
			if err != nil || res == nil {
				continue
			}
			if attempt == 1 {
				res.PutMetadata(zxinggo.MetadataOrientation, 180)
				for i, p := range res.Points {
					res.Points[i] = zxinggo.ResultPoint{X: float64(lineLen) - p.X - 1, Y: p.Y}
				}
			}
			if rotate {
				rotDeg := 270
				if attempt == 1 {
					rotDeg = 180
				}
				res.PutMetadata(zxinggo.MetadataOrientation, rotDeg)
				h := image.Height()
				for i, p := range res.Points {
					res.Points[i] = zxinggo.ResultPoint{X: float64(rowNumber), Y: float64(h-1) - p.X}
				}
			}
			return res
		}
	}
	return nil
}

// reverseCounters reverses a pattern row in place (zxing-cpp fixupBars'
// std::reverse; point fixup happens on the result instead).
func reverseCounters(counters []int) {
	for i, j := 0, len(counters)-1; i < j; i, j = i+1, j-1 {
		counters[i], counters[j] = counters[j], counters[i]
	}
}

// countersToBitArray expands run-length counters into a BitArray row for
// readers that still consume bit rows. The counters cover the full line,
// so the size is stable per pass and the buffer is reused.
func countersToBitArray(counters []int, row *bitutil.BitArray) *bitutil.BitArray {
	total := 0
	for _, c := range counters {
		total += c
	}
	if row == nil || row.Size() != total {
		row = bitutil.NewBitArray(total)
	} else {
		row.Clear()
	}
	pos := counters[0]
	for i := 1; i < len(counters); i += 2 {
		end := pos + counters[i]
		for j := pos; j < end; j++ {
			row.Set(j)
		}
		pos = end + counters[i+1]
	}
	return row
}

// bitRowToCounters run-length encodes a bit row into pattern-row counters
// (leading white run first).
func bitRowToCounters(row *bitutil.BitArray, buf []int) []int {
	buf = buf[:0]
	n := row.Size()
	i := 0
	if i < n && row.Get(i) {
		buf = append(buf, 0)
	} else {
		for i < n && !row.Get(i) {
			i++
		}
		buf = append(buf, i)
	}
	for i < n {
		s := i
		for i < n && row.Get(i) {
			i++
		}
		buf = append(buf, i-s)
		if i == n {
			buf = append(buf, 0)
			break
		}
		s = i
		for i < n && !row.Get(i) {
			i++
		}
		buf = append(buf, i-s)
	}
	return buf
}

// RecordPattern records the widths of successive runs of black and white
// pixels in a row, starting at the given position.
func RecordPattern(row *bitutil.BitArray, start int, counters []int) error {
	numCounters := len(counters)
	for i := range counters {
		counters[i] = 0
	}
	end := row.Size()
	if start >= end {
		return zxinggo.ErrNotFound
	}
	isWhite := !row.Get(start)
	counterPosition := 0
	i := start
	for i < end {
		if row.Get(i) != isWhite {
			counters[counterPosition]++
		} else {
			counterPosition++
			if counterPosition == numCounters {
				break
			}
			counters[counterPosition] = 1
			isWhite = !isWhite
		}
		i++
	}
	if counterPosition != numCounters && (counterPosition != numCounters-1 || i != end) {
		return zxinggo.ErrNotFound
	}
	return nil
}

// RecordPatternInReverse records a pattern by first walking backwards to find
// the start of the pattern, then recording forward.
func RecordPatternInReverse(row *bitutil.BitArray, start int, counters []int) error {
	numTransitionsLeft := len(counters)
	last := row.Get(start)
	for start > 0 && numTransitionsLeft >= 0 {
		start--
		if row.Get(start) != last {
			numTransitionsLeft--
			last = !last
		}
	}
	if numTransitionsLeft >= 0 {
		return zxinggo.ErrNotFound
	}
	return RecordPattern(row, start+1, counters)
}

// PatternMatchVariance determines how closely observed counter widths match
// a target pattern. Returns the ratio of total variance to pattern size.
// Returns +Inf if any individual counter exceeds maxIndividualVariance.
// Uses float32 arithmetic to match Java's float precision exactly.
func PatternMatchVariance(counters []int, pattern []int, maxIndividualVariance float64) float64 {
	numCounters := len(counters)
	total := 0
	patternLength := 0
	for i := 0; i < numCounters; i++ {
		total += counters[i]
		patternLength += pattern[i]
	}
	if total < patternLength {
		return math.Inf(1)
	}

	// Use float32 to match Java's float precision
	unitBarWidth := float32(total) / float32(patternLength)
	maxIndVar := float32(maxIndividualVariance) * unitBarWidth

	var totalVariance float32
	for i := 0; i < numCounters; i++ {
		counter := float32(counters[i])
		scaledPattern := float32(pattern[i]) * unitBarWidth
		variance := counter - scaledPattern
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
