package localgrid

import (
	"math"
	"sort"

	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal/bmcursor"
)

// LocalGrid represents a local grid for sampling and analyzing regions within
// a BitMatrix (ported from zxing-cpp core/src/LocalGrid.h/.cpp). It tries to
// find the center of a module and step vectors pointing to the next module in
// x and y direction, refining them from edge-to-edge distances.
type LocalGrid struct {
	Img     *bitutil.BitMatrix
	Mod2Pix PerspectiveTransform
	Dim     bmcursor.PointI
	Center  bmcursor.PointI
	Origin  bmcursor.PointF
	StepX   bmcursor.PointF
	StepY   bmcursor.PointF
}

// New creates a LocalGrid for the given global transform and grid dimensions.
func New(image *bitutil.BitMatrix, mod2Pix PerspectiveTransform, dim bmcursor.PointI) LocalGrid {
	return LocalGrid{Img: image, Mod2Pix: mod2Pix, Dim: dim}
}

// GetPos maps a module-space offset (relative to the grid center) to pixels.
func (g LocalGrid) GetPos(p bmcursor.PointF) bmcursor.PointF {
	return g.Origin.Add(g.StepX.Mul(p.X)).Add(g.StepY.Mul(p.Y))
}

// GetPosI maps an integer module-space offset to pixels.
func (g LocalGrid) GetPosI(p bmcursor.PointI) bmcursor.PointF {
	return g.GetPos(bmcursor.PointF{X: float64(p.X), Y: float64(p.Y)})
}

// Get returns the image value at a module-space offset.
func (g LocalGrid) Get(p bmcursor.PointF) bmcursor.Value {
	return bmcursor.TestAtF(g.Img, g.GetPos(p))
}

// GetI returns the image value at an integer module-space offset.
func (g LocalGrid) GetI(p bmcursor.PointI) bmcursor.Value {
	return g.Get(bmcursor.PointF{X: float64(p.X), Y: float64(p.Y)})
}

func (g *LocalGrid) moveBy(offset bmcursor.PointI) {
	g.Origin = g.GetPosI(offset)
	g.Center = g.Center.Add(offset)
}

// At sets/finds the origin of the local grid near the given module
// coordinates and refines origin and step vectors from the image (zxing-cpp
// LocalGrid::at).
func (g *LocalGrid) At(p, offset bmcursor.PointI) {
	g.Center = p
	c := bmcursor.CenteredI(p)
	g.Origin = g.Mod2Pix.Apply(c)
	g.StepX = g.Mod2Pix.Apply(c.Add(bmcursor.PointF{X: 1, Y: 0})).Sub(g.Origin)
	g.StepY = g.Mod2Pix.Apply(c.Add(bmcursor.PointF{X: 0, Y: 1})).Sub(g.Origin)

	offsets := []bmcursor.PointI{
		{X: -1, Y: 0}, {X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: -1, Y: 1},
	}

	g.moveBy(offset)
	g.adjustOriginAndStep(false, 0, 0, offsets, false)
	g.adjustOriginAndStep(true, 0, 0, offsets, false)
	for range 2 {
		g.adjustOriginAndStep(false, 0, 2, offsets, false)
		g.adjustOriginAndStep(true, 0, 2, offsets, false)
	}
	g.moveBy(offset.Neg())
}

// findValue checks whether there is a value v at p or a quarter module
// before or after p (zxing-cpp LocalGrid::findValue).
func (g LocalGrid) findValue(p bmcursor.PointI, r int, dir bmcursor.PointI, v bmcursor.Value) bool {
	d := bmcursor.PointF{}
	if dir.X != 0 {
		d.X = 0.25
	}
	if dir.Y != 0 {
		d.Y = 0.25
	}
	p = p.Add(dir.Mul(r))
	pf := bmcursor.PointF{X: float64(p.X), Y: float64(p.Y)}
	return g.Get(pf) == v || g.Get(pf.Add(d)) == v || g.Get(pf.Sub(d)) == v
}

// clusterAvg returns the average of the largest cluster of values within
// threshold of each other and whether there were values outside of it
// (zxing-cpp LocalGrid.cpp clusterAvg).
func clusterAvg(v []float64, threshold float64) (float64, bool) {
	sort.Float64s(v)
	bestStart, bestLen, start := 0, 0, 0
	for end := range len(v) {
		for v[end]-v[start] >= threshold {
			start++
		}
		if end-start+1 > bestLen {
			bestLen = end - start + 1
			bestStart = start
		}
	}
	sum := 0.0
	for i := bestStart; i < bestStart+bestLen; i++ {
		sum += v[i]
	}
	sum /= float64(bestLen)
	return sum, bestLen < len(v)
}

// runLength is the number of pixels in a run given the steps to the edges in
// both directions (0 if unknown).
func runLength(stepsPos, stepsNeg int) int {
	if stepsPos != 0 && stepsNeg != 0 {
		return stepsPos + stepsNeg - 1
	}
	return 0
}

// estimateModSize estimates the module size from edge-to-edge distances;
// returns false if there is no (significant) improvement.
func (g LocalGrid) estimateModSize(dir bmcursor.PointF, modSize float64, limit, last int, offsets []bmcursor.PointI) (float64, bool) {
	// edge-to-edge distances are quantized to whole pixels, so they can't
	// refine the module size of (near) 1px/module symbols
	if last <= 0 || modSize < 1.5 {
		return 0, false
	}

	var e2e []int

	// Go from the origin outward along dir and collect edge-to-edge distances.
	scanE2ERay := func(start, dir bmcursor.PointF) {
		if !bmcursor.IsInF(g.Img, start) {
			return
		}
		minScan := int(math.Ceil(float64(last) * modSize))

		cursor := bmcursor.NewCursorF(g.Img, start, dir)
		reverse := bmcursor.NewCursorF(g.Img, start, dir.Neg())
		stepsPos := cursor.StepToEdge(1, limit, false, false)
		stepsNeg := reverse.StepToEdge(1, limit, false, false)
		if stepsPos == 0 {
			return
		}

		previousRun := runLength(stepsPos, stepsNeg)
		scanned := stepsPos
		for scanned < limit {
			run := cursor.StepToEdge(1, limit-scanned, false, false)
			if run == 0 {
				break
			}
			scanned += run
			if previousRun != 0 {
				if previousRun+run > limit {
					break
				}
				e2e = append(e2e, previousRun+run)
				if scanned >= minScan {
					break
				}
			}
			previousRun = run
		}
	}

	for _, o := range offsets {
		if math.Hypot(float64(o.X), float64(o.Y)) > 1 {
			continue
		}
		oDir := g.StepX.Mul(float64(o.X)).Add(g.StepY.Mul(float64(o.Y)))
		l := bmcursor.LengthF(oDir)
		if l == 0 {
			continue
		}
		oDir = oDir.Mul(1 / l)
		d := bmcursor.DotF(oDir, dir)
		switch {
		case d > 0.99:
			scanE2ERay(g.Origin, dir)
		case d < -0.99:
			scanE2ERay(g.Origin, dir.Neg())
		default:
			for i := 1; i <= last; i++ {
				pos := g.GetPosI(bmcursor.PointI{X: i * o.X, Y: i * o.Y})
				scanE2ERay(pos, dir.Neg())
				scanE2ERay(pos, dir)
			}
		}
	}

	fitModSize := func(e2e []int, estimate float64) (float64, int) {
		bestMS := estimate
		bestErr := math.Inf(1)
		spread := 0.75
		if len(e2e) <= 1 {
			spread = (estimate - 0.5) / estimate
		}
		minMS := estimate * spread
		maxMS := estimate / spread
		bestN := 0

		// Huber loss with delta 1
		huberLoss := func(d float64) float64 {
			if math.Abs(d) < 1 {
				return 0.5 * d * d
			}
			return math.Abs(d) - 0.5
		}

		for ms := minMS; ms <= maxMS; ms += estimate * 0.01 {
			err, weightSum := 0.0, 0.0
			n, sumMods := 0, 0

			for _, steps := range e2e {
				if float64(steps) < estimate {
					continue
				}
				mods := math.Round(float64(steps) / ms)
				d := float64(steps) - mods*ms
				weight := estimate / float64(steps)
				err += weight * huberLoss(d)
				weightSum += weight
				n++
				sumMods += int(mods)
			}

			if weightSum > 0 {
				err /= weightSum
			} else {
				err /= 1
			}
			avgMods := 0.0
			if n > 0 {
				avgMods = float64(sumMods) / float64(n) * 0.2
			}
			err += avgMods + huberLoss(ms-estimate)*0.5

			if err < bestErr {
				bestErr = err
				bestMS = ms
				bestN = n
			}
		}

		if bestMS == minMS || bestMS == maxMS {
			bestMS = estimate
		}
		return bestMS, bestN
	}

	bestMS, n := fitModSize(e2e, modSize)
	if n < len(e2e)/2 || math.Abs(bestMS-modSize) <= modSize*0.02 {
		return 0, false
	}
	return bestMS, true
}

// nearestHalfResidual is the distance of n to the nearest position that is
// half a period s away from a multiple of s.
func nearestHalfResidual(n, s float64) float64 {
	return n - (math.Round((n-s/2)/s)*s + s/2)
}

// estimateOriginShift estimates the shift of the origin along dir from the
// distances to the nearest edges.
func (g LocalGrid) estimateOriginShift(dir bmcursor.PointF, modSize float64, limit, first, last int, offsets []bmcursor.PointI, trimToGrid bool) (float64, bool) {
	var deltas []float64
	for i := first; i <= last; i++ {
		for _, offset := range offsets {
			p := g.Center.Add(offset.Mul(i))
			start := g.GetPosI(offset.Mul(i))
			startC := bmcursor.CenteredF(start)
			if !bmcursor.IsInF(g.Img, startC) ||
				(trimToGrid && (p.X < 0 || p.Y < 0 || p.X >= g.Dim.X || p.Y >= g.Dim.Y)) {
				continue
			}
			curPos := bmcursor.NewCursorF(g.Img, startC, dir)
			curNeg := bmcursor.NewCursorF(g.Img, startC, dir.Neg())
			stepsPos := curPos.StepToEdge(1, limit, false, false)
			stepsNeg := curNeg.StepToEdge(1, limit, false, false)
			if stepsPos == 0 && stepsNeg == 0 {
				continue // no edge found, no information
			}
			// +0.5 because the center of the pixel is at .5, .5
			diff := start.Sub(startC)
			distPos := 0.0
			if stepsPos != 0 {
				distPos = float64(stepsPos) - bmcursor.DotF(diff, dir) - 0.5
			}
			distNeg := 0.0
			if stepsNeg != 0 {
				distNeg = float64(stepsNeg) - bmcursor.DotF(diff, dir.Neg()) - 0.5
			}

			// signed distance to the nearest edge (preferably in positive direction)
			dist := 0.0
			switch {
			case stepsPos != 0:
				dist = distPos
			case stepsNeg != 0:
				dist = -distNeg
			}

			// if both edges are known and the run is short enough, use it to
			// refine the local module size
			localModSize := modSize
			if run := runLength(stepsPos, stepsNeg); run != 0 && float64(run)/modSize < 6 {
				mods := math.Max(1, math.Round(float64(run)/modSize))
				localModSize = float64(run) / mods
			}

			if dist < 0 {
				deltas = append(deltas, -nearestHalfResidual(-dist, localModSize))
			} else {
				deltas = append(deltas, nearestHalfResidual(dist, localModSize))
			}

			if i == 0 {
				break // only do the center point once
			}
		}
	}

	if len(deltas) == 0 {
		return 0, false
	}

	meanDelta, hasOutliers := clusterAvg(deltas, math.Max(1.0, modSize/2))
	if hasOutliers {
		// wrap the outliers by one module and try again
		for i := range deltas {
			if deltas[i] < meanDelta-modSize/2 {
				deltas[i] += modSize
			} else if deltas[i] > meanDelta+modSize/2 {
				deltas[i] -= modSize
			}
		}
		meanDelta, _ = clusterAvg(deltas, math.Max(1.0, modSize/3))
	}
	return meanDelta, true
}

// adjustOriginAndStep refines the step vector along axis and shifts the
// origin along it (zxing-cpp LocalGrid::adjustOriginAndStep). axis == true
// selects the Y axis.
func (g *LocalGrid) adjustOriginAndStep(axis bool, first, last int, offsets []bmcursor.PointI, trimToGrid bool) bool {
	if len(offsets) == 0 || first > last {
		return false
	}

	step := &g.StepX
	if axis {
		step = &g.StepY
	}
	dir := bmcursor.BresenhamF(*step)
	modSize := bmcursor.LengthF(*step) / bmcursor.LengthF(dir) // mod size in steps along dir
	limit := int(8 * modSize)

	if ms, ok := g.estimateModSize(dir, modSize, limit, last, offsets); ok {
		*step = step.Mul(ms / modSize)
		modSize = ms
	}

	shift, ok := g.estimateOriginShift(dir, modSize, limit, first, last, offsets, trimToGrid)
	if !ok {
		return false
	}
	g.Origin = g.Origin.Add(dir.Mul(shift))
	return true
}

// tuneFromTiming adjusts origin and step from the timing pattern in the given
// directions (zxing-cpp LocalGrid::tuneFromTiming). dirs contains any of
// 'l', 'r', 'u', 'd'.
func (g *LocalGrid) tuneFromTiming(start bmcursor.PointI, dirs string, radius int) bool {
	var dirX, dirY []bmcursor.PointI
	if containsByte(dirs, 'l') {
		dirX = append(dirX, bmcursor.PointI{X: -1, Y: 0})
	}
	if containsByte(dirs, 'r') {
		dirX = append(dirX, bmcursor.PointI{X: 1, Y: 0})
	}
	if containsByte(dirs, 'u') {
		dirY = append(dirY, bmcursor.PointI{X: 0, Y: -1})
	}
	if containsByte(dirs, 'd') {
		dirY = append(dirY, bmcursor.PointI{X: 0, Y: 1})
	}

	g.moveBy(start)
	resX := g.adjustOriginAndStep(false, 0, radius, dirX, true)
	resY := g.adjustOriginAndStep(true, 0, radius, dirY, true)
	g.moveBy(start.Neg())
	return resX || resY
}

func containsByte(s string, c byte) bool {
	for i := range len(s) {
		if s[i] == c {
			return true
		}
	}
	return false
}

// isTimingPatternCross checks whether the local grid origin is at a timing
// pattern cross center at grid position p (zxing-cpp
// LocalGrid::isTimingPatternCross).
func (g LocalGrid) isTimingPatternCross(p bmcursor.PointI, isBlack bool, radius, errorThreshold int) bool {
	wrapOffset := func(center, offset, dim int) int {
		pos := center + offset
		if pos < 0 {
			return radius - pos
		}
		if pos >= dim {
			return -(radius + (pos - dim) + 1)
		}
		return offset
	}

	errors := 0
	for r := range radius + 1 {
		check := func(x, y int) {
			x = wrapOffset(g.Center.X, x, g.Dim.X)
			y = wrapOffset(g.Center.Y, y, g.Dim.Y)
			d := bmcursor.PointI{X: x, Y: y}
			want := bmcursor.White
			if (x+y)%2 == bool2int(!isBlack) {
				want = bmcursor.Black
			}
			if !g.findValue(p, 1, d, want) {
				errors++
			}
		}
		check(-r, 0)
		check(r, 0)
		check(0, -r)
		check(0, r)
		if errors > errorThreshold {
			return false
		}
	}
	return errors <= errorThreshold
}

func bool2int(b bool) int {
	if b {
		return 1
	}
	return 0
}

// FindTimingPatternCross searches for the center of a timing pattern cross
// near the local grid origin (zxing-cpp LocalGrid::findTimingPatternCross).
func (g *LocalGrid) FindTimingPatternCross(isBlack bool, radius int) (bmcursor.PointF, bool) {
	for _, p := range bmcursor.Spiral(3) {
		// check if there is a timing pattern cross candidate centered at p
		// with half the radius
		if g.isTimingPatternCross(p, isBlack, radius/2, 0) {
			backup := *g
			g.Origin = g.GetPosI(p)
			// adjust origin and step with full radius in all directions
			g.tuneFromTiming(bmcursor.PointI{}, "lrud", radius)

			// check again with the full radius to make sure we are correctly
			// aligned to the timing pattern
			if g.isTimingPatternCross(bmcursor.PointI{}, isBlack, radius, 0) {
				return g.Origin, true
			}
			*g = backup
		}
	}
	return bmcursor.PointF{}, false
}
