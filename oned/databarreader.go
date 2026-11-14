package oned

import (
	"fmt"
	"math"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/pattern"
)

// DataBarReader decodes GS1 DataBar-14 (Omni and Stacked) barcodes from
// pattern rows, ported from zxing-cpp's oned/ODDataBarReader.cpp. It
// replaces the Java ZXing RSS14Reader: the edge-to-edge character model
// and the strict finder/guard/character checks decode blurred and
// low-contrast symbols the run-length odd/even adjustment cannot.
type DataBarReader struct {
	leftPairs  []dbPair
	rightPairs []dbPair
}

func NewDataBarReader() *DataBarReader {
	return &DataBarReader{}
}

// Reset clears the accumulated pairs between scan passes
// (zxing-cpp DataBarReader::reset via DecodingState).
func (r *DataBarReader) Reset() {
	r.leftPairs = r.leftPairs[:0]
	r.rightPairs = r.rightPairs[:0]
}

const dbFullPairSize = 8 + 5 + 8 // left char + finder + right char

type dbCharacter struct {
	value    int
	checksum int
}

type dbPair struct {
	left, right dbCharacter
	finder      int
	xStart      int
	xStop       int
	y           int
	count       int
}

// key identifies a pair for tallying (zxing-cpp Pair::operator== / PairHash:
// finder and both characters, not position or count).
func (p dbPair) key() [5]int {
	return [5]int{p.finder, p.left.value, p.left.checksum, p.right.value, p.right.checksum}
}

// --- finder / guard / character checks (ODDataBarCommon.h) ---

// dbIsFinder checks the five finder elements a..e (sum 15 modules) using
// only bar+space pairs to limit the effect of poor thresholding.
func dbIsFinder(a, b, c, d, e int) bool {
	w := 2 * (b + c) // b+c can be 10, 11 or 12 modules
	n := d + e       // d+e is always 2
	// the offsets (5 and 2) reduce quantization effects for small module sizes
	return w+5 > 9*n && w-5 < 13*n && a < 2+4*e && 4*a > n
}

func dbFinder(v pattern.View) pattern.View    { return v.SubView(8, 5) }
func dbLeftChar(v pattern.View) pattern.View  { return v.SubView(0, 8) }
func dbRightChar(v pattern.View) pattern.View { return v.SubView(13, 8) }

func dbModSizeFinder(v pattern.View) float64 {
	return float64(dbFinder(v).Sum(0)) / 15.0
}

func dbIsGuard(a, b int) bool {
	return a > b*3/4-2 && a < b*5/4+2
}

func dbIsCharacter(v pattern.View, modules int, modSizeRef float64) bool {
	err := math.Abs(float64(v.Sum(0))/float64(modules)/modSizeRef - 1)
	return err < 0.1
}

func dbIsCharacterPair(v pattern.View, modsLeft, modsRight int) bool {
	modSizeRef := dbModSizeFinder(v)
	return dbIsCharacter(dbLeftChar(v), modsLeft, modSizeRef) &&
		dbIsCharacter(dbRightChar(v), modsRight, modSizeRef)
}

func dbIsLeftPair(v pattern.View) bool {
	return dbIsFinder(v.At(8), v.At(9), v.At(10), v.At(11), v.At(12)) &&
		dbIsGuard(v.At(-1), v.At(11)) &&
		dbIsCharacterPair(v, 16, 15)
}

func dbIsRightPair(v pattern.View) bool {
	return dbIsFinder(v.At(12), v.At(11), v.At(10), v.At(9), v.At(8)) &&
		dbIsGuard(v.At(9), v.At(21)) &&
		dbIsCharacterPair(v, 15, 16)
}

// --- edge-to-edge normalization (Pattern.h / ODDataBarCommon.h) ---

// dbNormalizedE2E measures RET_LEN edge-to-similar-edge distances of
// LEN=RET_LEN+2 counters, normalized to mods modules (all zeros if the
// module size is not positive).
func dbNormalizedE2E(v pattern.View, mods int, reversed bool, out []int) {
	n := len(out) + 2
	moduleSize := float64(v.Sum(n)) / float64(mods)
	for i := range out {
		out[i] = 0
		if moduleSize > 0 {
			iv := i
			if reversed {
				iv = n - 2 - i
			}
			out[i] = int(float64(v.At(iv)+v.At(iv+1))/moduleSize + .5)
		}
	}
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// dbNormalizedPatternFromE2E derives the 8 element widths of a data
// character from 6 edge-to-edge measurements (NormalizedPatternFromE2E<8>).
func dbNormalizedPatternFromE2E(v pattern.View, mods int, reversed bool) [8]int {
	var widths [8]int
	var e2e [6]int
	dbNormalizedE2E(v, mods, reversed, e2e[:])

	// min-even-is-one: DataBarLimited, DataBar outside character
	// min-odd-is-one:  DataBarExpanded, DataBar inside character
	minOddIsOne := mods == 15 || mods == 17

	barSum := 0
	if minOddIsOne {
		widths[0] = 8
	} else {
		widths[0] = 1
	}
	barSum = widths[0]
	for i := 0; i < 6; i++ {
		widths[i+1] = e2e[i] - widths[i]
		barSum += widths[i+1]
	}
	widths[7] = mods - barSum // last even element makes mods modules

	minOdd, minEven := widths[0], widths[1]
	for i := 2; i < 8; i++ {
		if i&1 == 0 {
			if widths[i] < minOdd {
				minOdd = widths[i]
			}
		} else if widths[i] < minEven {
			minEven = widths[i]
		}
	}

	if minOddIsOne && minOdd > 1 {
		// minimum odd width is too big, readjust so minimum odd is 1
		for i := 0; i < 8; i += 2 {
			widths[i] -= minOdd - 1
			widths[i+1] += minOdd - 1
		}
	} else if !minOddIsOne && minEven > 1 {
		// minimum even width is too big, readjust so minimum even is 1
		for i := 0; i < 8; i += 2 {
			widths[i] += minEven - 1
			widths[i+1] -= minEven - 1
		}
	}

	return widths
}

// dbReadDataCharacterRaw extracts the odd/even element widths of a data
// character and rejects any character not exactly fitting the module sum
// and parity constraints.
func dbReadDataCharacterRaw(v pattern.View, numModules int, reversed bool, odd, even *[4]int) bool {
	p := dbNormalizedPatternFromE2E(v, numModules, reversed)
	for i := 0; i < 8; i++ {
		if i&1 == 0 {
			odd[i/2] = p[i]
		} else {
			even[i/2] = p[i]
		}
	}

	const minSum = 4 // each data character has 4 bars and 4 spaces
	maxSum := numModules - minSum
	oddSum := odd[0] + odd[1] + odd[2] + odd[3]
	evnSum := even[0] + even[1] + even[2] + even[3]

	sumErr := oddSum + evnSum - numModules
	// sum < min -> negative error; sum > max -> positive error
	oddMin := minSum
	if numModules == 15 {
		oddMin++
	}
	evnMax := maxSum
	if numModules == 15 {
		evnMax--
	}
	oddSumErr := min(0, oddSum-oddMin) + max(0, oddSum-maxSum)
	evnSumErr := min(0, evnSum-minSum) + max(0, evnSum-evnMax)

	oddParityErr := (oddSum&1 == 1) == (numModules > 15)
	evnParityErr := (evnSum&1 == 1) == (numModules < 17)

	// Ignore any character that is not exactly fitting the requirements.
	return sumErr == 0 && oddSumErr == 0 && evnSumErr == 0 && !oddParityErr && !evnParityErr
}

// --- finder pattern parsing ---

// dbE2EFinderPatterns are the 9 DataBar finder patterns as normalized
// edge-to-edge triples (ODDataBarReader.cpp ParseFinderPattern).
var dbE2EFinderPatterns = [9][3]int{
	{11, 10, 3}, // {3, 8, 2, 1, 1}
	{8, 10, 6},  // {3, 5, 5, 1, 1}
	{6, 10, 8},  // {3, 3, 7, 1, 1}
	{4, 10, 10}, // {3, 1, 9, 1, 1}
	{9, 11, 5},  // {2, 7, 4, 1, 1}
	{7, 11, 7},  // {2, 5, 6, 1, 1}
	{5, 11, 9},  // {2, 3, 8, 1, 1}
	{6, 12, 8},  // {1, 5, 7, 1, 1}
	{4, 12, 10}, // {1, 3, 9, 1, 1}
}

func dbParseFinderPattern(v pattern.View, reversed bool) int {
	var e2e [3]int
	dbNormalizedE2E(v, 15, reversed, e2e[:])

	bestI, bestE := -1, 3
	for i := 0; i < len(dbE2EFinderPatterns); i++ {
		e := 0
		for j := 0; j < 3; j++ {
			e += abs(dbE2EFinderPatterns[i][j] - e2e[j])
		}
		if e < bestE {
			bestE, bestI = e, i
		}
	}
	i := 0
	if bestE <= 1 {
		i = 1 + bestI
	}
	if reversed {
		return -i
	}
	return i
}

// --- data character / pair decoding (ODDataBarReader.cpp) ---

var (
	dbOutsideEvenTotalSubset = [5]int{1, 10, 34, 70, 126}
	dbInsideOddTotalSubset   = [4]int{4, 20, 48, 81}
	dbOutsideGsum            = [5]int{0, 161, 961, 2015, 2715}
	dbInsideGsum             = [4]int{0, 336, 1036, 1516}
	dbOutsideOddWidest       = [5]int{8, 6, 4, 3, 1}
	dbInsideOddWidest        = [4]int{2, 4, 6, 8}
)

func dbReadDataCharacter(v pattern.View, outsideChar, rightPair bool) (dbCharacter, bool) {
	var odd, even [4]int
	numModules := 15
	if outsideChar {
		numModules = 16
	}
	if !dbReadDataCharacterRaw(v, numModules, outsideChar == rightPair, &odd, &even) {
		return dbCharacter{}, false
	}

	calcChecksumPortion := func(a [4]int) int {
		res := 0
		for i := 3; i >= 0; i-- {
			res = 9*res + a[i]
		}
		return res
	}
	checksumPortion := calcChecksumPortion(odd) + 3*calcChecksumPortion(even)

	if outsideChar {
		oddSum := odd[0] + odd[1] + odd[2] + odd[3]
		group := (12 - oddSum) / 2
		oddWidest := dbOutsideOddWidest[group]
		evnWidest := 9 - oddWidest
		vOdd := getRSSvalue(odd[:], oddWidest, false)
		vEvn := getRSSvalue(even[:], evnWidest, true)
		tEvn := dbOutsideEvenTotalSubset[group]
		gSum := dbOutsideGsum[group]
		return dbCharacter{value: vOdd*tEvn + vEvn + gSum, checksum: checksumPortion}, true
	}

	evnSum := even[0] + even[1] + even[2] + even[3]
	group := (10 - evnSum) / 2
	oddWidest := dbInsideOddWidest[group]
	evnWidest := 9 - oddWidest
	vOdd := getRSSvalue(odd[:], oddWidest, true)
	vEvn := getRSSvalue(even[:], evnWidest, false)
	tOdd := dbInsideOddTotalSubset[group]
	gSum := dbInsideGsum[group]
	return dbCharacter{value: vEvn*tOdd + vOdd + gSum, checksum: checksumPortion}, true
}

func dbReadPair(v pattern.View, rightPair bool) (dbPair, bool) {
	finder := dbParseFinderPattern(dbFinder(v), rightPair)
	if finder == 0 {
		return dbPair{}, false
	}
	var outside, inside pattern.View
	if rightPair {
		outside, inside = dbRightChar(v), dbLeftChar(v)
	} else {
		outside, inside = dbLeftChar(v), dbRightChar(v)
	}
	oc, ok := dbReadDataCharacter(outside, true, rightPair)
	if !ok {
		return dbPair{}, false
	}
	ic, ok := dbReadDataCharacter(inside, false, rightPair)
	if !ok {
		return dbPair{}, false
	}
	// include left and right guards
	xStart := v.PixelsInFront() - v.At(-1)
	xStop := v.PixelsTillEnd() + 2*v.At(dbFullPairSize)
	return dbPair{left: oc, right: ic, finder: finder, xStart: xStart, xStop: xStop}, true
}

func dbPairValue(p dbPair) int64 {
	return 1597*int64(p.left.value) + int64(p.right.value)
}

func dbValue(leftPair, rightPair dbPair) int64 {
	res := int64(4537077)*dbPairValue(leftPair) + dbPairValue(rightPair)
	if res >= 10000000000000 {
		// strip the 2D linkage flag (GS1 Composite) if any,
		// ISO/IEC 24724:2011 Section 5.2.3
		res -= 10000000000000
	}
	return res
}

func dbChecksumIsValid(leftPair, rightPair dbPair) bool {
	checksum := func(p dbPair) int { return p.left.checksum + 4*p.right.checksum }
	a := (checksum(leftPair) + 16*checksum(rightPair)) % 79
	b := 9*(abs(leftPair.finder)-1) + (abs(rightPair.finder) - 1)
	if b > 72 {
		b--
	}
	if b > 8 {
		b--
	}
	return a == b && dbValue(leftPair, rightPair) <= 9999999999999 // 13 digits
}

func dbPositionIsPlausible(l, r dbPair) bool {
	wl := l.xStop - l.xStart
	wr := r.xStop - r.xStart
	h := abs(l.y - r.y)
	dxl := abs(l.xStart - r.xStart)
	dxr := abs(l.xStop - r.xStart)

	// - the pairs must be wider than the stack is high
	// - the pairs must be roughly of the same width
	// - the pairs x positions must be sufficiently aligned (not too slanted)
	return h < wl && h < wr && wl > wr/2 && wr > wl/2 && (dxl < wl/2 || dxr < wr/2)
}

func dbIsStacked(first, last dbPair) bool {
	// two halves that are far away from each other in y or overlapping in x
	return abs(first.y-last.y) > first.xStop-first.xStart ||
		last.xStart < (first.xStart+first.xStop)/2
}

// dbConstructText renders the 14-digit GTIN (13 symbol digits + mod-10
// check digit), matching the Java ZXing RSS-14 text format.
func dbConstructText(leftPair, rightPair dbPair) string {
	txt := fmt.Sprintf("%013d", dbValue(leftPair, rightPair))
	checkDigit := 0
	for i := 0; i < 13; i++ {
		digit := int(txt[i] - '0')
		if i&1 == 0 {
			checkDigit += 3 * digit
		} else {
			checkDigit += digit
		}
	}
	checkDigit = 10 - (checkDigit % 10)
	if checkDigit == 10 {
		checkDigit = 0
	}
	return txt + string(rune('0'+checkDigit))
}

func dbConstructResult(leftPair, rightPair dbPair) *zxinggo.Result {
	var points []zxinggo.ResultPoint
	if dbIsStacked(leftPair, rightPair) {
		points = []zxinggo.ResultPoint{
			{X: float64(leftPair.xStart), Y: float64(leftPair.y)},
			{X: float64(leftPair.xStop), Y: float64(leftPair.y)},
			{X: float64(rightPair.xStop), Y: float64(rightPair.y)},
			{X: float64(rightPair.xStart), Y: float64(rightPair.y)},
		}
	} else {
		y := (leftPair.y + rightPair.y) / 2
		points = []zxinggo.ResultPoint{
			{X: float64(leftPair.xStart), Y: float64(y)},
			{X: float64(rightPair.xStop), Y: float64(y)},
		}
	}
	res := zxinggo.NewResult(dbConstructText(leftPair, rightPair), nil, points, zxinggo.FormatRSS14)
	res.PutMetadata(zxinggo.MetadataSymbologyIdentifier, "]e0")
	return res
}

// dbTally inserts the pair or increments the count of an existing one
// (zxing-cpp unordered_set insert / count++).
func dbTally(list *[]dbPair, p dbPair) {
	key := p.key()
	for i := range *list {
		if (*list)[i].key() == key {
			(*list)[i].count++
			return
		}
	}
	*list = append(*list, p)
}

// DecodePattern implements PatternRowDecoder (DataBarReader::decodePattern).
// Pairs are accumulated across rows so stacked symbols (two halves on
// different rows) combine; a pair must be seen on more than one row.
func (r *DataBarReader) DecodePattern(rowNumber int, view *pattern.View, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	next := view.SubView(0, dbFullPairSize+1) // +1: guard pattern on the right, see dbIsRightPair
	// the first view tested is at index 1 (a black bar at 0 would be the guard)
	for next.Shift(1) {
		if dbIsLeftPair(next) {
			if lp, ok := dbReadPair(next, false); ok {
				lp.y = rowNumber
				dbTally(&r.leftPairs, lp)
				next.Shift(dbFullPairSize - 1)
			}
		}
		if next.Shift(1) && dbIsRightPair(next) {
			if rp, ok := dbReadPair(next, true); ok {
				rp.y = rowNumber
				dbTally(&r.rightPairs, rp)
				next.Shift(dbFullPairSize + 2)
			}
		}
	}

	for i := range r.leftPairs {
		for j := range r.rightPairs {
			lp, rp := r.leftPairs[i], r.rightPairs[j]
			if lp.count > 1 && rp.count > 1 && dbChecksumIsValid(lp, rp) && dbPositionIsPlausible(lp, rp) {
				res := dbConstructResult(lp, rp)
				// remove the consumed pairs so a second symbol can be found
				r.leftPairs = append(r.leftPairs[:i], r.leftPairs[i+1:]...)
				r.rightPairs = append(r.rightPairs[:j], r.rightPairs[j+1:]...)
				*view = pattern.View{} // guarantee progress (ODReader loop)
				return res, nil
			}
		}
	}

	*view = pattern.View{} // guarantee progress (ODReader loop)
	return nil, zxinggo.ErrNotFound
}

// DecodeRow adapts a bit row to the pattern-row interface.
func (r *DataBarReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	view := pattern.NewView(bitRowToCounters(row, nil))
	return r.DecodePattern(rowNumber, &view, opts)
}
