// Package datamatrix provides Data Matrix (ECC-200) reading and writing.
package datamatrix

import (
	"fmt"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/datamatrix/decoder"
	"github.com/fabiomarini/zxinggo/datamatrix/detector"
)

// Reader decodes Data Matrix barcodes from binary images.
type Reader struct {
	dec *decoder.Decoder
}

// NewReader creates a new Data Matrix Reader.
func NewReader() *Reader {
	return &Reader{
		dec: decoder.NewDecoder(),
	}
}

// Decode locates and decodes a Data Matrix barcode in the given image.
func (r *Reader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	if opts == nil {
		opts = &zxinggo.DecodeOptions{}
	}

	matrix, err := image.BlackMatrix()
	if err != nil {
		return nil, err
	}

	if opts.PureBarcode {
		bits, err := extractPureBits(matrix)
		if err != nil {
			return nil, err
		}
		dr, err := r.dec.Decode(bits)
		if err != nil {
			return nil, err
		}
		result := zxinggo.NewResult(dr.Text, dr.RawBytes, nil, zxinggo.FormatDataMatrix)
		result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, fmt.Sprintf("]d%d", dr.SymbologyModifier))
		result.PutMetadata(zxinggo.MetadataErrorsCorrected, dr.ErrorsCorrected)
		return result, nil
	}

	// Detection attempts: center seed first, then the black-pixel bounding
	// box center (handles off-center symbols and center-seeded rectangles
	// that freeze inside the symbol), and under TryHarder a coarse 3×3 grid
	// of seeds over the bounding box. A geometrically successful detection
	// can still produce a grid that fails to decode, so every candidate is
	// passed to the decoder and the first decoded one wins.
	seeds := []detectSeed{{x: matrix.Width() / 2, y: matrix.Height() / 2, initSize: 10}}
	if bl, bt, bw, bh, ok := detector.FindBoundingBox(matrix, 10); ok {
		seeds = append(seeds, detectSeed{x: bl + bw/2, y: bt + bh/2, initSize: max(10, min(bw, bh)/4)})
		if opts.TryHarder {
			for _, fy := range []int{1, 2, 3} {
				for _, fx := range []int{1, 2, 3} {
					s := detectSeed{x: bl + bw*fx/4, y: bt + bh*fy/4, initSize: 10}
					if !nearSeed(seeds, s, 10) {
						seeds = append(seeds, s)
					}
				}
			}
		}
	}

	var lastErr error = zxinggo.ErrNotFound
	for _, s := range seeds {
		detResult, err := detector.DetectFrom(matrix, s.x, s.y, s.initSize)
		if err == nil {
			if result, ok := r.tryDecode(detResult); ok {
				return result, nil
			}
		} else {
			lastErr = err
		}
		variants, err := detector.DetectRefinedVariants(matrix, s.x, s.y, s.initSize)
		if err != nil {
			lastErr = err
			continue
		}
		for _, v := range variants {
			if result, ok := r.tryDecode(v); ok {
				return result, nil
			}
		}
	}
	return nil, lastErr
}

// tryDecode runs the decoder on a detection result.
func (r *Reader) tryDecode(detResult *detector.DetectorResult) (*zxinggo.Result, bool) {
	dr, err := r.dec.Decode(detResult.Bits)
	if err != nil {
		return nil, false
	}
	result := zxinggo.NewResult(dr.Text, dr.RawBytes, detResult.Points, zxinggo.FormatDataMatrix)
	result.PutMetadata(zxinggo.MetadataSymbologyIdentifier, fmt.Sprintf("]d%d", dr.SymbologyModifier))
	result.PutMetadata(zxinggo.MetadataErrorsCorrected, dr.ErrorsCorrected)
	return result, true
}

// detectSeed is one WhiteRectangleDetector seeding attempt.
type detectSeed struct {
	x, y, initSize int
}

// nearSeed reports whether s is within dist pixels (in each axis) of any
// seed in tried.
func nearSeed(tried []detectSeed, s detectSeed, dist int) bool {
	for _, t := range tried {
		if abs(t.x-s.x) <= dist && abs(t.y-s.y) <= dist {
			return true
		}
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Reset resets internal state.
func (r *Reader) Reset() {}

// extractPureBits extracts a Data Matrix from a "pure" image — one that
// contains only the unrotated, unskewed barcode with some white border.
func extractPureBits(image *bitutil.BitMatrix) (*bitutil.BitMatrix, error) {
	leftTopBlack := image.TopLeftOnBit()
	rightBottomBlack := image.BottomRightOnBit()
	if leftTopBlack == nil || rightBottomBlack == nil {
		return nil, zxinggo.ErrNotFound
	}

	moduleSize, err := moduleSizePure(leftTopBlack, image)
	if err != nil {
		return nil, err
	}

	top := leftTopBlack[1]
	bottom := rightBottomBlack[1]
	left := leftTopBlack[0]
	right := rightBottomBlack[0]

	matrixWidth := (right - left + 1) / moduleSize
	matrixHeight := (bottom - top + 1) / moduleSize
	if matrixWidth <= 0 || matrixHeight <= 0 {
		return nil, zxinggo.ErrNotFound
	}

	// Nudge to the center of each module
	nudge := moduleSize / 2

	bits := bitutil.NewBitMatrixWithSize(matrixWidth, matrixHeight)
	for y := 0; y < matrixHeight; y++ {
		iOffset := top + y*moduleSize + nudge
		for x := 0; x < matrixWidth; x++ {
			if image.Get(left+x*moduleSize+nudge, iOffset) {
				bits.Set(x, y)
			}
		}
	}
	return bits, nil
}

func moduleSizePure(leftTopBlack []int, image *bitutil.BitMatrix) (int, error) {
	width := image.Width()
	x := leftTopBlack[0]
	y := leftTopBlack[1]

	// Walk right along the top edge to find the module size
	for x < width && image.Get(x, y) {
		x++
	}
	if x == width {
		return 0, zxinggo.ErrNotFound
	}

	moduleSize := x - leftTopBlack[0]
	if moduleSize == 0 {
		return 0, zxinggo.ErrNotFound
	}
	return moduleSize, nil
}

// Compile-time check.
var _ zxinggo.Reader = (*Reader)(nil)
