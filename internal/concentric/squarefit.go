package concentric

import (
	"math"

	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/bmcursor"
	"github.com/fabiomarini/zxinggo/internal/localgrid"
)

// regLine is a minimal port of zxing-cpp RegressionLine for the two-point /
// point-range fits used by FitQuadrilateralToPoints.
type regLine struct {
	a, b, c float64
	valid   bool
}

// newRegLine fits a line to points[begin:end] (zxing-cpp
// RegressionLine::evaluate; the inward direction is neutral here).
func newRegLine(points []bmcursor.PointF, begin, end int) regLine {
	n := end - begin
	if n <= 0 {
		return regLine{}
	}
	mean := bmcursor.PointF{}
	for _, p := range points[begin:end] {
		mean = mean.Add(p)
	}
	mean = mean.Mul(1 / float64(n))

	var sumXX, sumYY, sumXY float64
	for _, p := range points[begin:end] {
		d := p.Sub(mean)
		sumXX += d.X * d.X
		sumYY += d.Y * d.Y
		sumXY += d.X * d.Y
	}

	var a, b float64
	if sumYY >= sumXX {
		l := math.Sqrt(sumYY*sumYY + sumXY*sumXY)
		a = +sumYY / l
		b = -sumXY / l
	} else {
		l := math.Sqrt(sumXX*sumXX + sumXY*sumXY)
		a = +sumXY / l
		b = -sumXX / l
	}
	c := a*mean.X + b*mean.Y
	return regLine{a, b, c, !math.IsNaN(a)}
}

// distance returns the absolute distance of p to the line.
func (l regLine) distance(p bmcursor.PointF) float64 {
	return math.Abs(l.a*p.X + l.b*p.Y - l.c)
}

// intersect returns the intersection point of two lines (zxing-cpp
// RegressionLine.h intersect).
func intersect(l1, l2 regLine) bmcursor.PointF {
	d := l1.a*l2.b - l1.b*l2.a
	x := (l1.c*l2.b - l1.b*l2.c) / d
	y := (l1.a*l2.c - l1.c*l2.a) / d
	return bmcursor.PointF{x, y}
}

// fitQuadrilateralToPoints fits a quadrilateral to a closed ring of points
// (zxing-cpp ConcentricFinder.cpp FitQuadrilateralToPoints).
func fitQuadrilateralToPoints(center bmcursor.PointF, points []bmcursor.PointF) (localgrid.Quad, bool) {
	var res localgrid.Quad
	n := len(points)

	// std::minmax_element returns the first minimum and the last maximum.
	minIdx, maxIdx := 0, 0
	for i := 1; i < n; i++ {
		if bmcursor.DistanceF(points[i], center) < bmcursor.DistanceF(points[minIdx], center) {
			minIdx = i
		}
		if !(bmcursor.DistanceF(points[i], center) < bmcursor.DistanceF(points[maxIdx], center)) {
			maxIdx = i
		}
	}

	// check if points are on a circle: for a square the min/max ratio is
	// 0.7, for a circle it is 1
	if bmcursor.DistanceF(center, points[minIdx])/bmcursor.DistanceF(center, points[maxIdx]) > 0.85 {
		return res, false
	}

	// rotate points such that the first one is the furthest away from the
	// center (hence, a corner)
	rotated := make([]bmcursor.PointF, 0, n)
	rotated = append(rotated, points[maxIdx:]...)
	rotated = append(rotated, points[:maxIdx]...)
	points = rotated

	// find the opposite corner by looking for the farthest point near the
	// opposite point
	corners := [4]int{0, 0, 0, 0}
	corners[2] = maxBy(n, n*3/8, n*5/8, func(i int) float64 {
		return bmcursor.DistanceF(points[i], center)
	})
	// find the two in between corners by looking for the points farthest
	// from the long diagonal
	line := newRegLineFrom2Points(points[corners[0]], points[corners[2]])
	dist2Diagonal := func(i int) float64 { return line.distance(points[i]) }
	corners[1] = maxBy(n, n*1/8, n*3/8, dist2Diagonal)
	corners[3] = maxBy(n, n*5/8, n*7/8, dist2Diagonal)

	var lines [4]regLine
	beg := [4]int{corners[0] + 1, corners[1] + 1, corners[2] + 1, corners[3] + 1}
	end := [4]int{corners[1], corners[2], corners[3], n}
	for i := range 4 {
		lines[i] = newRegLine(points, beg[i], end[i])
		if !lines[i].valid {
			return res, false
		}
	}

	// check if all points belonging to each line segment are sufficiently
	// close to that line
	for i := range 4 {
		l := end[i] - beg[i]
		if l <= 3 {
			continue
		}
		for j := beg[i]; j < end[i]; j++ {
			if lines[i].distance(points[j]) > math.Max(1, math.Min(8, float64(l)/8)) {
				return res, false
			}
		}
	}

	for i := range 4 {
		res[i] = intersect(lines[i], lines[(i+1)%4])
	}
	return res, true
}

// newRegLineFrom2Points fits a line through two points.
func newRegLineFrom2Points(a, b bmcursor.PointF) regLine {
	return newRegLine([]bmcursor.PointF{a, b}, 0, 2)
}

// maxBy returns the index of the first maximum of f over [lo, hi), clamped
// into [0, n) (zxing-cpp std::max_element semantics).
func maxBy(n, lo, hi int, f func(int) float64) int {
	if lo < 0 {
		lo = 0
	}
	if hi > n {
		hi = n
	}
	best := lo
	bestV := f(lo)
	for i := lo + 1; i < hi; i++ {
		if v := f(i); v > bestV {
			bestV = v
			best = i
		}
	}
	return best
}

// quadrilateralIsPlausibleSquare checks side-length plausibility (zxing-cpp
// ConcentricFinder.cpp QuadrilateralIsPlausibleSquare).
func quadrilateralIsPlausibleSquare(q localgrid.Quad, lineIndex int) bool {
	m := bmcursor.DistanceF(q[0], q[3])
	M := m
	for i := 1; i < 4; i++ {
		if d := bmcursor.DistanceF(q[i-1], q[i]); d < m {
			m = d
		} else if d > M {
			M = d
		}
	}
	return m >= float64(lineIndex*2) && m > M/3
}

// FitSquareToPoints fits a square to the lineIndex'th ring of points around
// center (zxing-cpp ConcentricFinder.cpp FitSquareToPoints).
func FitSquareToPoints(image *bitutil.BitMatrix, center bmcursor.PointF, width, lineIndex int, backup bool) (localgrid.Quad, bool) {
	var res localgrid.Quad
	points := collectRingPoints(image, center, width, lineIndex, backup)
	if len(points) == 0 {
		return res, false
	}
	q, ok := fitQuadrilateralToPoints(center, points)
	if !ok {
		return res, false
	}
	if !quadrilateralIsPlausibleSquare(q, lineIndex-bool2int(backup)) {
		return res, false
	}
	return q, true
}
