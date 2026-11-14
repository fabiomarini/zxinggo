package pdf417

import (
	"strings"
	"testing"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/binarizer"
)

func TestPDF417WriterBasic(t *testing.T) {
	writer := NewPDF417Writer()
	matrix, err := writer.Encode("Hello, World!", zxinggo.FormatPDF417, 400, 200, nil)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if matrix.Width() == 0 || matrix.Height() == 0 {
		t.Fatal("expected non-empty matrix")
	}
	t.Logf("matrix size: %dx%d", matrix.Width(), matrix.Height())
}

func TestPDF417WriterNumeric(t *testing.T) {
	writer := NewPDF417Writer()
	matrix, err := writer.Encode("1234567890123456", zxinggo.FormatPDF417, 400, 200, nil)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if matrix.Width() == 0 || matrix.Height() == 0 {
		t.Fatal("expected non-empty matrix")
	}
}

func TestPDF417WriterWrongFormat(t *testing.T) {
	writer := NewPDF417Writer()
	_, err := writer.Encode("test", zxinggo.FormatQRCode, 400, 200, nil)
	if err == nil {
		t.Error("expected error for wrong format")
	}
}

func TestPDF417WriterWithOptions(t *testing.T) {
	writer := NewPDF417Writer()
	margin := 10
	opts := &zxinggo.EncodeOptions{
		Margin:          &margin,
		ErrorCorrection: "4",
	}
	matrix, err := writer.Encode("Test with options", zxinggo.FormatPDF417, 400, 200, opts)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if matrix.Width() == 0 || matrix.Height() == 0 {
		t.Fatal("expected non-empty matrix")
	}
}

// roundTrip encodes contents with the PDF417 writer and decodes the render
// back with the Go reader, returning the decoded text.
func roundTrip(t *testing.T, contents string, opts *zxinggo.EncodeOptions) string {
	t.Helper()

	matrix, err := zxinggo.Encode(contents, zxinggo.FormatPDF417, 600, 300, opts)
	if err != nil {
		t.Fatalf("Encode(%q) failed: %v", contents, err)
	}
	img := zxinggo.BitMatrixToImage(matrix)
	source := zxinggo.NewGrayImageLuminanceSource(img)
	bin := binarizer.NewGlobalHistogram(source)
	bitmap := zxinggo.NewBinaryBitmap(bin)
	decodeOpts := &zxinggo.DecodeOptions{
		PossibleFormats: []zxinggo.Format{zxinggo.FormatPDF417},
	}
	result, err := zxinggo.Decode(bitmap, decodeOpts)
	if err != nil {
		t.Fatalf("Decode of render of %q failed: %v", contents, err)
	}
	return result.Text
}

func TestPDF417RoundTrip(t *testing.T) {
	long := strings.Repeat("PDF417 round-trip payload 0123456789 ", 10) // 380 chars
	tests := []struct {
		name     string
		contents string
		level    string // EncodeOptions.ErrorCorrection; "" = default (2)
	}{
		{"ascii", "Hello, World!", ""},
		{"ascii upper", "HELLO WORLD 12345", "0"},
		{"numeric", "123456789012345678901234567890", "4"},
		{"utf8 multibyte", "Grüße, Welt: über 9 Nüsse", ""},
		{"long content", long, "2"},
		{"byte heavy", "a\x01\x02\x7f\x1b byte data", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts *zxinggo.EncodeOptions
			if tt.level != "" {
				opts = &zxinggo.EncodeOptions{ErrorCorrection: tt.level}
			}
			got := roundTrip(t, tt.contents, opts)
			if got != tt.contents {
				t.Errorf("round-trip mismatch:\n got %q\nwant %q", got, tt.contents)
			}
		})
	}
}

func TestPDF417RoundTripCompact(t *testing.T) {
	contents := "Hello, World!"
	got := roundTrip(t, contents, &zxinggo.EncodeOptions{PDF417Compact: true})
	if got != contents {
		t.Errorf("compact round-trip mismatch:\n got %q\nwant %q", got, contents)
	}
}
