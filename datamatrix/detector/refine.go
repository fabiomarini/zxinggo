package detector

// Edge-line refinement for the Data Matrix detector.
//
// The Java-port pipeline places the four corner points by walking the
// WhiteRectangleDetector diagonals and shifting to module centers; on
// photographs with small modules the placement can be off by a full module,
// which makes the sampled grid undecodable even when the symbol was found.
// zxing-cpp's detector avoids this by fitting the symbol's four edge lines
// to the binarized image (see DMDetector.cpp / LocalGrid.cpp). This file
// ports the same idea in a minimal form: for each of the four edges (two
// solid L edges and two alternating clock-track edges) the black band along
// the edge is located at many sample positions, a line is fitted through the
// band centers, and the four fitted lines are intersected to produce refined
// module-center corner points. The dimensions are estimated two ways (clock
// -track black-band counts and edge length over run-spacing module size) and
// returned as candidates for the reader to try against the decoder.

import (
	"math"
	"sort"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/bitutil"
)

// edgeFit holds the result of fitting one symbol edge line. The fitted band
// center sits at offset(t) = a + b*t along the unit inward normal (nx, ny)
// from the current edge line, with t in [0,1] along p->q.
type edgeFit struct {
	a, b   float64
	nx, ny float64
	mod    float64 // module size estimated from run spacing (0 = not estimated)
	bands  int     // number of distinct black bands along the edge
	ok     bool
}

// edgeSample is one band-center observation: edge parameter t and measured
// offset along the normal.
type edgeSample struct{ t, off float64 }

// refineEdges fits the four symbol edge lines to the image and returns
// refined module-center corner points [topLeft, bottomLeft, bottomRight,
// topRight] plus dimension candidates to try. points/dimX/dimY come from the
// unrefined detection pipeline. ok=false means refinement failed and the
// original points should be used.
func refineEdges(image *bitutil.BitMatrix, points []zxinggo.ResultPoint, dimX, dimY int) ([]zxinggo.ResultPoint, [][2]int, bool) {
	tl, bl, br, tr := points[0], points[1], points[2], points[3]
	cx := (tl.X + bl.X + br.X + tr.X) / 4
	cy := (tl.Y + bl.Y + br.Y + tr.Y) / 4

	dimX = max(dimX, 4)
	dimY = max(dimY, 4)
	modX := distanceFloat(tl.X, tl.Y, tr.X, tr.Y) / float64(dimX-1)
	modY := distanceFloat(tl.X, tl.Y, bl.X, bl.Y) / float64(dimY-1)
	if modX < 1 || modY < 1 {
		return points, nil, false
	}

	top := fitEdge(image, tl, tr, modX, cx, cy)
	// Second pass with a corrected module size when the clock-track run
	// spacing disagrees with the initial estimate: the initial dimensions
	// come from transition counts, which are unreliable when a corner is
	// misplaced, and a wrong module size widens the search window until the
	// fits lock onto data rows.
	if top.mod > 0 && math.Abs(top.mod-modX) > 0.15*modX {
		modX = top.mod
	}
	if math.Abs(modY-modX) > 0.3*modX {
		modY = modX // square symbol
	}
	if top.mod > 0 || modY != modX {
		top = fitEdge(image, tl, tr, modX, cx, cy)
	}
	left := fitEdge(image, tl, bl, modY, cx, cy)
	bottom := fitEdge(image, bl, br, modX, cx, cy)
	right := fitEdge(image, tr, br, modY, cx, cy)
	if !top.ok || !left.ok || !bottom.ok || !right.ok {
		return points, nil, false
	}

	// Endpoints of the shifted edge lines.
	tlT := shiftAlong(tl, top, 0)
	trT := shiftAlong(tr, top, 1)
	tlL := shiftAlong(tl, left, 0)
	blL := shiftAlong(bl, left, 1)
	blB := shiftAlong(bl, bottom, 0)
	brB := shiftAlong(br, bottom, 1)
	trR := shiftAlong(tr, right, 0)
	brR := shiftAlong(br, right, 1)

	nTL, ok1 := intersect(tlT, trT, tlL, blL)
	nTR, ok2 := intersect(tlT, trT, trR, brR)
	nBR, ok3 := intersect(blB, brB, trR, brR)
	nBL, ok4 := intersect(blB, brB, tlL, blL)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return points, nil, false
	}

	// Sanity: refined corners must stay within a few modules of the old ones.
	if dist(nTL, tl) > 3*modX+3 || dist(nTR, tr) > 3*modX+3 || dist(nBR, br) > 3*modX+3 || dist(nBL, bl) > 3*modY+3 {
		return points, nil, false
	}

	// Dimension candidates: twice the clock-track band counts (exact but
	// vulnerable to missing modules in glare) and the refined edge lengths
	// over the run-spacing module size (exact but sensitive to module-size
	// error), plus the original dimensions.
	mx, my := modX, modY
	if top.mod > 0 {
		mx = top.mod
	}
	if right.mod > 0 {
		my = right.mod
	}
	cands := [][2]int{
		{2 * top.bands, 2 * right.bands},
		{roundEven(dist(nTL, nTR)/mx + 1), roundEven(dist(nTR, nBR)/my + 1)},
		{dimX, dimY},
	}
	var out [][2]int
	for _, c := range cands {
		if c[0] < 8 || c[1] < 8 || c[0] > 144 || c[1] > 144 {
			continue
		}
		dup := false
		for _, o := range out {
			if o == c {
				dup = true
			}
		}
		if !dup {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return points, nil, false
	}
	return []zxinggo.ResultPoint{nTL, nBL, nBR, nTR}, out, true
}

// fitEdge locates the black band along the line p->q at many sample
// positions and fits offset(t) = a + b*t along the inward unit normal.
func fitEdge(image *bitutil.BitMatrix, p, q zxinggo.ResultPoint, mod, cx, cy float64) edgeFit {
	dx, dy := q.X-p.X, q.Y-p.Y
	length := math.Hypot(dx, dy)
	if length < 4*mod {
		return edgeFit{}
	}
	// Unit normal pointing toward the symbol center.
	nx, ny := dy/length, -dx/length
	mx2, my2 := (p.X+q.X)/2, (p.Y+q.Y)/2
	if (cx-mx2)*nx+(cy-my2)*ny < 0 {
		nx, ny = -nx, -ny
	}

	maxOff := int(math.Round(mod)) + 2
	steps := int(length / (mod / 2))
	if steps < 8 {
		steps = 8
	}

	// Collect black runs at every sample position along the edge.
	type edgeRuns struct {
		t    float64
		runs [][2]int
	}
	var allRuns []edgeRuns
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		sx, sy := p.X+t*dx, p.Y+t*dy
		var runs [][2]int
		inRun := false
		var runStart int
		for o := -maxOff; o <= maxOff; o++ {
			px := int(math.Round(sx + float64(o)*nx))
			py := int(math.Round(sy + float64(o)*ny))
			black := px >= 0 && px < image.Width() && py >= 0 && py < image.Height() && image.Get(px, py)
			if black && !inRun {
				runStart, inRun = o, true
			} else if !black && inRun {
				runs = append(runs, [2]int{runStart, o - 1})
				inRun = false
			}
		}
		if inRun {
			runs = append(runs, [2]int{runStart, maxOff})
		}
		allRuns = append(allRuns, edgeRuns{t, runs})
	}

	// Band-center offset for a run: the band's outer edge is the robust
	// feature (binarization erosion shrinks bands inward, toward the data,
	// never outward, and merged clock-track+data runs keep their outer
	// boundary); the module center line sits half a module inside it.
	bandOff := func(r [2]int) float64 { return float64(r[0]) - 0.5 + mod/2 }

	// Initial samples: runs that contain the current line — the L line and
	// the clock track sit on it, while clock-track white gaps would latch
	// onto the more distant data rows and poison the fit.
	var samples []edgeSample
	for _, er := range allRuns {
		for _, r := range er.runs {
			if r[0] <= 0 && r[1] >= 0 {
				samples = append(samples, edgeSample{er.t, bandOff(r)})
				break
			}
		}
	}
	if len(samples) < steps/6 {
		return edgeFit{}
	}

	// Iteratively reweighted fit: refit, then re-select at every position
	// the run closest to the predicted line (within 0.6 modules) and refit
	// again. This lets a tilted initial line grow its sample coverage along
	// the whole edge instead of extrapolating a slope from one end.
	a, b := robustFit(samples, mod)
	for range 3 {
		var next []edgeSample
		for _, er := range allRuns {
			pred := a + b*er.t
			best, bestD := -1, 0.0
			for ri, r := range er.runs {
				d := math.Abs(bandOff(r) - pred)
				if best < 0 || d < bestD {
					best, bestD = ri, d
				}
			}
			if best >= 0 && bestD < 0.6*mod {
				next = append(next, edgeSample{er.t, bandOff(er.runs[best])})
			}
		}
		if len(next) < steps/6 || len(next) < len(samples) {
			break
		}
		samples = next
		a, b = robustFit(samples, mod)
	}

	// A slope fitted from a short span of the edge extrapolates wildly at
	// the far end; fall back to a pure offset (median) in that case.
	tmin, tmax := samples[0].t, samples[len(samples)-1].t
	for _, s := range samples {
		tmin, tmax = min(tmin, s.t), max(tmax, s.t)
	}
	if tmax-tmin < 0.4 {
		offs := make([]float64, len(samples))
		for i, s := range samples {
			offs[i] = s.off
		}
		a, b = median(offs), 0
	}

	f := edgeFit{a: a, b: b, nx: nx, ny: ny, ok: true}
	// Cluster the kept sample positions into black bands: samples closer
	// than 1.2*mod along the edge belong to the same module-width band.
	var pos []float64
	for _, s := range samples {
		pos = append(pos, s.t*length)
	}
	sort.Float64s(pos)
	var centers []float64
	cluster := []float64{pos[0]}
	for _, x := range pos[1:] {
		if x-cluster[len(cluster)-1] > 1.2*mod {
			centers = append(centers, mean(cluster))
			cluster = cluster[:0]
		}
		cluster = append(cluster, x)
	}
	centers = append(centers, mean(cluster))
	f.bands = len(centers)
	// Module size from clock-track band spacing: consecutive black bands are
	// two modules apart. Only meaningful for alternating edges, where the
	// kept samples cover roughly half the edge.
	if f.bands >= 4 && len(samples) <= 3*steps/4 {
		// The smallest gap between consecutive bands is the true spacing:
		// glare-hidden modules only ever double a gap, never shrink it.
		f.mod = math.MaxFloat64
		for i := 1; i < len(centers); i++ {
			if g := centers[i] - centers[i-1]; g < f.mod {
				f.mod = g
			}
		}
		f.mod /= 2
	}
	return f
}

// robustFit fits offset(t) = a + b*t through the samples: start from the
// median offset, keep samples within half a module of the line, then least
// squares, twice.
func robustFit(samples []edgeSample, mod float64) (float64, float64) {
	offs := make([]float64, len(samples))
	for i, s := range samples {
		offs[i] = s.off
	}
	a := median(offs)
	b := 0.0
	for range 2 {
		var keep []edgeSample
		for _, s := range samples {
			if math.Abs(s.off-(a+b*s.t)) < math.Max(mod/2, 1.5) {
				keep = append(keep, s)
			}
		}
		if len(keep) < 4 {
			return a, b
		}
		a, b = linFit(keep)
		samples = keep
	}
	return a, b
}

func mean(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// dist returns the Euclidean distance between two points.
func dist(a, b zxinggo.ResultPoint) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

// shiftAlong moves p by the fitted offset at parameter t along the edge's
// inward normal.
func shiftAlong(p zxinggo.ResultPoint, f edgeFit, t float64) zxinggo.ResultPoint {
	o := f.a + f.b*t
	return zxinggo.ResultPoint{X: p.X + o*f.nx, Y: p.Y + o*f.ny}
}

func intersect(p1, p2, p3, p4 zxinggo.ResultPoint) (zxinggo.ResultPoint, bool) {
	d1x, d1y := p2.X-p1.X, p2.Y-p1.Y
	d2x, d2y := p4.X-p3.X, p4.Y-p3.Y
	den := d1x*d2y - d1y*d2x
	if math.Abs(den) < 1e-6 {
		return zxinggo.ResultPoint{}, false
	}
	t := ((p3.X-p1.X)*d2y - (p3.Y-p1.Y)*d2x) / den
	return zxinggo.ResultPoint{X: p1.X + t*d1x, Y: p1.Y + t*d1y}, true
}

func linFit(pts []edgeSample) (float64, float64) {
	n := float64(len(pts))
	var st, so, stt, sto float64
	for _, s := range pts {
		st += s.t
		so += s.off
		stt += s.t * s.t
		sto += s.t * s.off
	}
	den := n*stt - st*st
	if math.Abs(den) < 1e-9 {
		return so / n, 0
	}
	b := (n*sto - st*so) / den
	a := (so - b*st) / n
	return a, b
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[len(s)/2]
}

func roundEven(f float64) int {
	n := int(math.Round(f))
	if n&1 == 1 {
		n++
	}
	return n
}
