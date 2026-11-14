package oned

import (
	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/pattern"
)

// MultiFormatOneDReader attempts to decode 1D barcodes by trying multiple
// format-specific readers in sequence.
type MultiFormatOneDReader struct {
	readers         []RowDecoder
	possibleFormats map[zxinggo.Format]bool
}

// NewMultiFormatOneDReader creates a new multi-format reader configured by opts.
func NewMultiFormatOneDReader(opts *zxinggo.DecodeOptions) *MultiFormatOneDReader {
	var readers []RowDecoder
	var possibleFormats map[zxinggo.Format]bool

	if opts != nil && len(opts.PossibleFormats) > 0 {
		possibleFormats = make(map[zxinggo.Format]bool)
		for _, f := range opts.PossibleFormats {
			possibleFormats[f] = true
		}
		// Single pattern-row reader for all UPC/EAN formats (zxing-cpp
		// MultiUPCEANReader); it also handles the UPC-A conversion.
		if possibleFormats[zxinggo.FormatEAN13] || possibleFormats[zxinggo.FormatUPCA] ||
			possibleFormats[zxinggo.FormatEAN8] || possibleFormats[zxinggo.FormatUPCE] {
			readers = append(readers, NewMultiUPCEANReader(possibleFormats))
		}
		if possibleFormats[zxinggo.FormatCode39] {
			useCheckDigit := opts.AssumeCode39CheckDigit
			readers = append(readers, NewCode39ReaderWithCheckDigit(useCheckDigit, true))
		}
		if possibleFormats[zxinggo.FormatCode93] {
			readers = append(readers, NewCode93Reader())
		}
		if possibleFormats[zxinggo.FormatCode128] {
			readers = append(readers, NewCode128Reader())
		}
		if possibleFormats[zxinggo.FormatITF] {
			readers = append(readers, NewITFReader())
		}
		if possibleFormats[zxinggo.FormatCodabar] {
			readers = append(readers, NewCodabarReader())
		}
		if possibleFormats[zxinggo.FormatRSSExpanded] {
			readers = append(readers, NewRSSExpandedReader())
		}
		if possibleFormats[zxinggo.FormatRSS14] {
			// Last: the DataBar reader consumes the whole row view, which
			// also stops the driver's TryHarder retry (zxing-cpp behavior).
			readers = append(readers, NewDataBarReader())
		}
	}

	if len(readers) == 0 {
		readers = []RowDecoder{
			NewMultiUPCEANReader(nil),
			NewCode39ReaderWithCheckDigit(false, true),
			NewCode93Reader(),
			NewCode128Reader(),
			NewITFReader(),
			NewCodabarReader(),
			NewRSSExpandedReader(),
			NewDataBarReader(),
		}
	}

	return &MultiFormatOneDReader{readers: readers, possibleFormats: possibleFormats}
}

// DecodeRow tries each reader in sequence until one succeeds.
// Includes Java-compatible EAN-13 → UPC-A conversion when UPC-A was requested.
func (r *MultiFormatOneDReader) DecodeRow(rowNumber int, row *bitutil.BitArray, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	for _, reader := range r.readers {
		result, err := reader.DecodeRow(rowNumber, row, opts)
		if err == nil {
			return r.maybeConvertEAN13ToUPCA(result), nil
		}
	}
	return nil, zxinggo.ErrNotFound
}

// maybeConvertEAN13ToUPCA converts an EAN-13 result starting with '0' to UPC-A
// if UPC-A was requested. Matches Java MultiFormatUPCEANReader behavior.
func (r *MultiFormatOneDReader) maybeConvertEAN13ToUPCA(result *zxinggo.Result) *zxinggo.Result {
	if result.Format != zxinggo.FormatEAN13 || len(result.Text) == 0 || result.Text[0] != '0' {
		return result
	}
	// Convert if UPC-A was requested, or if no format filter was set (default readers)
	if r.possibleFormats == nil || r.possibleFormats[zxinggo.FormatUPCA] {
		upcaResult := zxinggo.NewResult(result.Text[1:], nil, result.Points, zxinggo.FormatUPCA)
		for k, v := range result.Metadata {
			upcaResult.PutMetadata(k, v)
		}
		return upcaResult
	}
	return result
}

// Decode decodes a 1D barcode from the given image. Row/column scanning
// and orientation handling live in DecodeOneD (zxing-cpp ODReader).
func (r *MultiFormatOneDReader) Decode(image *zxinggo.BinaryBitmap, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	return DecodeOneD(image, r, opts)
}

// DecodePattern tries each sub-reader on a pattern-row view (zxing-cpp
// RowReader interface). Sub-readers that still consume bit rows get an
// adapted BitArray, built lazily.
func (r *MultiFormatOneDReader) DecodePattern(rowNumber int, view *pattern.View, opts *zxinggo.DecodeOptions) (*zxinggo.Result, error) {
	var row *bitutil.BitArray
	for _, reader := range r.readers {
		if prd, ok := reader.(PatternRowDecoder); ok {
			res, err := prd.DecodePattern(rowNumber, view, opts)
			if err == nil && res != nil {
				return r.maybeConvertEAN13ToUPCA(res), nil
			}
			continue
		}
		if row == nil {
			row = countersToBitArray(view.Row(), row)
		}
		res, err := reader.DecodeRow(rowNumber, row, opts)
		if err == nil && res != nil {
			return r.maybeConvertEAN13ToUPCA(res), nil
		}
	}
	return nil, zxinggo.ErrNotFound
}

// Reset resets sub-readers that keep cross-row state (RSS readers),
// matching zxing-cpp's reset between scan passes.
func (r *MultiFormatOneDReader) Reset() {
	for _, reader := range r.readers {
		if rst, ok := reader.(interface{ Reset() }); ok {
			rst.Reset()
		}
	}
}
