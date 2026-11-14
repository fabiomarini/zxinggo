// Package zxinggo is a pure Go port of the ZXing barcode library.
package zxinggo

import (
	"fmt"
	"math"
	"time"

	"github.com/fabiomarini/zxinggo/bitutil"
)

// Format represents a barcode format.
type Format int

const (
	FormatQRCode Format = iota
	FormatPDF417
	FormatCode128
	FormatCode39
	FormatEAN13
	FormatEAN8
	FormatUPCA
	FormatUPCE
	FormatITF
	FormatCodabar
	FormatDataMatrix
	FormatAztec
	FormatRSS14
	FormatRSSExpanded
	FormatMaxiCode
	FormatCode93
)

// String returns the name of the barcode format.
func (f Format) String() string {
	switch f {
	case FormatQRCode:
		return "QR_CODE"
	case FormatPDF417:
		return "PDF_417"
	case FormatCode128:
		return "CODE_128"
	case FormatCode39:
		return "CODE_39"
	case FormatEAN13:
		return "EAN_13"
	case FormatEAN8:
		return "EAN_8"
	case FormatUPCA:
		return "UPC_A"
	case FormatUPCE:
		return "UPC_E"
	case FormatITF:
		return "ITF"
	case FormatCodabar:
		return "CODABAR"
	case FormatDataMatrix:
		return "DATA_MATRIX"
	case FormatAztec:
		return "AZTEC"
	case FormatRSS14:
		return "RSS_14"
	case FormatRSSExpanded:
		return "RSS_EXPANDED"
	case FormatMaxiCode:
		return "MAXICODE"
	case FormatCode93:
		return "CODE_93"
	default:
		return "UNKNOWN"
	}
}

// ResultMetadataKey identifies a type of metadata about a barcode result.
type ResultMetadataKey int

const (
	MetadataOther ResultMetadataKey = iota
	MetadataOrientation
	MetadataByteSegments
	MetadataErrorCorrectionLevel
	MetadataErrorsCorrected
	MetadataErasuresCorrected
	MetadataIssueNumber
	MetadataSuggestedPrice
	MetadataPossibleCountry
	MetadataUPCEANExtension
	MetadataPDF417ExtraMetadata
	MetadataStructuredAppendSequence
	MetadataStructuredAppendParity
	MetadataSymbologyIdentifier
)

// ResultPoint represents a point of interest in an image.
type ResultPoint struct {
	X, Y float64
}

// Distance returns the distance between two points.
func Distance(a, b ResultPoint) float64 {
	return math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y))
}

// CrossProductZ computes the z component of the cross product between vectors
// (bX-aX, bY-aY) and (cX-aX, cY-aY).
func CrossProductZ(a, b, c ResultPoint) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

// Result encapsulates the result of decoding a barcode.
type Result struct {
	Text      string
	RawBytes  []byte
	NumBits   int
	Points    []ResultPoint
	Format    Format
	Metadata  map[ResultMetadataKey]interface{}
	Timestamp time.Time
}

// NewResult creates a new Result with the given text, format, and points.
func NewResult(text string, rawBytes []byte, points []ResultPoint, format Format) *Result {
	numBits := 0
	if rawBytes != nil {
		numBits = 8 * len(rawBytes)
	}
	return &Result{
		Text:      text,
		RawBytes:  rawBytes,
		NumBits:   numBits,
		Points:    points,
		Format:    format,
		Metadata:  make(map[ResultMetadataKey]interface{}),
		Timestamp: time.Now(),
	}
}

// PutMetadata adds a metadata key/value pair. The Metadata map is lazily
// created, so Results built as struct literals are safe to annotate.
func (r *Result) PutMetadata(key ResultMetadataKey, value interface{}) {
	if r.Metadata == nil {
		r.Metadata = make(map[ResultMetadataKey]interface{})
	}
	r.Metadata[key] = value
}

// AddResultPoints appends additional result points.
func (r *Result) AddResultPoints(points []ResultPoint) {
	r.Points = append(r.Points, points...)
}

// BinaryBitmap represents a bitmap of binary (black/white) values.
type BinaryBitmap struct {
	binarizer Binarizer
	matrix    *bitutil.BitMatrix
}

// NewBinaryBitmap creates a new BinaryBitmap from the given Binarizer.
func NewBinaryBitmap(binarizer Binarizer) *BinaryBitmap {
	return &BinaryBitmap{binarizer: binarizer}
}

// Width returns the width of the bitmap.
func (b *BinaryBitmap) Width() int {
	return b.binarizer.Width()
}

// Height returns the height of the bitmap.
func (b *BinaryBitmap) Height() int {
	return b.binarizer.Height()
}

// Binarizer returns the underlying binarizer.
func (b *BinaryBitmap) Binarizer() Binarizer {
	return b.binarizer
}

// BlackRow returns a row of black/white values.
func (b *BinaryBitmap) BlackRow(y int, row *bitutil.BitArray) (*bitutil.BitArray, error) {
	return b.binarizer.BlackRow(y, row)
}

// BlackMatrix returns the 2D matrix of black/white values.
func (b *BinaryBitmap) BlackMatrix() (*bitutil.BitMatrix, error) {
	if b.matrix != nil {
		return b.matrix, nil
	}
	m, err := b.binarizer.BlackMatrix()
	if err != nil {
		return nil, err
	}
	b.matrix = m
	return m, nil
}

// Crop returns a new BinaryBitmap representing a rectangular sub-region.
// It returns an error if the source does not support cropping or the region
// is out of bounds.
func (b *BinaryBitmap) Crop(left, top, width, height int) (*BinaryBitmap, error) {
	source, ok := b.binarizer.LuminanceSource().(croppableSource)
	if !ok {
		return nil, fmt.Errorf("crop: source %T does not support cropping", b.binarizer.LuminanceSource())
	}
	cropped := source.Crop(left, top, width, height)
	if cropped == nil {
		return nil, fmt.Errorf("crop: region (%d,%d %dx%d) out of bounds", left, top, width, height)
	}
	bin, err := NewBinarizerFromSource(b.binarizer, cropped)
	if err != nil {
		return nil, err
	}
	return NewBinaryBitmap(bin), nil
}

// RotateCounterClockwise returns a new BinaryBitmap rotated 90 degrees CCW.
// It returns an error if the source or binarizer does not support rotation.
func (b *BinaryBitmap) RotateCounterClockwise() (*BinaryBitmap, error) {
	source, ok := b.binarizer.LuminanceSource().(rotatableSource)
	if !ok {
		return nil, fmt.Errorf("rotate: source %T does not support rotation", b.binarizer.LuminanceSource())
	}
	rotated := source.RotateCounterClockwise()
	if rotated == nil {
		return nil, fmt.Errorf("rotate: source %T failed to rotate", b.binarizer.LuminanceSource())
	}
	bin, err := NewBinarizerFromSource(b.binarizer, rotated)
	if err != nil {
		return nil, err
	}
	return NewBinaryBitmap(bin), nil
}

// NewBinarizerFromSource creates a new binarizer of the same type with a new
// source. This is a factory method to support rotation and inversion. It
// returns an error when the binarizer type cannot be re-created.
func NewBinarizerFromSource(template Binarizer, source LuminanceSource) (Binarizer, error) {
	if factory, ok := template.(BinarizerFactory); ok {
		return factory.CreateBinarizer(source), nil
	}
	return nil, fmt.Errorf("binarizer %T does not support re-creation with a new source", template)
}

// BinarizerFactory is an interface for binarizers that can create new instances
// with a different LuminanceSource.
type BinarizerFactory interface {
	CreateBinarizer(source LuminanceSource) Binarizer
}
