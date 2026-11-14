package zxinggo

import (
	"fmt"
	"reflect"
	"sync"
)

// MultiFormatReader is a factory/dispatcher that selects appropriate Reader
// implementations based on format hints and tries them in sequence.
// Readers are constructed per Decode call, so a single MultiFormatReader
// instance is safe for concurrent use and honors each call's options.
type MultiFormatReader struct{}

// NewMultiFormatReader creates a new multi-format reader. If opts specifies
// PossibleFormats, only those formats are tried. Otherwise all formats are tried.
func NewMultiFormatReader() *MultiFormatReader {
	return &MultiFormatReader{}
}

// Decode attempts to decode a barcode from the given image using all registered
// format readers.
func (r *MultiFormatReader) Decode(image *BinaryBitmap, opts *DecodeOptions) (*Result, error) {
	readers := buildReaders(opts)
	for _, reader := range readers {
		result, err := reader.Decode(image, opts)
		if err == nil {
			return result, nil
		}
	}
	if opts != nil && opts.AlsoInverted {
		// Retry against an inverted copy of the image. The original bitmap and
		// its cached matrices are left untouched; the inverted source feeds a
		// fresh binarizer/bitmap so both 1D (BlackRow) and 2D (BlackMatrix)
		// readers see inverted luminance.
		source := image.binarizer.LuminanceSource()
		if _, alreadyInverted := source.(*InvertedLuminanceSource); !alreadyInverted {
			if bin, err := NewBinarizerFromSource(image.binarizer, NewInvertedLuminanceSource(source)); err == nil {
				inverted := NewBinaryBitmap(bin)
				for _, reader := range readers {
					result, err := reader.Decode(inverted, opts)
					if err == nil {
						return result, nil
					}
				}
			}
		}
	}
	return nil, ErrNotFound
}

// DecodeWithFormat attempts to decode a barcode of the given format.
// The caller's opts is never modified.
func (r *MultiFormatReader) DecodeWithFormat(image *BinaryBitmap, format Format, opts *DecodeOptions) (*Result, error) {
	var local DecodeOptions
	if opts != nil {
		local = *opts
	}
	local.PossibleFormats = []Format{format}
	readers := buildReaders(&local)
	for _, reader := range readers {
		result, err := reader.Decode(image, &local)
		if err == nil {
			return result, nil
		}
	}
	return nil, fmt.Errorf("no barcode of format %s found: %w", format, ErrNotFound)
}

// Reset is retained for API compatibility; readers are constructed per Decode
// call and hold no state between calls.
func (r *MultiFormatReader) Reset() {}

// readerFactory is a function that creates a Reader. This is used as an
// extension point so format-specific packages can register themselves.
type readerFactory func(opts *DecodeOptions) Reader

var (
	registryMu      sync.RWMutex
	readerFactories = map[Format]readerFactory{}
	// readerOrder records registration order so buildReaders is deterministic.
	readerOrder []Format
)

// RegisterReader registers a reader factory for the given format. This should
// be called from an init() function in format-specific packages.
func RegisterReader(format Format, factory readerFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := readerFactories[format]; !exists {
		readerOrder = append(readerOrder, format)
	}
	readerFactories[format] = factory
}

// buildReaders creates readers based on the options. Formats are tried in
// registration order, or in PossibleFormats order when specified. Factories
// that are the same function value (e.g. the single 1D factory registered
// under every 1D format) are instantiated only once per call.
func buildReaders(opts *DecodeOptions) []Reader {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var readers []Reader
	seen := make(map[uintptr]bool)
	add := func(factory readerFactory) {
		p := reflect.ValueOf(factory).Pointer()
		if seen[p] {
			return
		}
		seen[p] = true
		readers = append(readers, factory(opts))
	}

	if opts != nil && len(opts.PossibleFormats) > 0 {
		for _, f := range opts.PossibleFormats {
			if factory, ok := readerFactories[f]; ok {
				add(factory)
			}
		}
	}

	if len(readers) == 0 {
		// Try all registered readers in deterministic registration order.
		for _, f := range readerOrder {
			add(readerFactories[f])
		}
	}

	return readers
}
