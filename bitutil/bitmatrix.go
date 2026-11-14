package bitutil

import (
	"fmt"
	"math/bits"
	"strings"
)

// BitMatrix represents a 2D matrix of bits.
// x is the column position, y is the row position. The origin is at the top-left.
type BitMatrix struct {
	width   int
	height  int
	rowSize int
	data    []uint32
}

// NewBitMatrix creates a new square BitMatrix with the given dimension.
func NewBitMatrix(dimension int) *BitMatrix {
	return NewBitMatrixWithSize(dimension, dimension)
}

// NewBitMatrixWithSize creates a new BitMatrix with the given width and height.
func NewBitMatrixWithSize(width, height int) *BitMatrix {
	if width < 1 || height < 1 {
		panic("bitmatrix: dimensions must be greater than 0")
	}
	rowSize := (width + 31) / 32
	return &BitMatrix{
		width:   width,
		height:  height,
		rowSize: rowSize,
		data:    make([]uint32, rowSize*height),
	}
}

// newBitMatrixFromData creates a BitMatrix from existing data.
func newBitMatrixFromData(width, height, rowSize int, data []uint32) *BitMatrix {
	return &BitMatrix{width: width, height: height, rowSize: rowSize, data: data}
}

// ParseBoolMatrix creates a BitMatrix from a 2D boolean array. It returns an
// error when the array is empty.
func ParseBoolMatrix(image [][]bool) (*BitMatrix, error) {
	height := len(image)
	if height == 0 || len(image[0]) == 0 {
		return nil, fmt.Errorf("bitmatrix: cannot parse empty image")
	}
	width := len(image[0])
	bm := NewBitMatrixWithSize(width, height)
	for i := range height {
		for j := range width {
			if image[i][j] {
				bm.Set(j, i)
			}
		}
	}
	return bm, nil
}

// Get returns true if the bit at (x, y) is set.
func (bm *BitMatrix) Get(x, y int) bool {
	offset := y*bm.rowSize + x/32
	return (bm.data[offset]>>uint(x&0x1f))&1 != 0
}

// Set sets the bit at (x, y).
func (bm *BitMatrix) Set(x, y int) {
	offset := y*bm.rowSize + x/32
	bm.data[offset] |= 1 << uint(x&0x1f)
}

// Unset clears the bit at (x, y).
func (bm *BitMatrix) Unset(x, y int) {
	offset := y*bm.rowSize + x/32
	bm.data[offset] &^= 1 << uint(x&0x1f)
}

// Flip flips the bit at (x, y).
func (bm *BitMatrix) Flip(x, y int) {
	offset := y*bm.rowSize + x/32
	bm.data[offset] ^= 1 << uint(x&0x1f)
}

// FlipAll flips every bit in the matrix.
func (bm *BitMatrix) FlipAll() {
	for i := range bm.data {
		bm.data[i] = ^bm.data[i]
	}
}

// Clear clears all bits.
func (bm *BitMatrix) Clear() {
	for i := range bm.data {
		bm.data[i] = 0
	}
}

// SetRegion sets a rectangular region of bits. It returns an error when the
// region is invalid or does not fit inside the matrix.
func (bm *BitMatrix) SetRegion(left, top, width, height int) error {
	if top < 0 || left < 0 {
		return fmt.Errorf("bitmatrix: left and top must be nonnegative")
	}
	if height < 1 || width < 1 {
		return fmt.Errorf("bitmatrix: height and width must be at least 1")
	}
	right := left + width
	bottom := top + height
	if bottom > bm.height || right > bm.width {
		return fmt.Errorf("bitmatrix: region must fit inside the matrix")
	}
	// Fill whole words at once; only the first and last word need masking.
	firstWord := left >> 5
	lastWord := (right - 1) >> 5
	firstMask := ^uint32(0) << uint(left&0x1f)
	lastMask := ^uint32(0) >> uint(31-((right-1)&0x1f))
	for y := top; y < bottom; y++ {
		offset := y * bm.rowSize
		if firstWord == lastWord {
			bm.data[offset+firstWord] |= firstMask & lastMask
			continue
		}
		bm.data[offset+firstWord] |= firstMask
		for xw := firstWord + 1; xw < lastWord; xw++ {
			bm.data[offset+xw] = ^uint32(0)
		}
		bm.data[offset+lastWord] |= lastMask
	}
	return nil
}

// Row returns a row as a BitArray. If row is nil or too small, a new one is allocated.
func (bm *BitMatrix) Row(y int, row *BitArray) *BitArray {
	if row == nil || row.Size() < bm.width {
		row = NewBitArray(bm.width)
	} else {
		row.Clear()
	}
	offset := y * bm.rowSize
	for x := 0; x < bm.rowSize; x++ {
		row.SetBulk(x*32, bm.data[offset+x])
	}
	return row
}

// SetRow sets the row at y from the given BitArray. It returns an error when
// y is out of range or the row is too small for the matrix.
func (bm *BitMatrix) SetRow(y int, row *BitArray) error {
	if y < 0 || y >= bm.height {
		return fmt.Errorf("bitmatrix: row index %d out of range [0, %d)", y, bm.height)
	}
	if row == nil || len(row.BitData()) < bm.rowSize {
		return fmt.Errorf("bitmatrix: row data too small for matrix row size %d", bm.rowSize)
	}
	copy(bm.data[y*bm.rowSize:y*bm.rowSize+bm.rowSize], row.BitData()[:bm.rowSize])
	return nil
}

// Rotate rotates the matrix by the given degrees (0, 90, 180, 270). It returns
// an error when degrees is not a multiple of 90.
func (bm *BitMatrix) Rotate(degrees int) error {
	switch degrees % 360 {
	case 0:
		return nil
	case 90:
		bm.Rotate90()
	case 180:
		bm.Rotate180()
	case 270:
		bm.Rotate90()
		bm.Rotate180()
	default:
		return fmt.Errorf("bitmatrix: degrees must be a multiple of 90")
	}
	return nil
}

// Rotate180 rotates the matrix 180 degrees.
func (bm *BitMatrix) Rotate180() {
	topRow := NewBitArray(bm.width)
	bottomRow := NewBitArray(bm.width)
	maxHeight := (bm.height + 1) / 2
	for i := 0; i < maxHeight; i++ {
		topRow = bm.Row(i, topRow)
		bottomRowIndex := bm.height - 1 - i
		bottomRow = bm.Row(bottomRowIndex, bottomRow)
		topRow.Reverse()
		bottomRow.Reverse()
		_ = bm.SetRow(i, bottomRow)
		_ = bm.SetRow(bottomRowIndex, topRow)
	}
}

// Rotate90 rotates the matrix 90 degrees counterclockwise.
func (bm *BitMatrix) Rotate90() {
	newWidth := bm.height
	newHeight := bm.width
	newRowSize := (newWidth + 31) / 32
	newData := make([]uint32, newRowSize*newHeight)

	for y := 0; y < bm.height; y++ {
		for x := 0; x < bm.width; x++ {
			offset := y*bm.rowSize + x/32
			if (bm.data[offset]>>uint(x&0x1f))&1 != 0 {
				newOffset := (newHeight-1-x)*newRowSize + y/32
				newData[newOffset] |= 1 << uint(y&0x1f)
			}
		}
	}
	bm.width = newWidth
	bm.height = newHeight
	bm.rowSize = newRowSize
	bm.data = newData
}

// EnclosingRectangle returns [left, top, width, height] of the enclosing
// rectangle of all set bits, or nil if all bits are unset.
func (bm *BitMatrix) EnclosingRectangle() []int {
	left := bm.width
	top := bm.height
	right := -1
	bottom := -1

	for y := 0; y < bm.height; y++ {
		for x32 := 0; x32 < bm.rowSize; x32++ {
			theBits := bm.data[y*bm.rowSize+x32]
			if theBits != 0 {
				if y < top {
					top = y
				}
				if y > bottom {
					bottom = y
				}
				if x32*32 < left {
					bit := 0
					for (theBits << uint(31-bit)) == 0 {
						bit++
					}
					if x32*32+bit < left {
						left = x32*32 + bit
					}
				}
				if x32*32+31 > right {
					bit := 31
					for (theBits >> uint(bit)) == 0 {
						bit--
					}
					if x32*32+bit > right {
						right = x32*32 + bit
					}
				}
			}
		}
	}

	if right < left || bottom < top {
		return nil
	}
	return []int{left, top, right - left + 1, bottom - top + 1}
}

// TopLeftOnBit returns the [x, y] of the top-left set bit, or nil if none are set.
func (bm *BitMatrix) TopLeftOnBit() []int {
	bitsOffset := 0
	for bitsOffset < len(bm.data) && bm.data[bitsOffset] == 0 {
		bitsOffset++
	}
	if bitsOffset == len(bm.data) {
		return nil
	}
	y := bitsOffset / bm.rowSize
	x := (bitsOffset % bm.rowSize) * 32
	theBits := bm.data[bitsOffset]
	x += bits.TrailingZeros32(theBits)
	return []int{x, y}
}

// BottomRightOnBit returns the [x, y] of the bottom-right set bit, or nil if none are set.
func (bm *BitMatrix) BottomRightOnBit() []int {
	bitsOffset := len(bm.data) - 1
	for bitsOffset >= 0 && bm.data[bitsOffset] == 0 {
		bitsOffset--
	}
	if bitsOffset < 0 {
		return nil
	}
	y := bitsOffset / bm.rowSize
	x := (bitsOffset % bm.rowSize) * 32
	theBits := bm.data[bitsOffset]
	x += 31 - bits.LeadingZeros32(theBits)
	return []int{x, y}
}

// Width returns the width.
func (bm *BitMatrix) Width() int { return bm.width }

// Height returns the height.
func (bm *BitMatrix) Height() int { return bm.height }

// RowSize returns the row size in uint32 units.
func (bm *BitMatrix) RowSize() int { return bm.rowSize }

// Clone returns a deep copy of the BitMatrix.
func (bm *BitMatrix) Clone() *BitMatrix {
	d := make([]uint32, len(bm.data))
	copy(d, bm.data)
	return newBitMatrixFromData(bm.width, bm.height, bm.rowSize, d)
}

// String returns a string representation using "X " for set and "  " for unset.
func (bm *BitMatrix) String() string {
	return bm.StringWithChars("X ", "  ")
}

// StringWithChars returns a string representation using the given set/unset strings.
func (bm *BitMatrix) StringWithChars(setString, unsetString string) string {
	var sb strings.Builder
	sb.Grow(bm.height * (bm.width + 1))
	for y := 0; y < bm.height; y++ {
		for x := 0; x < bm.width; x++ {
			if bm.Get(x, y) {
				sb.WriteString(setString)
			} else {
				sb.WriteString(unsetString)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Equals returns true if two BitMatrices are equal.
func (bm *BitMatrix) Equals(other *BitMatrix) bool {
	if bm.width != other.width || bm.height != other.height || bm.rowSize != other.rowSize {
		return false
	}
	for i := range bm.data {
		if bm.data[i] != other.data[i] {
			return false
		}
	}
	return true
}
