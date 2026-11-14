// Package concentric ports the ConcentricFinder machinery from zxing-cpp
// (core/src/ConcentricFinder.h/.cpp): ring/center estimation and square
// fitting around concentric finder patterns such as the Aztec bullseye.
package concentric

import (
	"math"

	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/bmcursor"
	"github.com/fabiomarini/zxinggo/internal/localgrid"
)

// ConcentricPattern is a found concentric pattern: center and size
// (zxing-cpp ConcentricPattern).
type ConcentricPattern struct {
	P    bmcursor.PointF
	Size float64
}

// aztecFinderPattern is the 7x7 bullseye pattern {1,1,1,1,1,1,1}.
var aztecFinderPattern = []int{1, 1, 1, 1, 1, 1, 1}

// averageEdgePixels averages the edge pixels along the cursor direction
// (zxing-cpp ConcentricFinder.cpp AverageEdgePixels).
func averageEdgePixels(cur bmcursor.CursorI, rng, numOfEdges int) (ConcentricPattern, bool) {
	sum := bmcursor.PointF{}
	totalSteps := 0
	for range numOfEdges {
		steps := cur.StepToEdge(1, rng-totalSteps, false, true)
		if steps == 0 {
			return ConcentricPattern{}, false
		}
		totalSteps += steps
		sum = sum.Add(bmcursor.CenteredI(cur.P)).Add(bmcursor.CenteredI(cur.P.Add(cur.Back())))
	}
	return ConcentricPattern{sum.Mul(1 / float64(2*numOfEdges)), float64(totalSteps)}, true
}

// centerOfDoubleCross estimates the center from a double cross of edges
// (zxing-cpp ConcentricFinder.cpp CenterOfDoubleCross).
func centerOfDoubleCross(image *bitutil.BitMatrix, center bmcursor.PointI, width, numOfEdges int) (ConcentricPattern, bool) {
	sumP := bmcursor.PointF{}
	sumS := 0.0
	for _, d := range [4]bmcursor.PointI{{X: 0, Y: 1}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 1, Y: -1}} {
		c1 := bmcursor.NewCursorI(image, center, d)
		c2 := bmcursor.NewCursorI(image, center, d.Neg())
		avr1, ok1 := averageEdgePixels(c1, width*3/5, numOfEdges)
		avr2, ok2 := averageEdgePixels(c2, width*3/5, numOfEdges)
		if !ok1 || !ok2 {
			return ConcentricPattern{}, false
		}
		m, M := avr1.Size, avr2.Size
		if m > M {
			m, M = M, m
		}
		if M > 1.2*m+1 { // only accept if the two legs are very similar in size
			return ConcentricPattern{}, false
		}
		sumP = sumP.Add(avr1.P).Add(avr2.P)
		sumS += avr1.Size + avr2.Size
	}
	return ConcentricPattern{sumP.Mul(1.0 / 8), 2 * sumS / 8}, true
}

// CenterOfRing walks the nth ring around center and estimates its center and
// size (zxing-cpp ConcentricFinder.cpp CenterOfRing).
func CenterOfRing(image *bitutil.BitMatrix, center bmcursor.PointI, width, nth int, requireCircle bool) (ConcentricPattern, bool) {
	meanRToSquareWidth := func(r float64) float64 { return 1.74 * r } // square with mean radius r has width ~1.74*r

	maxN := 4 * (width + 1) * 3 / 2 // upper limit for circumference of the ring
	maxR := 2 * width
	if requireCircle {
		maxR = width + 1
	}
	inner := nth < 0
	if nth < 0 {
		nth = -nth
	}

	cur := bmcursor.NewCursorI(image, center, bmcursor.PointI{X: 1, Y: 0})
	if cur.StepToEdge(nth, maxR, inner, true) == 0 {
		return ConcentricPattern{}, false
	}
	cur.TurnRight() // move clockwise and keep edge on the right/left depending on backup
	edgeDir := bmcursor.Right
	if inner {
		edgeDir = bmcursor.Left
	}

	neighbourMask := uint32(0)
	start := cur.P
	sumP := bmcursor.PointF{}
	sumR := 0.0
	nP := 0
	for {
		sumP = sumP.Add(bmcursor.CenteredI(cur.P))
		nP++

		// find out if we come full circle around the center; 8 bits have to
		// be set in the end.
		bd := bmcursor.BresenhamI(cur.P.Sub(center))
		neighbourMask |= 1 << (4 + bmcursor.DotI(bd, bmcursor.PointI{X: 1, Y: 3}))

		if !cur.StepAlongEdge(edgeDir, false) {
			return ConcentricPattern{}, false
		}

		r := bmcursor.DistanceFI(cur.P, center)
		if inner {
			r += 0.5 // go from pixel center to pixel edge
		} else {
			r -= 0.5
		}
		sumR += r
		if r > float64(maxR) || cur.P == center || nP > maxN {
			return ConcentricPattern{}, false
		}
		if cur.P == start {
			break
		}
	}

	if requireCircle && neighbourMask != 0b111101111 {
		return ConcentricPattern{}, false
	}

	meanP := sumP.Mul(1 / float64(nP))
	meanR := sumR / float64(nP)
	C := float64(nP) / meanR // edge pixels per unit length: circle 2*pi, square 8
	centerMove := bmcursor.DistanceF(meanP, bmcursor.PointFI(center)) / float64(width)
	// C > 12 means the edge is very irregular; centerMove > 0.5 means the
	// center moved more than half the estimated width of the ring
	if requireCircle && (C > 12 || centerMove > 0.5) {
		return ConcentricPattern{}, false
	}

	return ConcentricPattern{meanP, meanRToSquareWidth(meanR)}, true
}

// CenterOfRings averages the centers of the rings 2..numOfRings
// (zxing-cpp ConcentricFinder.cpp CenterOfRings).
func CenterOfRings(image *bitutil.BitMatrix, center bmcursor.PointF, rng, numOfRings int) (ConcentricPattern, bool) {
	n := 1
	size := 0.0
	sum := center
	for i := 2; i < numOfRings+1; i++ {
		c, ok := CenterOfRing(image, bmcursor.PointIF(center), rng, i, true)
		if !ok {
			if n == 1 {
				return ConcentricPattern{}, false
			}
			break
		} else if bmcursor.DistanceF(c.P, center) > float64(rng)/float64(numOfRings)/2 {
			return ConcentricPattern{}, false
		}
		sum = sum.Add(c.P)
		size = c.Size
		n++
	}
	if n == numOfRings {
		return ConcentricPattern{sum.Mul(1 / float64(n)), size}, true
	}
	return ConcentricPattern{sum.Mul(1 / float64(n)), 0}, true
}

// collectRingPoints walks the ring and collects its (centered) points
// (zxing-cpp ConcentricFinder.cpp CollectRingPoints).
func collectRingPoints(image *bitutil.BitMatrix, center bmcursor.PointF, width, edgeIndex int, backup bool) []bmcursor.PointF {
	centerI := bmcursor.PointIF(center)
	maxN := 4 * (width + 1) * 3 / 2 // upper limit for circumference of the ring
	maxR := width * 2

	cur := bmcursor.NewCursorI(image, centerI, bmcursor.PointI{X: 1, Y: 0})
	if cur.StepToEdge(edgeIndex, maxR, backup, true) == 0 {
		return nil
	}
	cur.TurnRight() // move clockwise and keep edge on the right/left depending on backup
	edgeDir := bmcursor.Right
	if backup {
		edgeDir = bmcursor.Left
	}

	neighbourMask := uint32(0)
	start := cur.P
	points := make([]bmcursor.PointF, 0, maxN/2)

	for {
		points = append(points, bmcursor.CenteredI(cur.P))

		// find out if we come full circle around the center
		bd := bmcursor.BresenhamI(cur.P.Sub(centerI))
		neighbourMask |= 1 << (4 + bmcursor.DotI(bd, bmcursor.PointI{X: 1, Y: 3}))

		if !cur.StepAlongEdge(edgeDir, false) {
			return nil
		}

		if bmcursor.DistanceFI(cur.P, centerI) > float64(maxR) || cur.P == centerI || len(points) > maxN {
			return nil
		}
		if cur.P == start {
			break
		}
	}

	if neighbourMask != 0b111101111 {
		return nil
	}
	return points
}

// FinetuneConcentricPatternCenter refines a candidate center by ring analysis
// (zxing-cpp ConcentricFinder.cpp FinetuneConcentricPatternCenter).
func FinetuneConcentricPatternCenter(image *bitutil.BitMatrix, center bmcursor.PointF, width, finderPatternSize int) (ConcentricPattern, bool) {
	// make sure we have at least one path of white around the center
	res1, ok := CenterOfRing(image, bmcursor.PointIF(center), width*2/3, 1, true)
	if ok && image.Get(int(res1.P.X), int(res1.P.Y)) {
		// and then either at least one more ring around that
		res2, ok := CenterOfRings(image, res1.P, width, finderPatternSize/2)
		if ok && image.Get(int(res2.P.X), int(res2.P.Y)) {
			res2.Size = res2.Size * 7 / 5 // CenterOfRings only estimates the radius of the white ring
			return res2, true
		}
		// or the center can be approximated by a square
		res2, ok = centerOfDoubleCross(image, bmcursor.PointIF(res1.P), width, finderPatternSize/2+1)
		if ok && image.Get(int(res2.P.X), int(res2.P.Y)) {
			return res2, true
		}
	}
	return ConcentricPattern{}, false
}

// checkSymmetricPattern checks a symmetric pattern around the cursor in
// direction cur.D using fast edge counters (zxing-cpp ConcentricFinder.h
// CheckSymmetricPattern, specialized for the 7x7 finder pattern).
func checkSymmetricPattern(cur *bmcursor.CursorI, rng int, updatePosition, relaxed bool) int {
	curFwd := bmcursor.NewEdgeCounter(*cur)
	curBwd := bmcursor.NewEdgeCounter(cur.TurnedBack())

	centerFwd := curFwd.StepToNextEdge(rng)
	if centerFwd == 0 {
		return 0
	}
	centerBwd := curBwd.StepToNextEdge(rng)
	if centerBwd == 0 {
		return 0
	}

	res := make([]int, 7)
	res[3] = centerFwd + centerBwd - 1 // -1 because the starting pixel is counted twice
	rng -= res[3]

	for i := 1; i <= 3; i++ {
		v := curFwd.StepToNextEdge(rng)
		res[3+i] = v
		rng -= v
		if v == 0 {
			return 0
		}
		v = curBwd.StepToNextEdge(rng)
		res[3-i] = v
		rng -= v
		if v == 0 {
			return 0
		}
	}

	if !isFinderPattern(res, relaxed) {
		return 0
	}

	if updatePosition {
		cur.Step(res[3]/2 - (centerBwd - 1))
	}

	sum := 0
	for _, v := range res {
		sum += v
	}
	return sum
}

// isFinderPattern is zxing-cpp IsPattern for the fixed {1,1,1,1,1,1,1}
// pattern (non-sparse path).
func isFinderPattern(res []int, relaxed bool) bool {
	width := 0
	for _, v := range res {
		width += v
	}
	moduleSize := float64(width) / 7.0
	threshold := moduleSize*(0.5+bool2float(relaxed)*0.25) + 0.5
	for _, v := range res {
		if math.Abs(float64(v)-moduleSize) > threshold {
			return false
		}
	}
	return true
}

func bool2float(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// LocateConcentricPattern verifies the 7x7 finder pattern around center and
// returns the refined center (zxing-cpp ConcentricFinder.h
// LocateConcentricPattern, specialized for the Aztec finder pattern).
// relaxed selects the IsPattern threshold of the axis-aligned checks
// (zxing-cpp template parameter E2E; the only instantiation used by the
// Aztec detector is relaxed == true).
func LocateConcentricPattern(image *bitutil.BitMatrix, center bmcursor.PointF, width int, relaxed bool) (ConcentricPattern, bool) {
	cur := bmcursor.NewCursorI(image, bmcursor.PointIF(center), bmcursor.PointI{})
	rng := width * 2
	minSpread, maxSpread := image.Width(), 0
	// TODO: setting maxError to 1 can substantially help with symbols with
	// damaged finder patterns, but it substantially increases the runtime.
	maxError := 0
	for _, d := range [2]bmcursor.PointI{{X: 0, Y: 1}, {X: 1, Y: 0}} {
		cur.SetDirection(d)
		spread := checkSymmetricPattern(&cur, rng, true, relaxed)
		if spread != 0 {
			if spread < minSpread {
				minSpread = spread
			}
			if spread > maxSpread {
				maxSpread = spread
			}
		} else {
			maxError--
			if maxError < 0 {
				return ConcentricPattern{}, false
			}
		}
	}

	for _, d := range [2]bmcursor.PointI{{X: 1, Y: 1}, {X: 1, Y: -1}} {
		cur.SetDirection(d)
		spread := checkSymmetricPattern(&cur, rng, false, true)
		if spread != 0 {
			if spread < minSpread {
				minSpread = spread
			}
			if spread > maxSpread {
				maxSpread = spread
			}
		} else {
			maxError--
			if maxError < 0 {
				return ConcentricPattern{}, false
			}
		}
	}

	if maxSpread > 5*minSpread {
		return ConcentricPattern{}, false
	}

	newCenter, ok := FinetuneConcentricPatternCenter(image, bmcursor.PointFI(cur.P), width, len(aztecFinderPattern))
	if !ok {
		return ConcentricPattern{}, false
	}
	if newCenter.Size == 0 {
		newCenter.Size = float64(maxSpread+minSpread) / 2
	}
	return newCenter, true
}

// FindConcentricPatternCorners finds the corners of a concentric pattern by
// fitting squares to the ringIndex'th ring around the center; a negative
// ringIndex finds the switch between 2 rings instead of the center of a ring
// (zxing-cpp ConcentricFinder.cpp FindConcentricPatternCorners).
func FindConcentricPatternCorners(image *bitutil.BitMatrix, center bmcursor.PointF, width, ringIndex int) (localgrid.Quad, bool) {
	innerCorners, ok := FitSquareToPoints(image, center, width, abs(ringIndex), ringIndex < 0)
	if !ok {
		return localgrid.Quad{}, false
	}
	outerCorners, ok := FitSquareToPoints(image, center, width, abs(ringIndex)+bool2int(ringIndex > 0), ringIndex > 0)
	if !ok {
		return localgrid.Quad{}, false
	}
	return blend(innerCorners, outerCorners), true
}

func bool2int(b bool) int {
	if b {
		return 1
	}
	return 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// blend averages two quadrilaterals, aligning them by rotating b such that
// its corner closest to a[0] comes first (zxing-cpp Quadrilateral.h Blend).
func blend(a, b localgrid.Quad) localgrid.Quad {
	offset := 0
	best := math.Inf(1)
	for i := range 4 {
		if d := bmcursor.DistanceF(b[i], a[0]); d < best {
			best = d
			offset = i
		}
	}
	var res localgrid.Quad
	for i := range 4 {
		res[i] = a[i].Add(b[(i+offset)%4]).Mul(0.5)
	}
	return res
}
