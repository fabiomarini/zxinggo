package zxinggo

import (
	"image"
	"image/color"
)

// ImageLuminanceSource is a LuminanceSource implementation that wraps a Go
// image.Image, converting each pixel to greyscale luminance on the fly.
type ImageLuminanceSource struct {
	luminances []byte
	width      int
	height     int
}

// NewImageLuminanceSource creates a LuminanceSource from a Go image.Image.
// The image is converted to greyscale luminance values upon construction.
// Uses the same luminance formula as Java ZXing's BufferedImageLuminanceSource:
// (306*R + 601*G + 117*B + 0x200) >> 10, operating on 8-bit color components.
// Fully-transparent pixels become white. Common concrete image types take a
// direct Pix/stride fast path; exotic types fall back to At/RGBA.
func NewImageLuminanceSource(img image.Image) *ImageLuminanceSource {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	luminances := make([]byte, w*h)

	switch src := img.(type) {
	case *image.Gray:
		for y := range h {
			copy(luminances[y*w:(y+1)*w], src.Pix[(y+bounds.Min.Y)*src.Stride+bounds.Min.X:(y+bounds.Min.Y)*src.Stride+bounds.Min.X+w])
		}
	case *image.RGBA:
		for y := range h {
			pix := src.Pix[(y+bounds.Min.Y)*src.Stride+bounds.Min.X*4:]
			dst := luminances[y*w : (y+1)*w]
			for x := range w {
				a := pix[x*4+3]
				if a == 0 {
					dst[x] = 0xFF
				} else {
					dst[x] = byte((306*int(pix[x*4]) + 601*int(pix[x*4+1]) + 117*int(pix[x*4+2]) + 0x200) >> 10)
				}
			}
		}
	case *image.NRGBA:
		for y := range h {
			pix := src.Pix[(y+bounds.Min.Y)*src.Stride+bounds.Min.X*4:]
			dst := luminances[y*w : (y+1)*w]
			for x := range w {
				a := pix[x*4+3]
				if a == 0 {
					dst[x] = 0xFF
				} else {
					// At().RGBA() reports premultiplied values; match them exactly.
					a16 := uint32(a) * 257
					r := uint32(pix[x*4]) * 257 * a16 >> 16 >> 8
					g := uint32(pix[x*4+1]) * 257 * a16 >> 16 >> 8
					b := uint32(pix[x*4+2]) * 257 * a16 >> 16 >> 8
					dst[x] = byte((306*int(r) + 601*int(g) + 117*int(b) + 0x200) >> 10)
				}
			}
		}
	case *image.Gray16:
		for y := range h {
			pix := src.Pix[(y+bounds.Min.Y)*src.Stride+bounds.Min.X*2:]
			dst := luminances[y*w : (y+1)*w]
			for x := range w {
				dst[x] = pix[x*2]
			}
		}
	case *image.RGBA64:
		for y := range h {
			row := src.Pix[(y+bounds.Min.Y)*src.Stride+bounds.Min.X*8:]
			dst := luminances[y*w : (y+1)*w]
			for x := range w {
				if row[x*8+6]|row[x*8+7] == 0 {
					dst[x] = 0xFF
				} else {
					dst[x] = byte((306*int(row[x*8]) + 601*int(row[x*8+2]) + 117*int(row[x*8+4]) + 0x200) >> 10)
				}
			}
		}
	case *image.NRGBA64:
		for y := range h {
			row := src.Pix[(y+bounds.Min.Y)*src.Stride+bounds.Min.X*8:]
			dst := luminances[y*w : (y+1)*w]
			for x := range w {
				if row[x*8+6]|row[x*8+7] == 0 {
					dst[x] = 0xFF
				} else {
					// color.NRGBA64.RGBA() premultiplies: v*a/0xffff on 16-bit values.
					a16 := uint32(row[x*8+6])<<8 | uint32(row[x*8+7])
					r := (uint32(row[x*8])<<8|uint32(row[x*8+1]))*a16/0xffff>>8
					g := (uint32(row[x*8+2])<<8|uint32(row[x*8+3]))*a16/0xffff>>8
					b := (uint32(row[x*8+4])<<8|uint32(row[x*8+5]))*a16/0xffff>>8
					dst[x] = byte((306*int(r) + 601*int(g) + 117*int(b) + 0x200) >> 10)
				}
			}
		}
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
				_, _, _, a := c.RGBA()
				if a == 0 {
					// Fully-transparent pixels are forced to white, matching Java behavior.
					luminances[y*w+x] = 0xFF
				} else {
					r, g, b, _ := c.RGBA()
					// 16-bit premultiplied RGBA to 8-bit components, Java formula.
					luminances[y*w+x] = byte((306*(r>>8) + 601*(g>>8) + 117*(b>>8) + 0x200) >> 10)
				}
			}
		}
	}

	return &ImageLuminanceSource{
		luminances: luminances,
		width:      w,
		height:     h,
	}
}

// NewGrayImageLuminanceSource creates a LuminanceSource from a *image.Gray,
// using the pixel data directly without conversion.
func NewGrayImageLuminanceSource(img *image.Gray) *ImageLuminanceSource {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// If the image stride matches the width, we can use the pixel data directly
	if img.Stride == w && bounds.Min.X == 0 && bounds.Min.Y == 0 {
		lum := make([]byte, w*h)
		copy(lum, img.Pix[:w*h])
		return &ImageLuminanceSource{
			luminances: lum,
			width:      w,
			height:     h,
		}
	}

	// Otherwise copy row by row
	luminances := make([]byte, w*h)
	for y := 0; y < h; y++ {
		srcOff := (bounds.Min.Y+y)*img.Stride + bounds.Min.X
		copy(luminances[y*w:], img.Pix[srcOff:srcOff+w])
	}
	return &ImageLuminanceSource{
		luminances: luminances,
		width:      w,
		height:     h,
	}
}

// Row returns a row of luminance data.
func (s *ImageLuminanceSource) Row(y int, row []byte) []byte {
	if y < 0 || y >= s.height {
		return nil
	}
	if row == nil || len(row) < s.width {
		row = make([]byte, s.width)
	}
	offset := y * s.width
	copy(row, s.luminances[offset:offset+s.width])
	return row
}

// Column returns a column of luminance data, top to bottom.
func (s *ImageLuminanceSource) Column(x int, buf []byte) []byte {
	if x < 0 || x >= s.width {
		return nil
	}
	if buf == nil || len(buf) < s.height {
		buf = make([]byte, s.height)
	}
	for y := range s.height {
		buf[y] = s.luminances[y*s.width+x]
	}
	return buf
}

// Matrix returns the entire luminance matrix.
func (s *ImageLuminanceSource) Matrix() []byte {
	result := make([]byte, len(s.luminances))
	copy(result, s.luminances)
	return result
}

// Width returns the width of the image.
func (s *ImageLuminanceSource) Width() int {
	return s.width
}

// Height returns the height of the image.
func (s *ImageLuminanceSource) Height() int {
	return s.height
}

// RotateCounterClockwise returns a new LuminanceSource rotated 90 degrees
// counterclockwise. This is used by 1D readers to try reading barcodes that
// may be oriented vertically.
func (s *ImageLuminanceSource) RotateCounterClockwise() LuminanceSource {
	newWidth := s.height
	newHeight := s.width
	newLum := make([]byte, newWidth*newHeight)
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			// (x, y) in old image -> (y, width - 1 - x) in new image
			newLum[(s.width-1-x)*newWidth+y] = s.luminances[y*s.width+x]
		}
	}
	return &ImageLuminanceSource{
		luminances: newLum,
		width:      newWidth,
		height:     newHeight,
	}
}

// Crop returns a new LuminanceSource that represents a rectangular sub-region
// of this source. It returns nil when the rectangle is empty or extends
// outside the source bounds.
func (s *ImageLuminanceSource) Crop(left, top, cropWidth, cropHeight int) LuminanceSource {
	if left < 0 || top < 0 || cropWidth <= 0 || cropHeight <= 0 ||
		left+cropWidth > s.width || top+cropHeight > s.height {
		return nil
	}
	newLum := make([]byte, cropWidth*cropHeight)
	for y := range cropHeight {
		srcOff := (top+y)*s.width + left
		copy(newLum[y*cropWidth:], s.luminances[srcOff:srcOff+cropWidth])
	}
	return &ImageLuminanceSource{
		luminances: newLum,
		width:      cropWidth,
		height:     cropHeight,
	}
}

// InvertedLuminanceSource wraps another LuminanceSource and returns inverted
// luminance values (255 - v), enabling white-on-black barcode reading without
// mutating the wrapped source.
type InvertedLuminanceSource struct {
	source LuminanceSource
}

// NewInvertedLuminanceSource creates an inverting wrapper around the given
// luminance source.
func NewInvertedLuminanceSource(source LuminanceSource) *InvertedLuminanceSource {
	return &InvertedLuminanceSource{source: source}
}

// Width returns the width of the wrapped source.
func (s *InvertedLuminanceSource) Width() int { return s.source.Width() }

// Height returns the height of the wrapped source.
func (s *InvertedLuminanceSource) Height() int { return s.source.Height() }

// Row returns the inverted luminance values for the given row.
func (s *InvertedLuminanceSource) Row(y int, row []byte) []byte {
	row = s.source.Row(y, row)
	for i := range row {
		row[i] = 255 - row[i]
	}
	return row
}

// Matrix returns the inverted luminance matrix.
func (s *InvertedLuminanceSource) Matrix() []byte {
	m := s.source.Matrix()
	for i := range m {
		m[i] = 255 - m[i]
	}
	return m
}

// Crop returns an inverted source over the cropped region of the wrapped
// source, or nil when cropping is unsupported or out of bounds.
func (s *InvertedLuminanceSource) Crop(left, top, cropWidth, cropHeight int) LuminanceSource {
	if c, ok := s.source.(croppableSource); ok {
		if cropped := c.Crop(left, top, cropWidth, cropHeight); cropped != nil {
			return NewInvertedLuminanceSource(cropped)
		}
	}
	return nil
}

// RotateCounterClockwise returns an inverted source over the rotated wrapped
// source, or nil when rotation is unsupported.
func (s *InvertedLuminanceSource) RotateCounterClockwise() LuminanceSource {
	if r, ok := s.source.(rotatableSource); ok {
		if rotated := r.RotateCounterClockwise(); rotated != nil {
			return NewInvertedLuminanceSource(rotated)
		}
	}
	return nil
}

// BitMatrixToImage converts a BitMatrix to a grayscale image where black
// modules are black (0) and white modules are white (255).
func BitMatrixToImage(matrix interface {
	Width() int
	Height() int
	Get(x, y int) bool
}) *image.Gray {
	w := matrix.Width()
	h := matrix.Height()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if matrix.Get(x, y) {
				img.SetGray(x, y, color.Gray{Y: 0})
			} else {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return img
}
