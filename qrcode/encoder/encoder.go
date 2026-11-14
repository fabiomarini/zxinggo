// Package encoder implements QR code encoding.
package encoder

import (
	"fmt"
	"math"
	"strings"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/qrcode/decoder"
	"github.com/fabiomarini/zxinggo/reedsolomon"
)

const numMaskPatterns = 8

// ByteMatrix is a simple 2D byte matrix for QR code encoding.
type ByteMatrix struct {
	Data          []byte
	Width, Height int
}

// NewByteMatrix creates a new ByteMatrix.
func NewByteMatrix(width, height int) *ByteMatrix {
	return &ByteMatrix{Data: make([]byte, width*height), Width: width, Height: height}
}

// Get returns the value at (x, y).
func (bm *ByteMatrix) Get(x, y int) byte { return bm.Data[y*bm.Width+x] }

// Set sets the value at (x, y).
func (bm *ByteMatrix) Set(x, y int, value byte) { bm.Data[y*bm.Width+x] = value }

// SetBool sets the value at (x, y) as 1 (true) or 0 (false).
func (bm *ByteMatrix) SetBool(x, y int, value bool) {
	if value {
		bm.Data[y*bm.Width+x] = 1
	} else {
		bm.Data[y*bm.Width+x] = 0
	}
}

// Clear fills the matrix with the given value.
func (bm *ByteMatrix) Clear(value byte) {
	for i := range bm.Data {
		bm.Data[i] = value
	}
}

// QRCode holds the encoded QR code data.
type QRCode struct {
	Mode        decoder.Mode
	ECLevel     decoder.ErrorCorrectionLevel
	Version     *decoder.Version
	MaskPattern int
	Matrix      *ByteMatrix
}

// alphanumericTable maps ASCII values to alphanumeric codes.
var alphanumericTable = [128]int{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	36, -1, -1, -1, 37, 38, -1, -1, -1, -1, 39, 40, -1, 41, 42, 43,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 44, -1, -1, -1, -1, -1,
	-1, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
	25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
}

// GetAlphanumericCode returns the alphanumeric code for a character.
func GetAlphanumericCode(code int) int {
	if code < 128 {
		return alphanumericTable[code]
	}
	return -1
}

// ChooseMode determines the best encoding mode for the content.
func ChooseMode(content string) decoder.Mode {
	hasNumeric := false
	hasAlphanumeric := false
	for _, c := range content {
		if c >= '0' && c <= '9' {
			hasNumeric = true
		} else if GetAlphanumericCode(int(c)) != -1 {
			hasAlphanumeric = true
		} else {
			return decoder.ModeByte
		}
	}
	if hasAlphanumeric {
		return decoder.ModeAlphanumeric
	}
	if hasNumeric {
		return decoder.ModeNumeric
	}
	return decoder.ModeByte
}

// Encode encodes content into a QRCode.
func Encode(content string, ecLevel decoder.ErrorCorrectionLevel, qrVersion int, maskPattern int) (*QRCode, error) {
	mode := ChooseMode(content)

	// Build header bits
	headerBits := bitutil.NewBitArray(0)
	headerBits.AppendBits(uint32(mode.Bits()), 4)

	// Build data bits
	dataBits := bitutil.NewBitArray(0)
	if err := appendBytes(content, mode, dataBits); err != nil {
		return nil, err
	}

	// Choose version
	var version *decoder.Version
	var err error
	if qrVersion > 0 {
		version, err = decoder.GetVersionForNumber(qrVersion)
		if err != nil {
			return nil, err
		}
	} else {
		version, err = chooseVersion(mode, headerBits, dataBits, ecLevel)
		if err != nil {
			return nil, err
		}
	}

	// Complete header with character count
	numLetters := len(content)
	countBits := mode.CharacterCountBits(version)
	headerBits.AppendBits(uint32(numLetters), countBits)

	// Combine header and data
	headerBits.AppendBitArray(dataBits)

	// Calculate total data bytes
	ecBlocks := version.ECBlocksForLevel(ecLevel)
	totalBytes := version.TotalCodewords
	numDataBytes := totalBytes - ecBlocks.TotalECCodewords()

	// Terminate and pad
	if err := terminateBits(numDataBytes, headerBits); err != nil {
		return nil, err
	}

	// Interleave with EC bytes
	numRSBlocks := ecBlocks.NumBlocks()
	finalBits, err := interleaveWithECBytes(headerBits, totalBytes, numDataBytes, numRSBlocks)
	if err != nil {
		return nil, err
	}

	qr := &QRCode{
		Mode:        mode,
		ECLevel:     ecLevel,
		Version:     version,
		MaskPattern: -1,
	}

	dimension := version.DimensionForVersion()
	matrix := NewByteMatrix(dimension, dimension)

	// Choose best mask pattern
	if maskPattern >= 0 && maskPattern < numMaskPatterns {
		qr.MaskPattern = maskPattern
	} else {
		qr.MaskPattern = chooseMaskPattern(finalBits, ecLevel, version, matrix)
	}

	qr.Matrix = matrix
	buildMatrix(finalBits, ecLevel, version, qr.MaskPattern, matrix)

	return qr, nil
}

func chooseVersion(mode decoder.Mode, headerBits *bitutil.BitArray, dataBits *bitutil.BitArray, ecLevel decoder.ErrorCorrectionLevel) (*decoder.Version, error) {
	for versionNum := 1; versionNum <= 40; versionNum++ {
		version, _ := decoder.GetVersionForNumber(versionNum)
		totalBits := headerBits.Size() + mode.CharacterCountBits(version) + dataBits.Size()
		ecBlocks := version.ECBlocksForLevel(ecLevel)
		numDataBytes := version.TotalCodewords - ecBlocks.TotalECCodewords()
		if totalBits <= numDataBytes*8 {
			return version, nil
		}
	}
	return nil, fmt.Errorf("%w: data too large", zxinggo.ErrWriter)
}

func terminateBits(numDataBytes int, bits *bitutil.BitArray) error {
	capacity := numDataBytes * 8
	if bits.Size() > capacity {
		return fmt.Errorf("%w: data bits exceed capacity", zxinggo.ErrWriter)
	}

	// Terminator mode
	for i := 0; i < 4 && bits.Size() < capacity; i++ {
		bits.AppendBit(false)
	}

	// Pad to byte boundary
	numBitsInLastByte := bits.Size() & 0x07
	if numBitsInLastByte > 0 {
		for i := numBitsInLastByte; i < 8; i++ {
			bits.AppendBit(false)
		}
	}

	// Pad with alternating bytes
	numPaddingBytes := numDataBytes - bits.SizeInBytes()
	for i := 0; i < numPaddingBytes; i++ {
		if i%2 == 0 {
			bits.AppendBits(0xEC, 8)
		} else {
			bits.AppendBits(0x11, 8)
		}
	}
	return nil
}

func appendBytes(content string, mode decoder.Mode, bits *bitutil.BitArray) error {
	switch mode {
	case decoder.ModeNumeric:
		return appendNumericBytes(content, bits)
	case decoder.ModeAlphanumeric:
		return appendAlphanumericBytes(content, bits)
	case decoder.ModeByte:
		return append8BitBytes(content, bits)
	default:
		return fmt.Errorf("%w: unsupported mode", zxinggo.ErrWriter)
	}
}

func appendNumericBytes(content string, bits *bitutil.BitArray) error {
	length := len(content)
	i := 0
	for i < length {
		num1 := int(content[i] - '0')
		if i+2 < length {
			num2 := int(content[i+1] - '0')
			num3 := int(content[i+2] - '0')
			bits.AppendBits(uint32(num1*100+num2*10+num3), 10)
			i += 3
		} else if i+1 < length {
			num2 := int(content[i+1] - '0')
			bits.AppendBits(uint32(num1*10+num2), 7)
			i += 2
		} else {
			bits.AppendBits(uint32(num1), 4)
			i++
		}
	}
	return nil
}

func appendAlphanumericBytes(content string, bits *bitutil.BitArray) error {
	length := len(content)
	i := 0
	for i < length {
		code1 := GetAlphanumericCode(int(content[i]))
		if code1 == -1 {
			return fmt.Errorf("%w: invalid alphanumeric character", zxinggo.ErrWriter)
		}
		if i+1 < length {
			code2 := GetAlphanumericCode(int(content[i+1]))
			if code2 == -1 {
				return fmt.Errorf("%w: invalid alphanumeric character", zxinggo.ErrWriter)
			}
			bits.AppendBits(uint32(code1*45+code2), 11)
			i += 2
		} else {
			bits.AppendBits(uint32(code1), 6)
			i++
		}
	}
	return nil
}

func append8BitBytes(content string, bits *bitutil.BitArray) error {
	for i := 0; i < len(content); i++ {
		bits.AppendBits(uint32(content[i]), 8)
	}
	return nil
}

func interleaveWithECBytes(bits *bitutil.BitArray, numTotalBytes, numDataBytes, numRSBlocks int) (*bitutil.BitArray, error) {
	if bits.SizeInBytes() != numDataBytes {
		return nil, fmt.Errorf("%w: data bytes mismatch", zxinggo.ErrWriter)
	}

	// Split data into blocks
	dataBytesOffset := 0
	maxNumDataBytes := 0
	maxNumEcBytes := 0

	type blockPair struct {
		dataBytes []byte
		ecBytes   []byte
	}
	blocks := make([]blockPair, numRSBlocks)

	for i := 0; i < numRSBlocks; i++ {
		numDataBytesInBlock, numEcBytesInBlock := getNumDataBytesAndNumECBytesForBlockID(
			numTotalBytes, numDataBytes, numRSBlocks, i)

		dataBytes := make([]byte, numDataBytesInBlock)
		if err := bits.ToBytes(8*dataBytesOffset, dataBytes, 0, numDataBytesInBlock); err != nil {
			return nil, err
		}
		ecBytes, err := generateECBytes(dataBytes, numEcBytesInBlock)
		if err != nil {
			return nil, err
		}
		blocks[i] = blockPair{dataBytes: dataBytes, ecBytes: ecBytes}

		if numDataBytesInBlock > maxNumDataBytes {
			maxNumDataBytes = numDataBytesInBlock
		}
		if numEcBytesInBlock > maxNumEcBytes {
			maxNumEcBytes = numEcBytesInBlock
		}
		dataBytesOffset += numDataBytesInBlock
	}

	result := bitutil.NewBitArray(0)

	// Interleave data bytes
	for i := 0; i < maxNumDataBytes; i++ {
		for _, block := range blocks {
			if i < len(block.dataBytes) {
				result.AppendBits(uint32(block.dataBytes[i]), 8)
			}
		}
	}
	// Interleave EC bytes
	for i := 0; i < maxNumEcBytes; i++ {
		for _, block := range blocks {
			if i < len(block.ecBytes) {
				result.AppendBits(uint32(block.ecBytes[i]), 8)
			}
		}
	}

	if result.SizeInBytes() != numTotalBytes {
		return nil, fmt.Errorf("%w: interleaved size mismatch", zxinggo.ErrWriter)
	}
	return result, nil
}

func getNumDataBytesAndNumECBytesForBlockID(numTotalBytes, numDataBytes, numRSBlocks, blockID int) (int, int) {
	if blockID >= numRSBlocks {
		return 0, 0
	}
	numRsBlocksInGroup2 := numTotalBytes % numRSBlocks
	numRsBlocksInGroup1 := numRSBlocks - numRsBlocksInGroup2
	numTotalBytesInGroup1 := numTotalBytes / numRSBlocks
	numTotalBytesInGroup2 := numTotalBytesInGroup1 + 1
	numDataBytesInGroup1 := numDataBytes / numRSBlocks
	numDataBytesInGroup2 := numDataBytesInGroup1 + 1
	numEcBytesInGroup1 := numTotalBytesInGroup1 - numDataBytesInGroup1
	numEcBytesInGroup2 := numTotalBytesInGroup2 - numDataBytesInGroup2

	if blockID < numRsBlocksInGroup1 {
		return numDataBytesInGroup1, numEcBytesInGroup1
	}
	return numDataBytesInGroup2, numEcBytesInGroup2
}

func generateECBytes(dataBytes []byte, numEcBytesInBlock int) ([]byte, error) {
	numDataBytes := len(dataBytes)
	toEncode := make([]int, numDataBytes+numEcBytesInBlock)
	for i, b := range dataBytes {
		toEncode[i] = int(b) & 0xFF
	}
	enc := reedsolomon.NewEncoder(reedsolomon.QRCodeField256)
	if err := enc.Encode(toEncode, numEcBytesInBlock); err != nil {
		return nil, err
	}
	ecBytes := make([]byte, numEcBytesInBlock)
	for i := 0; i < numEcBytesInBlock; i++ {
		ecBytes[i] = byte(toEncode[numDataBytes+i])
	}
	return ecBytes, nil
}

func chooseMaskPattern(bits *bitutil.BitArray, ecLevel decoder.ErrorCorrectionLevel, version *decoder.Version, matrix *ByteMatrix) int {
	minPenalty := math.MaxInt32
	bestPattern := 0
	for i := 0; i < numMaskPatterns; i++ {
		buildMatrix(bits, ecLevel, version, i, matrix)
		penalty := calculateMaskPenalty(matrix)
		if penalty < minPenalty {
			minPenalty = penalty
			bestPattern = i
		}
	}
	return bestPattern
}

func calculateMaskPenalty(matrix *ByteMatrix) int {
	return applyMaskPenaltyRule1(matrix) +
		applyMaskPenaltyRule2(matrix) +
		applyMaskPenaltyRule3(matrix) +
		applyMaskPenaltyRule4(matrix)
}

// Mask penalty rule 1: penalize runs of 5+ same-color modules
func applyMaskPenaltyRule1(matrix *ByteMatrix) int {
	return applyMaskPenaltyRule1Horizontal(matrix) + applyMaskPenaltyRule1Vertical(matrix)
}

func applyMaskPenaltyRule1Horizontal(matrix *ByteMatrix) int {
	penalty := 0
	w := matrix.Width
	for y := 0; y < matrix.Height; y++ {
		row := matrix.Data[y*w : (y+1)*w]
		numSameBitCells := 0
		prevBit := byte(255) // invalid
		for _, bit := range row {
			if bit == prevBit {
				numSameBitCells++
			} else {
				if numSameBitCells >= 5 {
					penalty += 3 + (numSameBitCells - 5)
				}
				numSameBitCells = 1
				prevBit = bit
			}
		}
		if numSameBitCells >= 5 {
			penalty += 3 + (numSameBitCells - 5)
		}
	}
	return penalty
}

func applyMaskPenaltyRule1Vertical(matrix *ByteMatrix) int {
	penalty := 0
	data := matrix.Data
	w := matrix.Width
	for x := 0; x < w; x++ {
		numSameBitCells := 0
		prevBit := byte(255) // invalid
		for y, off := 0, x; y < matrix.Height; y, off = y+1, off+w {
			bit := data[off]
			if bit == prevBit {
				numSameBitCells++
			} else {
				if numSameBitCells >= 5 {
					penalty += 3 + (numSameBitCells - 5)
				}
				numSameBitCells = 1
				prevBit = bit
			}
		}
		if numSameBitCells >= 5 {
			penalty += 3 + (numSameBitCells - 5)
		}
	}
	return penalty
}

// Mask penalty rule 2: penalize 2x2 blocks of same color
func applyMaskPenaltyRule2(matrix *ByteMatrix) int {
	penalty := 0
	w := matrix.Width
	for y := 0; y < matrix.Height-1; y++ {
		// two adjacent rows in one slice; row[x+w] is (x, y+1)
		row := matrix.Data[y*w : (y+2)*w]
		for x := 0; x < w-1; x++ {
			value := row[x]
			if value == row[x+1] && value == row[x+w] && value == row[x+w+1] {
				penalty += 3
			}
		}
	}
	return penalty
}

// Mask penalty rule 3: penalize finder-like patterns
func applyMaskPenaltyRule3(matrix *ByteMatrix) int {
	penalty := 0
	w, h := matrix.Width, matrix.Height
	data := matrix.Data
	// horizontal: scan rows directly
	for y := 0; y < h; y++ {
		row := data[y*w : (y+1)*w]
		for x := 0; x+6 < w; x++ {
			if row[x] == 1 && row[x+1] == 0 &&
				row[x+2] == 1 && row[x+3] == 1 &&
				row[x+4] == 1 && row[x+5] == 0 &&
				row[x+6] == 1 {
				leadingWhite := x+10 < w && row[x+7] == 0 && row[x+8] == 0 &&
					row[x+9] == 0 && row[x+10] == 0
				trailingWhite := x >= 4 && row[x-1] == 0 && row[x-2] == 0 &&
					row[x-3] == 0 && row[x-4] == 0
				if leadingWhite || trailingWhite {
					penalty += 40
				}
			}
		}
	}
	// vertical: stride one column with a single flat index
	for x := 0; x < w; x++ {
		for y := 0; y+6 < h; y++ {
			i := y*w + x
			if data[i] == 1 && data[i+w] == 0 &&
				data[i+2*w] == 1 && data[i+3*w] == 1 &&
				data[i+4*w] == 1 && data[i+5*w] == 0 &&
				data[i+6*w] == 1 {
				leadingWhite := y+10 < h && data[i+7*w] == 0 && data[i+8*w] == 0 &&
					data[i+9*w] == 0 && data[i+10*w] == 0
				trailingWhite := y >= 4 && data[i-w] == 0 && data[i-2*w] == 0 &&
					data[i-3*w] == 0 && data[i-4*w] == 0
				if leadingWhite || trailingWhite {
					penalty += 40
				}
			}
		}
	}
	return penalty
}

// Mask penalty rule 4: penalize deviation from 50% dark modules
func applyMaskPenaltyRule4(matrix *ByteMatrix) int {
	numDarkCells := 0
	for _, v := range matrix.Data {
		if v == 1 {
			numDarkCells++
		}
	}
	total := matrix.Height * matrix.Width
	fivePercentVariances := abs(numDarkCells*2-total) * 10 / total
	return fivePercentVariances * 10
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// buildMatrix builds the QR code matrix with all patterns and data.
func buildMatrix(dataBits *bitutil.BitArray, ecLevel decoder.ErrorCorrectionLevel,
	version *decoder.Version, maskPattern int, matrix *ByteMatrix) {

	matrix.Clear(0xFF) // empty marker

	embedBasicPatterns(version, matrix)
	embedTypeInfo(ecLevel, maskPattern, matrix)
	maybeEmbedVersionInfo(version, matrix)
	embedDataBits(dataBits, maskPattern, matrix)
}

// Position detection pattern (7x7 finder pattern)
var positionDetectionPattern = [7][7]byte{
	{1, 1, 1, 1, 1, 1, 1},
	{1, 0, 0, 0, 0, 0, 1},
	{1, 0, 1, 1, 1, 0, 1},
	{1, 0, 1, 1, 1, 0, 1},
	{1, 0, 1, 1, 1, 0, 1},
	{1, 0, 0, 0, 0, 0, 1},
	{1, 1, 1, 1, 1, 1, 1},
}

// Position adjustment pattern (5x5 alignment pattern)
var positionAdjustmentPattern = [5][5]byte{
	{1, 1, 1, 1, 1},
	{1, 0, 0, 0, 1},
	{1, 0, 1, 0, 1},
	{1, 0, 0, 0, 1},
	{1, 1, 1, 1, 1},
}

func embedBasicPatterns(version *decoder.Version, matrix *ByteMatrix) {
	// Position detection patterns and separators
	embedPositionDetectionPattern(0, 0, matrix)
	embedPositionDetectionPattern(matrix.Width-7, 0, matrix)
	embedPositionDetectionPattern(0, matrix.Height-7, matrix)

	// Horizontal separators
	embedHorizontalSeparator(0, 7, matrix)
	embedHorizontalSeparator(matrix.Width-8, 7, matrix)
	embedHorizontalSeparator(0, matrix.Height-8, matrix)

	// Vertical separators
	embedVerticalSeparator(7, 0, matrix)
	embedVerticalSeparator(matrix.Width-8, 0, matrix)
	embedVerticalSeparator(7, matrix.Height-7, matrix)

	// Alignment patterns
	if version.Number >= 2 {
		embedPositionAdjustmentPatterns(version, matrix)
	}

	// Timing patterns
	embedTimingPatterns(matrix)

	// Dark module
	matrix.Set(8, matrix.Height-8, 1)
}

func embedPositionDetectionPattern(xStart, yStart int, matrix *ByteMatrix) {
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			matrix.Set(xStart+x, yStart+y, positionDetectionPattern[y][x])
		}
	}
}

func embedHorizontalSeparator(xStart, yStart int, matrix *ByteMatrix) {
	for x := 0; x < 8; x++ {
		if xStart+x < matrix.Width {
			matrix.Set(xStart+x, yStart, 0)
		}
	}
}

func embedVerticalSeparator(xStart, yStart int, matrix *ByteMatrix) {
	for y := 0; y < 7; y++ {
		if yStart+y < matrix.Height {
			matrix.Set(xStart, yStart+y, 0)
		}
	}
}

func embedPositionAdjustmentPatterns(version *decoder.Version, matrix *ByteMatrix) {
	centers := version.AlignmentPatternCenters
	for _, cy := range centers {
		for _, cx := range centers {
			// Only embed if the center cell is empty (not already occupied by a finder pattern)
			if matrix.Get(cx, cy) != 0xFF {
				continue
			}
			for y := 0; y < 5; y++ {
				for x := 0; x < 5; x++ {
					matrix.Set(cx-2+x, cy-2+y, positionAdjustmentPattern[y][x])
				}
			}
		}
	}
}

func embedTimingPatterns(matrix *ByteMatrix) {
	for i := 8; i < matrix.Width-8; i++ {
		bit := byte((i + 1) % 2)
		if matrix.Get(i, 6) == 0xFF {
			matrix.Set(i, 6, bit)
		}
		if matrix.Get(6, i) == 0xFF {
			matrix.Set(6, i, bit)
		}
	}
}

const (
	typeInfoPoly        = 0x537
	typeInfoMaskPattern = 0x5412
	versionInfoPoly     = 0x1f25
)

func embedTypeInfo(ecLevel decoder.ErrorCorrectionLevel, maskPattern int, matrix *ByteMatrix) {
	typeInfo := (ecLevel.Bits() << 3) | maskPattern
	bchCode := calculateBCHCode(typeInfo, typeInfoPoly)
	typeInfoBits := (typeInfo << 10) | bchCode
	typeInfoBits ^= typeInfoMaskPattern

	// Type info coordinates around top-left
	typeInfoCoordinates := [][2]int{
		{8, 0}, {8, 1}, {8, 2}, {8, 3}, {8, 4}, {8, 5}, {8, 7}, {8, 8},
		{7, 8}, {5, 8}, {4, 8}, {3, 8}, {2, 8}, {1, 8}, {0, 8},
	}

	for i := 0; i < 15; i++ {
		bit := byte((typeInfoBits >> uint(i)) & 1)
		coord := typeInfoCoordinates[i]
		matrix.Set(coord[0], coord[1], bit)

		// Also place in the second location
		if i < 8 {
			matrix.Set(matrix.Width-1-i, 8, bit)
		} else {
			matrix.Set(8, matrix.Height-7+(i-8), bit)
		}
	}
}

func maybeEmbedVersionInfo(version *decoder.Version, matrix *ByteMatrix) {
	if version.Number < 7 {
		return
	}
	versionInfoBits := calculateBCHCode(version.Number, versionInfoPoly)
	versionInfoBits = (version.Number << 12) | versionInfoBits

	bitIndex := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < 3; j++ {
			bit := byte((versionInfoBits >> uint(bitIndex)) & 1)
			bitIndex++
			// Bottom-left
			matrix.Set(i, matrix.Height-11+j, bit)
			// Top-right
			matrix.Set(matrix.Width-11+j, i, bit)
		}
	}
}

func embedDataBits(dataBits *bitutil.BitArray, maskPattern int, matrix *ByteMatrix) {
	bitIndex := 0
	dimension := matrix.Height
	w := matrix.Width
	data := matrix.Data
	maskFn := decoder.DataMasks[maskPattern]
	numBits := dataBits.Size()

	for j := dimension - 1; j > 0; j -= 2 {
		if j == 6 {
			j-- // skip timing column
		}
		upward := (((dimension - 1 - j) / 2) & 1) == 0
		for count := 0; count < dimension; count++ {
			i := count
			if upward {
				i = dimension - 1 - count
			}
			base := i * w
			for col := 0; col < 2; col++ {
				x := j - col
				if data[base+x] == 0xFF { // empty cell
					var bit bool
					if bitIndex < numBits {
						bit = dataBits.Get(bitIndex)
						bitIndex++
					}
					// Apply mask
					if maskFn(i, x) {
						bit = !bit
					}
					if bit {
						data[base+x] = 1
					} else {
						data[base+x] = 0
					}
				}
			}
		}
	}
}

func calculateBCHCode(value, poly int) int {
	msbSetInPoly := findMSBSet(poly)
	value <<= uint(msbSetInPoly - 1)
	for findMSBSet(value) >= msbSetInPoly {
		value ^= poly << uint(findMSBSet(value)-msbSetInPoly)
	}
	return value
}

func findMSBSet(value int) int {
	count := 0
	for value != 0 {
		value >>= 1
		count++
	}
	return count
}

// RenderResult renders a QRCode to a BitMatrix with the given dimensions.
func RenderResult(code *QRCode, width, height, quietZone int) (*bitutil.BitMatrix, error) {
	input := code.Matrix
	inputWidth := input.Width
	inputHeight := input.Height
	qrWidth := inputWidth + quietZone*2
	qrHeight := inputHeight + quietZone*2
	outputWidth := width
	if outputWidth < qrWidth {
		outputWidth = qrWidth
	}
	outputHeight := height
	if outputHeight < qrHeight {
		outputHeight = qrHeight
	}

	multiple := outputWidth / qrWidth
	if h := outputHeight / qrHeight; h < multiple {
		multiple = h
	}

	leftPadding := (outputWidth - inputWidth*multiple) / 2
	topPadding := (outputHeight - inputHeight*multiple) / 2

	output := bitutil.NewBitMatrixWithSize(outputWidth, outputHeight)

	w := inputWidth
	for inputY := 0; inputY < inputHeight; inputY++ {
		row := input.Data[inputY*w : (inputY+1)*w]
		outputY := topPadding + inputY*multiple
		for inputX := 0; inputX < w; inputX++ {
			if row[inputX] == 1 {
				outputX := leftPadding + inputX*multiple
				if err := output.SetRegion(outputX, outputY, multiple, multiple); err != nil {
					return nil, err
				}
			}
		}
	}

	return output, nil
}

// ToBitMatrix converts a QRCode's ByteMatrix to a BitMatrix.
func (qr *QRCode) ToBitMatrix() *bitutil.BitMatrix {
	bm := bitutil.NewBitMatrixWithSize(qr.Matrix.Width, qr.Matrix.Height)
	for y := 0; y < qr.Matrix.Height; y++ {
		for x := 0; x < qr.Matrix.Width; x++ {
			if qr.Matrix.Get(x, y) == 1 {
				bm.Set(x, y)
			}
		}
	}
	return bm
}

// String returns a visual representation of the QR code.
func (qr *QRCode) String() string {
	var sb strings.Builder
	for y := 0; y < qr.Matrix.Height; y++ {
		for x := 0; x < qr.Matrix.Width; x++ {
			switch qr.Matrix.Get(x, y) {
			case 1:
				sb.WriteString("##")
			case 0:
				sb.WriteString("  ")
			default:
				sb.WriteString("  ")
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
