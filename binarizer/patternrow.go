package binarizer

import zxinggo "github.com/fabiomarini/zxinggo"

// RowScanner extracts run-length pattern rows from a LuminanceSource using
// the global-histogram black point with sharpening, ported from zxing-cpp
// GlobalHistogramBinarizer::getPatternRow. It owns scratch buffers, so one
// scanner must not be used concurrently.
type RowScanner struct {
	source   zxinggo.LuminanceSource
	lum      []byte
	bin      []byte
	buckets  [luminanceBuckets]int
	counters []int
	matrix   []byte // lazily cached, used for columns when source lacks Column
	colBuf   []byte
}

// columnSource is optionally implemented by luminance sources that can
// expose a column without copying the whole image.
type columnSource interface {
	Column(x int, buf []byte) []byte
}

// NewRowScanner creates a RowScanner for the given luminance source.
func NewRowScanner(source zxinggo.LuminanceSource) *RowScanner {
	w, h := source.Width(), source.Height()
	return &RowScanner{
		source:   source,
		lum:      make([]byte, max(w, h)),
		bin:      make([]byte, max(w, h)),
		counters: make([]int, 0, max(w, h)+2),
	}
}

// Row returns the run-length counters for luminance row y. The first entry
// is the white run in front of the first bar. The result is valid until the
// next call on the same scanner.
func (rs *RowScanner) Row(y int) ([]int, error) {
	lum := rs.source.Row(y, rs.lum)
	if lum == nil {
		return nil, zxinggo.ErrNotFound
	}
	return rs.patternLine(lum)
}

// Column returns the run-length counters for luminance column x, scanned
// bottom-to-top (matching a 90° clockwise rotation of the image, as
// zxing-cpp's ImageView::rotated(90) does).
func (rs *RowScanner) Column(x int) ([]int, error) {
	h := rs.source.Height()
	var col []byte
	if cs, ok := rs.source.(columnSource); ok {
		col = cs.Column(x, rs.colBuf)
		rs.colBuf = col
		// read bottom-to-top in place
		for i := range h / 2 {
			col[i], col[h-1-i] = col[h-1-i], col[i]
		}
	} else {
		if rs.matrix == nil {
			rs.matrix = rs.source.Matrix()
		}
		w := rs.source.Width()
		if rs.colBuf == nil || len(rs.colBuf) < h {
			rs.colBuf = make([]byte, h)
		}
		col = rs.colBuf[:h]
		for y := range h {
			col[y] = rs.matrix[(h-1-y)*w+x]
		}
	}
	if col == nil {
		return nil, zxinggo.ErrNotFound
	}
	return rs.patternLine(col)
}

// patternLine thresholds one luminance line (histogram black point minus one
// with sharpening) and run-length encodes it. Ported from zxing-cpp
// ThresholdSharpened + GetPatternRow.
func (rs *RowScanner) patternLine(lum []byte) ([]int, error) {
	width := len(lum)
	if width < 3 {
		return nil, zxinggo.ErrNotFound
	}

	for i := range rs.buckets {
		rs.buckets[i] = 0
	}
	for _, p := range lum {
		rs.buckets[int(p)>>luminanceShift]++
	}
	blackPoint, err := estimateBlackPoint(rs.buckets[:])
	if err != nil {
		return nil, err
	}
	threshold := blackPoint - 1
	if threshold <= 0 {
		return nil, zxinggo.ErrNotFound
	}

	bin := rs.bin[:width]
	bin[0] = boolByte(int(lum[0]) <= threshold)
	for x := 1; x < width-1; x++ {
		sharpened := (-int(lum[x-1]) + int(lum[x])*4 - int(lum[x+1])) / 2
		bin[x] = boolByte(sharpened <= threshold)
	}
	bin[width-1] = boolByte(int(lum[width-1]) <= threshold)

	return runLengths(bin, rs.counters), nil
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// runLengths encodes a binarized line as run lengths starting with the
// white run in front of the first bar (zxing-cpp GetPatternRow).
func runLengths(bin []byte, buf []int) []int {
	buf = buf[:0]
	i := 0
	if bin[0] != 0 {
		buf = append(buf, 0)
	} else {
		for i < len(bin) && bin[i] == 0 {
			i++
		}
		buf = append(buf, i)
	}
	for i < len(bin) {
		s := i
		for i < len(bin) && bin[i] != 0 {
			i++
		}
		buf = append(buf, i-s)
		if i == len(bin) {
			buf = append(buf, 0)
			break
		}
		s = i
		for i < len(bin) && bin[i] == 0 {
			i++
		}
		buf = append(buf, i-s)
	}
	return buf
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
