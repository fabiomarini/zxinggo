// Package localgrid ports the grid sampling machinery used by the zxing-cpp
// Aztec detector: the PerspectiveTransform (core/src/PerspectiveTransform.h),
// the quadrilateral helpers (core/src/Quadrilateral.h), the iterative grid
// refinement sampler (core/src/LocalGrid.cpp) and the SampleGrid functions
// (core/src/GridSampler.cpp).
package localgrid

import (
	"math"

	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/bmcursor"
)

// Quad is a quadrilateral defined by its four corner points: top-left,
// top-right, bottom-right, bottom-left (zxing-cpp QuadrilateralF).
type Quad [4]bmcursor.PointF

// IsConvex reports whether the quadrilateral is convex (zxing-cpp
// IsConvex from Quadrilateral.h, including the M/m < 4 stability check).
func IsConvex(q Quad) bool {
	const inf = math.MaxFloat64
	m, M := inf, 0.0
	sign := false
	for i := range 4 {
		d1 := q[(i+2)%4].Sub(q[(i+1)%4])
		d2 := q[i].Sub(q[(i+1)%4])
		cp := bmcursor.CrossF(d1, d2)
		if a := math.Abs(cp); a < m {
			m = a
		} else if a > M {
			M = a
		}
		if i == 0 {
			sign = cp > 0
		} else if sign != (cp > 0) {
			return false
		}
	}
	return M/m < 4.0
}

// CenteredSquare returns the square with corners at +/- size/2 (integer
// division as in zxing-cpp CenteredSquare(int size)).
func CenteredSquare(size int) Quad {
	f := float64(size / 2)
	return Quad{
		{X: -f, Y: -f}, {X: f, Y: -f}, {X: f, Y: f}, {X: -f, Y: f},
	}
}

// MoveQuad translates every corner by off (zxing-cpp Move()).
func MoveQuad(q Quad, off bmcursor.PointF) Quad {
	var res Quad
	for i := range 4 {
		res[i] = q[i].Add(off)
	}
	return res
}

// RotatedCorners rotates the corner order by n and optionally mirrors it
// (zxing-cpp RotatedCorners()).
func RotatedCorners(q Quad, n int, mirror bool) Quad {
	var res Quad
	n = ((n % 4) + 4) % 4
	for i := range 4 {
		res[i] = q[(i+n)%4]
	}
	if mirror {
		res[1], res[3] = res[3], res[1]
	}
	return res
}

// RectangleQuad returns the quadrilateral {(x0+o,y0+o),(x1+o,y0+o),
// (x1+o,y1+o),(x0+o,y1+o)} (zxing-cpp Rectangle(x0, x1, y0, y1, o)).
func RectangleQuad(x0, x1, y0, y1 int, o float64) Quad {
	xl, xh := float64(x0)+o, float64(x1)+o
	yt, yb := float64(y0)+o, float64(y1)+o
	return Quad{{X: xl, Y: yt}, {X: xh, Y: yt}, {X: xh, Y: yb}, {X: xl, Y: yb}}
}

// PerspectiveTransform maps module coordinates to pixel coordinates
// (zxing-cpp PerspectiveTransform). The zero value is invalid; an invalid
// transform projects every point to NaN.
type PerspectiveTransform struct {
	a11, a12, a13 float64
	a21, a22, a23 float64
	a31, a32, a33 float64
	valid         bool
}

// rawPT assembles the matrix entries in zxing-cpp constructor order:
// {a11, a21, a31, a12, a22, a32, a13, a23, a33}.
func rawPT(a11, a21, a31, a12, a22, a32, a13, a23, a33 float64) PerspectiveTransform {
	return PerspectiveTransform{a11, a12, a13, a21, a22, a23, a31, a32, a33, true}
}

// NewPerspectiveTransform computes the transform from src to dst. It is
// invalid when either quadrilateral is not convex.
func NewPerspectiveTransform(src, dst Quad) PerspectiveTransform {
	if !IsConvex(src) || !IsConvex(dst) {
		return PerspectiveTransform{}
	}
	return unitSquareTo(dst).times(unitSquareTo(src).inverse())
}

// IsValid reports whether the transform was computed successfully.
func (t PerspectiveTransform) IsValid() bool { return t.valid }

func (t PerspectiveTransform) nan() PerspectiveTransform {
	n := math.NaN()
	return PerspectiveTransform{n, n, n, n, n, n, n, n, n, false}
}

// inverse returns the adjoint (zxing-cpp PerspectiveTransform::inverse).
func (t PerspectiveTransform) inverse() PerspectiveTransform {
	if !t.valid {
		return t.nan()
	}
	return rawPT(
		t.a22*t.a33-t.a23*t.a32,
		t.a23*t.a31-t.a21*t.a33,
		t.a21*t.a32-t.a22*t.a31,
		t.a13*t.a32-t.a12*t.a33,
		t.a11*t.a33-t.a13*t.a31,
		t.a12*t.a31-t.a11*t.a32,
		t.a12*t.a23-t.a13*t.a22,
		t.a13*t.a21-t.a11*t.a23,
		t.a11*t.a22-t.a12*t.a21)
}

// times returns this * other (zxing-cpp PerspectiveTransform::times).
func (t PerspectiveTransform) times(o PerspectiveTransform) PerspectiveTransform {
	if !t.valid || !o.valid {
		return t.nan()
	}
	return rawPT(
		t.a11*o.a11+t.a21*o.a12+t.a31*o.a13,
		t.a11*o.a21+t.a21*o.a22+t.a31*o.a23,
		t.a11*o.a31+t.a21*o.a32+t.a31*o.a33,
		t.a12*o.a11+t.a22*o.a12+t.a32*o.a13,
		t.a12*o.a21+t.a22*o.a22+t.a32*o.a23,
		t.a12*o.a31+t.a22*o.a32+t.a32*o.a33,
		t.a13*o.a11+t.a23*o.a12+t.a33*o.a13,
		t.a13*o.a21+t.a23*o.a22+t.a33*o.a23,
		t.a13*o.a31+t.a23*o.a32+t.a33*o.a33)
}

// unitSquareTo maps the unit square onto q
// (zxing-cpp PerspectiveTransform::UnitSquareTo).
func unitSquareTo(q Quad) PerspectiveTransform {
	x0, y0 := q[0].X, q[0].Y
	x1, y1 := q[1].X, q[1].Y
	x2, y2 := q[2].X, q[2].Y
	x3, y3 := q[3].X, q[3].Y

	d3 := q[0].Sub(q[1]).Add(q[2]).Sub(q[3])
	if d3.X == 0 && d3.Y == 0 {
		// Affine
		return rawPT(x1-x0, x2-x1, x0, y1-y0, y2-y1, y0, 0, 0, 1)
	}
	d1 := q[1].Sub(q[2])
	d2 := q[3].Sub(q[2])
	denominator := bmcursor.CrossF(d1, d2)
	a13 := bmcursor.CrossF(d3, d2) / denominator
	a23 := bmcursor.CrossF(d1, d3) / denominator
	return rawPT(x1-x0+a13*x1, x3-x0+a23*x3, x0,
		y1-y0+a13*y1, y3-y0+a23*y3, y0,
		a13, a23, 1)
}

// Apply projects a module coordinate to a pixel coordinate
// (zxing-cpp PerspectiveTransform::operator()).
func (t PerspectiveTransform) Apply(p bmcursor.PointF) bmcursor.PointF {
	denominator := t.a13*p.X + t.a23*p.Y + t.a33
	return bmcursor.PointF{X: (t.a11*p.X + t.a21*p.Y + t.a31) / denominator, Y: (t.a12*p.X + t.a22*p.Y + t.a32) / denominator}
}

// ROI is a region of interest of the sampled grid with its own projection
// (zxing-cpp GridSampler ROI).
type ROI struct {
	X0, X1, Y0, Y1 int
	Mod2Pix        PerspectiveTransform
}

// APPoint is an optional alignment pattern position
// (zxing-cpp std::optional<PointF>).
type APPoint struct {
	P  bmcursor.PointF
	OK bool
}

// SampleGrid samples a width x height module grid through mod2Pix
// (zxing-cpp SampleGrid(image, width, height, mod2Pix)). It returns the
// sampled bits and the projected symbol corners, or ok == false.
func SampleGrid(image *bitutil.BitMatrix, width, height int, mod2Pix PerspectiveTransform) (*bitutil.BitMatrix, [4]bmcursor.PointI, bool) {
	return SampleGridROIs(image, width, height, []ROI{{0, width, 0, height, mod2Pix}})
}

// SampleGridROIs samples a grid through a list of regions of interest
// (zxing-cpp SampleGrid(image, width, height, rois)).
func SampleGridROIs(image *bitutil.BitMatrix, width, height int, rois []ROI) (*bitutil.BitMatrix, [4]bmcursor.PointI, bool) {
	if width <= 0 || height <= 0 {
		return nil, [4]bmcursor.PointI{}, false
	}

	for _, roi := range rois {
		// Precheck the corners of every ROI to bail out early if the grid is
		// "obviously" not completely inside the image.
		isInside := func(roi ROI, x, y int) bool {
			return bmcursor.IsInF(image, roi.Mod2Pix.Apply(bmcursor.CenteredI(bmcursor.PointI{X: x, Y: y})))
		}
		if !roi.Mod2Pix.IsValid() || !isInside(roi, roi.X0, roi.Y0) || !isInside(roi, roi.X1-1, roi.Y0) ||
			!isInside(roi, roi.X1-1, roi.Y1-1) || !isInside(roi, roi.X0, roi.Y1-1) {
			return nil, [4]bmcursor.PointI{}, false
		}
	}

	res := bitutil.NewBitMatrixWithSize(width, height)
	for _, roi := range rois {
		for y := roi.Y0; y < roi.Y1; y++ {
			for x := roi.X0; x < roi.X1; x++ {
				p := roi.Mod2Pix.Apply(bmcursor.CenteredI(bmcursor.PointI{X: x, Y: y}))
				// Even when all boundary grid points project inside the image,
				// an inner grid point may not (numerical instability, #563).
				if !bmcursor.IsInF(image, p) {
					return nil, [4]bmcursor.PointI{}, false
				}
				if image.Get(int(p.X), int(p.Y)) {
					res.Set(x, y)
				}
			}
		}
	}

	projectCorner := func(p bmcursor.PointI) bmcursor.PointI {
		for _, roi := range rois {
			if roi.X0 <= p.X && p.X <= roi.X1 && roi.Y0 <= p.Y && p.Y <= roi.Y1 {
				return bmcursor.PointIF(roi.Mod2Pix.Apply(bmcursor.PointFI(p)).Add(bmcursor.PointF{X: 0.5, Y: 0.5}))
			}
		}
		return bmcursor.PointI{}
	}

	quad := [4]bmcursor.PointI{
		projectCorner(bmcursor.PointI{X: 0, Y: 0}),
		projectCorner(bmcursor.PointI{X: width, Y: 0}),
		projectCorner(bmcursor.PointI{X: width, Y: height}),
		projectCorner(bmcursor.PointI{X: 0, Y: height}),
	}
	return res, quad, true
}

// SampleGridAlignment samples a grid using found alignment pattern positions
// (zxing-cpp SampleGrid(image, width, height, mod2Pix, apP, apMX, apMY)).
// apP is indexed [y][x].
func SampleGridAlignment(image *bitutil.BitMatrix, width, height int, mod2Pix PerspectiveTransform, apP [][]APPoint, apMX, apMY []int) (*bitutil.BitMatrix, [4]bmcursor.PointI, bool) {
	W := len(apMX) - 1
	H := len(apMY) - 1

	// Fill any remaining gaps by a projection based on the fallback mod2Pix
	// projection (works if the symbol is flat).
	for y := range H + 1 {
		for x := range W + 1 {
			if !apP[y][x].OK {
				apP[y][x] = APPoint{mod2Pix.Apply(bmcursor.CenteredI(bmcursor.PointI{X: apMX[x], Y: apMY[y]})), true}
			}
		}
	}
	var rois []ROI
	for y := range H {
		for x := range W {
			x0, x1 := apMX[x], apMX[x+1]
			y0, y1 := apMY[y], apMY[y+1]
			dst := Quad{apP[y][x].P, apP[y][x+1].P, apP[y+1][x+1].P, apP[y+1][x].P}
			rois = append(rois, ROI{
				X0: x0, Y0: y0, X1: x1, Y1: y1,
				Mod2Pix: NewPerspectiveTransform(RectangleQuad(x0, x1, y0, y1, 0.5), dst),
			})
			r := &rois[len(rois)-1]
			if x == 0 {
				r.X0 = 0
			}
			if x == W-1 {
				r.X1 = width
			}
			if y == 0 {
				r.Y0 = 0
			}
			if y == H-1 {
				r.Y1 = height
			}
		}
	}

	return SampleGridROIs(image, width, height, rois)
}
