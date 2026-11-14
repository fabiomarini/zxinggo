package oned

// UPC/EAN shared tables and checksum, used by the pattern-row reader
// (multilupceanreader.go) and the UPC/EAN writers.

// UPC/EAN guard patterns.
var (
	UPCEANStartEndPattern = []int{1, 1, 1}
	UPCEANMiddlePattern   = []int{1, 1, 1, 1, 1}
	UPCEANEndPattern      = []int{1, 1, 1, 1, 1, 1}
)

// LPatterns contains the "odd" or "L" patterns for encoding UPC/EAN digits.
var LPatterns = [10][]int{
	{3, 2, 1, 1}, // 0
	{2, 2, 2, 1}, // 1
	{2, 1, 2, 2}, // 2
	{1, 4, 1, 1}, // 3
	{1, 1, 3, 2}, // 4
	{1, 2, 3, 1}, // 5
	{1, 1, 1, 4}, // 6
	{1, 3, 1, 2}, // 7
	{1, 2, 1, 3}, // 8
	{3, 1, 1, 2}, // 9
}

// LAndGPatterns includes both the L and G patterns.
// Indices 0-9 are L patterns, 10-19 are G patterns (reversed L patterns).
var LAndGPatterns [20][]int

func init() {
	for i := 0; i < 10; i++ {
		LAndGPatterns[i] = LPatterns[i]
	}
	for i := 10; i < 20; i++ {
		widths := LPatterns[i-10]
		reversed := make([]int, len(widths))
		for j := 0; j < len(widths); j++ {
			reversed[j] = widths[len(widths)-j-1]
		}
		LAndGPatterns[i] = reversed
	}
}

// ean13FirstDigitEncodings maps the first EAN-13 digit to the L/G parity
// pattern of the left six digits.
var ean13FirstDigitEncodings = []int{0x00, 0x0B, 0x0D, 0x0E, 0x13, 0x19, 0x1C, 0x15, 0x16, 0x1A}

// upceNumSysAndCheckDigitPatterns represents the number system (index/10)
// and check digit (index%10) implied by the parity of the six UPC-E digits.
var upceNumSysAndCheckDigitPatterns = []int{
	0x38, 0x34, 0x32, 0x31, 0x2C, 0x26, 0x23, 0x2A, 0x29, 0x25,
	0x07, 0x0B, 0x0D, 0x0E, 0x13, 0x19, 0x1C, 0x15, 0x16, 0x1A,
}

// CheckStandardUPCEANChecksum verifies the UPC/EAN check digit.
func CheckStandardUPCEANChecksum(s string) bool {
	length := len(s)
	if length == 0 {
		return false
	}
	check := int(s[length-1] - '0')
	return GetStandardUPCEANChecksum(s[:length-1]) == check
}

// GetStandardUPCEANChecksum computes the UPC/EAN check digit for a string of digits
// (without the check digit itself).
func GetStandardUPCEANChecksum(s string) int {
	length := len(s)
	sum := 0
	for i := length - 1; i >= 0; i -= 2 {
		d := int(s[i] - '0')
		if d < 0 || d > 9 {
			return -1
		}
		sum += d
	}
	sum *= 3
	for i := length - 2; i >= 0; i -= 2 {
		d := int(s[i] - '0')
		if d < 0 || d > 9 {
			return -1
		}
		sum += d
	}
	return (1000 - sum) % 10
}

// ConvertUPCEtoUPCA expands a UPC-E value into its equivalent UPC-A code.
func ConvertUPCEtoUPCA(upce string) string {
	if len(upce) < 7 {
		return upce
	}
	chars := upce[1:7]
	var b []byte
	b = append(b, upce[0])
	lastChar := chars[5]
	switch lastChar {
	case '0', '1', '2':
		b = append(b, chars[0:2]...)
		b = append(b, lastChar)
		b = append(b, "0000"...)
		b = append(b, chars[2:5]...)
	case '3':
		b = append(b, chars[0:3]...)
		b = append(b, "00000"...)
		b = append(b, chars[3:5]...)
	case '4':
		b = append(b, chars[0:4]...)
		b = append(b, "00000"...)
		b = append(b, chars[4])
	default:
		b = append(b, chars[0:5]...)
		b = append(b, "0000"...)
		b = append(b, lastChar)
	}
	if len(upce) >= 8 {
		b = append(b, upce[7])
	}
	return string(b)
}
