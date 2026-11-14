package decoder

import (
	"sort"
	"testing"
)

// TestBarcodeValueValueAscending locks the ordering of tied-max codeword values.
// Value() draws from a map whose iteration order Go randomizes; the ambiguity
// resolver enumerates candidate combinations as an odometer over these slices
// within a bounded number of tries, so the order is observable and must be
// stable. Ascending order matches the reference implementation (value-indexed
// count array) and keeps decoding deterministic.
func TestBarcodeValueValueAscending(t *testing.T) {
	tied := []int{900, 3, 250, 47, 610}
	for range 100 {
		bv := NewBarcodeValue()
		for _, v := range tied {
			bv.SetValue(v) // each value once -> all tie at confidence 1
		}
		got := bv.Value()
		if len(got) != len(tied) {
			t.Fatalf("Value() returned %d values, want %d", len(got), len(tied))
		}
		if !sort.IntsAreSorted(got) {
			t.Fatalf("Value() not ascending: %v", got)
		}
	}
}

// TestBarcodeValueValueMaxConfidence ensures Value() returns only the
// highest-confidence values, regardless of insertion order.
func TestBarcodeValueValueMaxConfidence(t *testing.T) {
	bv := NewBarcodeValue()
	bv.SetValue(10)
	bv.SetValue(20)
	bv.SetValue(20) // 20 wins with confidence 2
	bv.SetValue(30)
	bv.SetValue(30)
	if got := bv.Value(); len(got) != 2 || got[0] != 20 || got[1] != 30 {
		t.Fatalf("Value() = %v, want [20 30]", got)
	}
}
