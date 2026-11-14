package oned

import (
	"math"
	"strconv"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/pattern"
)

// MultiUPCEANReader decodes EAN-13, EAN-8, UPC-A and UPC-E from pattern
// rows, ported from zxing-cpp ODMultiUPCEANReader.
type MultiUPCEANReader struct {
	readEAN13 bool
	readEAN8  bool
	readUPCE  bool
	readUPCA  bool
}

// NewMultiUPCEANReader creates a reader restricted to the given formats
// (nil map enables all).
func NewMultiUPCEANReader(pf map[zxinggo.Format]bool) *MultiUPCEANReader {
	if pf == nil {
		return &MultiUPCEANReader{readEAN13: true, readEAN8: true, readUPCE: true, readUPCA: true}
	}
	return &MultiUPCEANReader{
		readEAN13: pf[zxinggo.FormatEAN13],
		readEAN8:  pf[zxinggo.FormatEAN8],
		readUPCE:  pf[zxinggo.FormatUPCE],
		readUPCA:  pf[zxinggo.FormatUPCA],
	}
}

const charLen = 4

var (
	upceEndPattern      = []int{1, 1, 1, 1, 1, 1}
	extStartPattern     = []int{1, 1, 2}
	extSeparatorPattern = []int{1, 1}
	addOnCheckEncodings = []int{0x18, 0x14, 0x12, 0x11, 0x0C, 0x06, 0x03, 0x0A, 0x09, 0x05}
	upceLPatterns       = [][]int{{3, 2, 1, 1}, {2, 2, 2, 1}, {2, 1, 2, 2}, {1, 4, 1, 1}, {1, 1, 3, 2}, {1, 2, 3, 1}, {1, 1, 1, 4}, {1, 3, 1, 2}, {1, 2, 1, 3}, {3, 1, 1, 2}}
	upceLAndGPatterns   = [][]int{{3, 2, 1, 1}, {2, 2, 2, 1}, {2, 1, 2, 2}, {1, 4, 1, 1}, {1, 1, 3, 2}, {1, 2, 3, 1}, {1, 1, 1, 4}, {1, 3, 1, 2}, {1, 2, 1, 3}, {3, 1, 1, 2}, {1, 1, 2, 3}, {1, 2, 2, 2}, {2, 2, 1, 2}, {1, 1, 4, 1}, {2, 3, 1, 1}, {1, 3, 2, 1}, {4, 1, 1, 1}, {2, 1, 3, 1}, {3, 1, 2, 1}, {2, 1, 1, 3}}
)

// GS1 quiet zone requirements (module counts), see zxing-cpp #526/#558.
const (
	quietZoneLeft     = 6
	quietZoneRightEAN = 3
	quietZoneRightUPC = 6
	quietZoneAddon    = 3
)

// DecodePattern implements PatternRowDecoder.
func (r *MultiUPCEANReader) DecodePattern(rowNumber int, view pattern.View, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	const minSize = 3 + 6*4 + 6 // UPC-E

	next := pattern.FindLeftGuard(view, minSize, startEndPattern, quietZoneLeft)
	if !next.IsValidSize() {
		return nil, zxinggo.ErrNotFound
	}
	begin := next

	var res upceanPartial
	ok := ((r.readEAN13 || r.readUPCA) && ean13Decode(&res, begin)) ||
		(r.readEAN8 && ean8Decode(&res, begin)) ||
		(r.readUPCE && upceDecode(&res, begin))
	if !ok {
		return nil, zxinggo.ErrNotFound
	}

	// The checksum is verified against the 13-digit EAN-13 expansion
	// (ISO/IEC 15420:2009), while the reported text stays the 8-digit
	// UPC-E code.
	checkTxt := res.txt
	if res.format == zxinggo.FormatUPCE {
		checkTxt = "0" + ConvertUPCEtoUPCA(res.txt)
	}

	if !CheckStandardUPCEANChecksum(checkTxt) {
		return nil, zxinggo.ErrChecksum
	}

	// A UPC-A symbol is reported as 12 digits (leading zero stripped) when
	// EAN-13 was not explicitly requested.
	if res.format == zxinggo.FormatEAN13 && !r.readEAN13 {
		if res.txt[0] != '0' {
			return nil, zxinggo.ErrNotFound
		}
		res.format = zxinggo.FormatUPCA
		res.txt = res.txt[1:]
	}

	// Symbology identifier modifiers, ISO/IEC 15420:2009 Annex B Table B.1
	symbologyID := "]E0"
	if res.format == zxinggo.FormatEAN8 {
		symbologyID = "]E4"
	}

	end := res.end
	addOnTxt := ""
	ext := res.end
	if ext.SkipSymbol() && ext.SkipSingle(int(float64(begin.Sum(3))*3.5)) {
		var addOn upceanPartial
		if addOnDecode(&addOn, ext, 5) || addOnDecode(&addOn, ext, 2) {
			addOnTxt = addOn.txt
			end = addOn.end
			symbologyID = "]E3"
		}
	}

	// AllowedEANExtensions acts as zxing-cpp's EanAddOnSymbol::Require with
	// a fixed set of lengths.
	if opts != nil && len(opts.AllowedEANExtensions) > 0 {
		found := false
		for _, allowed := range opts.AllowedEANExtensions {
			if len(addOnTxt) == allowed {
				found = true
				break
			}
		}
		if !found {
			return nil, zxinggo.ErrNotFound
		}
	}

	result := zxinggo.NewResult(res.txt, nil, []zxinggo.ResultPoint{
		{X: float64(begin.PixelsInFront()), Y: float64(rowNumber)},
		{X: float64(end.PixelsTillEnd()), Y: float64(rowNumber)},
	}, res.format)
	result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, symbologyID)
	if addOnTxt != "" {
		result.PutMetadata(zxinggo.MetadataUPCEANExtension, addOnTxt)
		switch len(addOnTxt) {
		case 2:
			result.PutMetadata(zxinggo.MetadataIssueNumber, addOnTxt)
		case 5:
			// Suggested price, ISO 21061 / Java UPCEANExtension5Support:
			// 3,1 weights on the first four digits, check digit last.
			d0, d1, d2, d3, d4 := addOnTxt[0]-'0', addOnTxt[1]-'0', addOnTxt[2]-'0', addOnTxt[3]-'0', addOnTxt[4]-'0'
			if (10-(3*(d0+d2)+d1+d3)%10)%10 == d4 {
				result.PutMetadata(zxinggo.MetadataSuggestedPrice, "$"+addOnTxt[1:3]+"."+addOnTxt[3:5])
			}
		}
	}
	return result, nil
}

type upceanPartial struct {
	txt    string
	end    pattern.View
	format zxinggo.Format
}

func ean13Decode(res *upceanPartial, begin pattern.View) bool {
	mid := begin.SubView(27, len(middlePattern))
	end := begin.SubView(56, len(startEndPattern))

	if !pattern.IsRightGuard(end, startEndPattern, quietZoneRightEAN, 0) ||
		pattern.IsPattern(mid, middlePattern, math.MaxInt32, 0, 0) == 0 {
		return false
	}

	next := begin.SubView(len(startEndPattern), charLen)
	txt := " " // make space for the first-digit encoding character
	lgPattern := 0
	if !decodeDigits(6, &next, &txt, &lgPattern) {
		return false
	}
	next = next.SubView(len(middlePattern), charLen)
	if !decodeDigits(6, &next, &txt, nil) {
		return false
	}

	i := indexOfInt(ean13FirstDigitEncodings, lgPattern)
	if i == -1 {
		return false
	}
	txt = string(rune('0'+i)) + txt[1:]

	res.txt = txt
	res.end = end
	res.format = zxinggo.FormatEAN13
	return true
}

func plausibleDigitModuleSize(begin pattern.View, start, i int, moduleSizeRef float64) bool {
	moduleSizeData := float64(begin.SubView(start+i*4, 4).Sum(0)) / 7.0
	d := moduleSizeData/moduleSizeRef - 1
	if d < 0 {
		d = -d
	}
	return d < 0.2
}

func ean8Decode(res *upceanPartial, begin pattern.View) bool {
	mid := begin.SubView(19, len(middlePattern))
	end := begin.SubView(40, len(startEndPattern))

	if !pattern.IsRightGuard(end, startEndPattern, quietZoneRightEAN, 0) ||
		pattern.IsPattern(mid, middlePattern, math.MaxInt32, 0, 0) == 0 {
		return false
	}

	// Guard and data module sizes must agree (zxing-cpp plausibility check).
	moduleSizeGuard := float64(begin.Sum(0)+mid.Sum(0)+end.Sum(0)) / 11.0
	for _, start := range [2]int{3, 24} {
		for i := range 4 {
			if !plausibleDigitModuleSize(begin, start, i, moduleSizeGuard) {
				return false
			}
		}
	}

	next := begin.SubView(len(startEndPattern), charLen)
	txt := ""
	if !decodeDigits(4, &next, &txt, nil) {
		return false
	}
	next = next.SubView(len(middlePattern), charLen)
	if !decodeDigits(4, &next, &txt, nil) {
		return false
	}

	res.txt = txt
	res.end = end
	res.format = zxinggo.FormatEAN8
	return true
}

func upceDecode(res *upceanPartial, begin pattern.View) bool {
	end := begin.SubView(27, len(upceEndPattern))

	if !pattern.IsRightGuard(end, upceEndPattern, quietZoneRightUPC, 0) {
		return false
	}

	// Guard/data module size agreement also filters UPC-E false positives.
	moduleSizeGuard := float64(begin.Sum(0)+end.Sum(0)) / 9.0
	for i := range 6 {
		if !plausibleDigitModuleSize(begin, 3, i, moduleSizeGuard) {
			return false
		}
	}

	next := begin.SubView(len(startEndPattern), charLen)
	txt := " " // make space for the number system digit
	lgPattern := 0
	if !decodeDigits(6, &next, &txt, &lgPattern) {
		return false
	}

	i := indexOfInt(upceNumSysAndCheckDigitPatterns, lgPattern)
	if i == -1 {
		return false
	}
	txt = string(rune('0'+i/10)) + txt[1:] + string(rune('0'+i%10))

	res.txt = txt
	res.end = end
	res.format = zxinggo.FormatUPCE
	return true
}

func addOnDecode(res *upceanPartial, begin pattern.View, digitCount int) bool {
	ext := begin.SubView(0, 3+digitCount*4+(digitCount-1)*2)
	moduleSize := pattern.IsPattern(ext, extStartPattern, math.MaxInt32, 0, 0)
	if moduleSize == 0 || !ext.IsValidSize() {
		return false
	}
	if !ext.IsAtLastBar() && ext.At(ext.Size()) <= int(float64(quietZoneAddon)*moduleSize)-1 {
		return false
	}

	res.end = ext
	ext = ext.SubView(len(extStartPattern), charLen)
	lgPattern := 0
	txt := ""
	for i := range digitCount {
		if !decodeDigit(ext, &txt, &lgPattern) {
			return false
		}
		ext.SkipSymbol()
		if i < digitCount-1 {
			if pattern.IsPattern(ext, extSeparatorPattern, 0, 0, moduleSize) == 0 {
				return false
			}
			ext.SkipPair()
		}
	}

	if digitCount == 2 {
		n, err := strconv.Atoi(txt)
		if err != nil || n%4 != lgPattern {
			return false
		}
	} else if ean5Checksum(txt) != indexOfInt(addOnCheckEncodings, lgPattern) {
		return false
	}

	res.txt = txt
	res.format = zxinggo.FormatEAN13
	return true
}

func ean5Checksum(s string) int {
	sum := 0
	n := len(s)
	for i := n - 2; i >= 0; i -= 2 {
		sum += int(s[i] - '0')
	}
	sum *= 3
	for i := n - 1; i >= 0; i -= 2 {
		sum += int(s[i] - '0')
	}
	sum *= 3
	return sum % 10
}

func decodeDigits(digitCount int, next *pattern.View, txt *string, lgPattern *int) bool {
	for range digitCount {
		if !decodeDigit(*next, txt, lgPattern) {
			return false
		}
		next.SkipSymbol()
	}
	return true
}

// decodeDigit matches one 4-counter digit symbol; lgPattern collects the
// L/G parity bits when non-nil (Odd=0/Even=1 style, as in zxing-cpp).
func decodeDigit(view pattern.View, txt *string, lgPattern *int) bool {
	const (
		maxAvgVariance        = 0.48
		maxIndividualVariance = 0.7
	)
	bestMatch := -1
	if lgPattern != nil {
		bestMatch = pattern.DecodeDigit(view, upceLAndGPatterns, maxAvgVariance, maxIndividualVariance, false)
	} else {
		bestMatch = pattern.DecodeDigit(view, upceLPatterns, maxAvgVariance, maxIndividualVariance, false)
	}
	if bestMatch == -1 {
		return false
	}
	*txt += string(rune('0' + bestMatch%10))
	if lgPattern != nil {
		*lgPattern <<= 1
		if bestMatch >= 10 {
			*lgPattern |= 1
		}
	}
	return true
}

func indexOfInt(list []int, v int) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}

// Local aliases for the shared UPC/EAN guard patterns.
var (
	startEndPattern = UPCEANStartEndPattern
	middlePattern   = UPCEANMiddlePattern
)

// DecodeRow adapts a bit row to the pattern-row interface.
func (r *MultiUPCEANReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	return r.DecodePattern(rowNumber, pattern.NewView(bitRowToCounters(row, nil)), opts)
}
