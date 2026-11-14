package decoder

import "sort"

// BarcodeValue tracks occurrences of codeword values and determines which
// value(s) have the highest confidence (most occurrences).
type BarcodeValue struct {
	values map[int]int
}

// NewBarcodeValue creates a new BarcodeValue.
func NewBarcodeValue() *BarcodeValue {
	return &BarcodeValue{
		values: make(map[int]int),
	}
}

// SetValue adds an occurrence of a value, incrementing its confidence count.
func (bv *BarcodeValue) SetValue(value int) {
	bv.values[value] = bv.values[value] + 1
}

// Value returns all values with the maximum occurrence count.
// Returns an empty slice if no values have been set.
func (bv *BarcodeValue) Value() []int {
	maxConfidence := -1
	var result []int
	for key, conf := range bv.values {
		if conf > maxConfidence {
			maxConfidence = conf
			result = []int{key}
		} else if conf == maxConfidence {
			result = append(result, key)
		}
	}
	// Iterate tied values in ascending order. The map iteration order is
	// randomized in Go, but the ambiguity resolver (createDecoderResultFromAmbiguousValues)
	// enumerates combinations as an odometer over these slices within a bounded
	// number of tries, so the order is observable. Ascending order matches the
	// reference implementation (which stores counts in a value-indexed array)
	// and makes decoding deterministic.
	sort.Ints(result)
	return result
}

// Confidence returns the confidence (occurrence count) for the given value,
// or 0 if the value has not been set.
func (bv *BarcodeValue) Confidence(value int) int {
	return bv.values[value]
}
