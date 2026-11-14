package detector

// Multi-grid sampling, ported from zxing-cpp qrcode/QRDetector.cpp (SampleQR)
// and GridSampler.cpp (SampleGrid with tiled ROIs). zxing-cpp yields several
// candidate sampling grids per detection and lets QRReader decode them in
// order until one succeeds; this file provides the same structure:
//
//   - a grid tiled by alignment patterns for version 7+ symbols, which
//     absorbs lens distortion and paper curvature by giving every tile
//     between alignment patterns its own local perspective transform;
//   - the classic 3/4-point perspective grid (the Java-port path);
//   - under TryHarder, grids with each anchor nudged by ±1 pixel, which
//     recovers symbols whose anchor estimates are slightly biased.
//
// Every candidate is validated by the decoder (format-info BCH, version-info
// BCH and Reed-Solomon), so trying more grids can only add correct decodes.

import (
	"math"

	"github.com/fabiomarini/zxinggo/bitutil"
	"github.com/fabiomarini/zxinggo/internal"
	"github.com/fabiomarini/zxinggo/qrcode/decoder"
	"github.com/fabiomarini/zxinggo/transform"
)

// fpPoint is a pixel-space anchor point used during grid construction.
type fpPoint struct{ x, y float64 }

// DetectCandidates detects a QR code and returns candidate sampling grids ordered
// by decreasing plausibility. It returns nil when no finder pattern set is
// found. See the file comment for the candidate ordering.
func (d *Detector) DetectCandidates(tryHarder bool) []*internal.DetectorResult {
	finder := &finderPatternFinder{image: d.image}
	info, err := finder.find(tryHarder)
	if err != nil {
		return nil
	}
	return d.candidatesFor(info, tryHarder)
}

// candidatesFor builds the candidate sampling grids for one finder pattern
// set, mirroring the co_yield sequence of zxing-cpp SampleQR.
func (d *Detector) candidatesFor(info *FinderPatternInfo, tryHarder bool) []*internal.DetectorResult {
	topLeft := info.TopLeft
	topRight := info.TopRight
	bottomLeft := info.BottomLeft

	moduleSize := d.calculateModuleSize(topLeft, topRight, bottomLeft)
	if moduleSize < 1.0 {
		return nil
	}

	dimension, err := computeDimension(topLeft, topRight, bottomLeft, moduleSize)
	if err != nil {
		return nil
	}

	provisionalVersion, err := decoder.GetProvisionalVersionForDimension(dimension)
	if err != nil {
		return nil
	}

	var alignmentPattern *AlignmentPattern
	if len(provisionalVersion.AlignmentPatternCenters) > 0 {
		bottomRightX := topRight.X - topLeft.X + bottomLeft.X
		bottomRightY := topRight.Y - topLeft.Y + bottomLeft.Y

		modulesBetweenFPCenters := provisionalVersion.DimensionForVersion() - 7
		correctionToTopLeft := 1.0 - 3.0/float64(modulesBetweenFPCenters)
		estAlignmentX := int(topLeft.X + correctionToTopLeft*(bottomRightX-topLeft.X))
		estAlignmentY := int(topLeft.Y + correctionToTopLeft*(bottomRightY-topLeft.Y))

		for i := 4; i <= 16; i <<= 1 {
			ap := d.findAlignmentInRegion(moduleSize, estAlignmentX, estAlignmentY, float64(i))
			if ap != nil {
				alignmentPattern = ap
				break
			}
		}
	}

	var res []*internal.DetectorResult

	// Tiled grid for version 7+ (dimension >= 45): zxing-cpp SampleQR only
	// takes the tiled path when the symbol has a 3x3 or denser alignment
	// grid and the bottom-right alignment pattern was located.
	apCenters := provisionalVersion.AlignmentPatternCenters
	if alignmentPattern != nil && len(apCenters) >= 3 && dimension >= 45 {
		if tiled := d.sampleTiledGrid(topLeft, topRight, bottomLeft, alignmentPattern, apCenters, dimension, moduleSize); tiled != nil {
			res = append(res, tiled)
		}
	}

	// Classic grids (the Java-port path).
	xform := createTransform(topLeft, topRight, bottomLeft, alignmentPattern, dimension)
	sampler := &transform.DefaultGridSampler{}
	if bits, err := sampler.SampleGridTransform(d.image, dimension, dimension, xform); err == nil {
		res = append(res, internal.NewDetectorResult(bits, detectorPoints(bottomLeft, topLeft, topRight, alignmentPattern)))
	}
	if alignmentPattern != nil {
		// 3-point fallback without the alignment pattern, tried by zxing-cpp
		// when the alignment-based grid fails.
		xform3 := createTransform(topLeft, topRight, bottomLeft, nil, dimension)
		if bits, err := sampler.SampleGridTransform(d.image, dimension, dimension, xform3); err == nil {
			res = append(res, internal.NewDetectorResult(bits, detectorPoints(bottomLeft, topLeft, topRight, nil)))
		}
	}

	if tryHarder {
		res = append(res, d.jitterCandidates(topLeft, topRight, bottomLeft, alignmentPattern, dimension)...)
	}
	return res
}

// detectorPoints assembles the result points of a detector candidate in the
// order used throughout this package: bottom-left, top-left, top-right and
// optionally the alignment pattern.
func detectorPoints(bottomLeft, topLeft, topRight *FinderPattern, alignmentPattern *AlignmentPattern) []internal.ResultPoint {
	if alignmentPattern != nil {
		return []internal.ResultPoint{
			{X: bottomLeft.X, Y: bottomLeft.Y},
			{X: topLeft.X, Y: topLeft.Y},
			{X: topRight.X, Y: topRight.Y},
			{X: alignmentPattern.X, Y: alignmentPattern.Y},
		}
	}
	return []internal.ResultPoint{
		{X: bottomLeft.X, Y: bottomLeft.Y},
		{X: topLeft.X, Y: topLeft.Y},
		{X: topRight.X, Y: topRight.Y},
	}
}

// sampleTiledGrid samples the symbol with one perspective transform per tile
// of the alignment pattern grid. Port of zxing-cpp SampleQR's alignment grid
// construction plus GridSampler.cpp SampleGrid(apP, apMX, apMY).
func (d *Detector) sampleTiledGrid(topLeft, topRight, bottomLeft *FinderPattern, alignmentPattern *AlignmentPattern, apCenters []int, dimension int, moduleSize float64) *internal.DetectorResult {
	N := len(apCenters) - 1

	// Module-coordinate grid (continuous module coordinates; a module cell
	// m spans [m, m+1) so its center is m+0.5). Every grid point is the
	// module coordinate of a precisely localized pixel anchor: the three
	// finder pattern centers (3,3), (dim-4,3), (3,dim-4); the bottom-right
	// alignment pattern center (dim-7,dim-7); and the remaining alignment
	// pattern centers (apCenters+0.5). Anchoring on the centers themselves
	// (rather than zxing-cpp's fitted finder corners) avoids amplifying
	// curvature error when extrapolating into the finder regions.
	dimF := float64(dimension)
	MXG := make([][]float64, N+1)
	MYG := make([][]float64, N+1)
	for x := range MXG {
		MXG[x] = make([]float64, N+1)
		MYG[x] = make([]float64, N+1)
		for y := range MXG[x] {
			MXG[x][y] = float64(apCenters[x]) + 0.5
			MYG[x][y] = float64(apCenters[y]) + 0.5
		}
	}
	MXG[0][0], MYG[0][0] = 3.5, 3.5
	MXG[N][0], MYG[N][0] = dimF-3.5, 3.5
	MXG[0][N], MYG[0][N] = 3.5, dimF-3.5
	MXG[N][N], MYG[N][N] = dimF-6.5, dimF-6.5

	// Pixel anchors: the three finder pattern centers and the bottom-right
	// alignment pattern center.
	A := make([][]fpPoint, N+1)
	for x := range A {
		A[x] = make([]fpPoint, N+1)
	}
	A[0][0] = fpPoint{topLeft.X, topLeft.Y}
	A[N][0] = fpPoint{topRight.X, topRight.Y}
	A[0][N] = fpPoint{bottomLeft.X, bottomLeft.Y}
	A[N][N] = fpPoint{alignmentPattern.X, alignmentPattern.Y}

	xform0 := createTransform(topLeft, topRight, bottomLeft, alignmentPattern, dimension)
	project := func(x, y float64) fpPoint {
		p := []float64{x, y}
		xform0.TransformPoints(p)
		return fpPoint{p[0], p[1]}
	}

	isCorner := func(x, y int) bool {
		return (x == 0 || x == N) && (y == 0 || y == N)
	}

	// Locate the remaining alignment patterns: project the grid point with
	// the initial transform (outer ring) or extrapolate it from the three
	// already-known neighbors (interior, zxing-cpp's parallelogram rule),
	// then search for the 1:1:1 pattern around the guess.
	for y := 0; y <= N; y++ {
		for x := 0; x <= N; x++ {
			if isCorner(x, y) {
				continue
			}
			guess := project(MXG[x][y], MYG[x][y])
			if x != 0 && y != 0 {
				guess = fpPoint{
					A[x-1][y].x + A[x][y-1].x - A[x-1][y-1].x,
					A[x-1][y].y + A[x][y-1].y - A[x-1][y-1].y,
				}
			}
			if found := d.locateAlignment(moduleSize, guess); found != nil {
				A[x][y] = *found
			} else {
				A[x][y] = project(MXG[x][y], MYG[x][y])
			}
		}
	}

	// Build one ROI (module range + local transform) per tile, extending the
	// outer tiles to the symbol edges as zxing-cpp does.
	type roi struct {
		x0, x1, y0, y1 int
		t              *transform.PerspectiveTransform
	}
	rois := make([]roi, 0, N*N)
	for y := 0; y < N; y++ {
		for x := 0; x < N; x++ {
			t := transform.QuadrilateralToQuadrilateral(
				MXG[x][y], MYG[x][y], MXG[x+1][y], MYG[x+1][y], MXG[x+1][y+1], MYG[x+1][y+1], MXG[x][y+1], MYG[x][y+1],
				A[x][y].x, A[x][y].y, A[x+1][y].x, A[x+1][y].y,
				A[x+1][y+1].x, A[x+1][y+1].y, A[x][y+1].x, A[x][y+1].y)
			x0, x1 := apCenters[x], apCenters[x+1]
			if x == 0 {
				x0 = 0
			}
			if x == N-1 {
				x1 = dimension
			}
			y0, y1 := apCenters[y], apCenters[y+1]
			if y == 0 {
				y0 = 0
			}
			if y == N-1 {
				y1 = dimension
			}
			// Precheck the ROI corners (zxing-cpp GridSampler.cpp).
			corners := [][2]float64{{float64(x0) + 0.5, float64(y0) + 0.5}, {float64(x1) - 0.5, float64(y0) + 0.5}, {float64(x1) - 0.5, float64(y1) - 0.5}, {float64(x0) + 0.5, float64(y1) - 0.5}}
			for _, c := range corners {
				p := transformPoint(t, c[0], c[1])
				if !d.inBounds(int(p.x), int(p.y)) {
					return nil
				}
			}
			rois = append(rois, roi{x0, x1, y0, y1, t})
		}
	}

	bits := bitutil.NewBitMatrixWithSize(dimension, dimension)
	for _, r := range rois {
		for j := r.y0; j < r.y1; j++ {
			for i := r.x0; i < r.x1; i++ {
				p := transformPoint(r.t, float64(i)+0.5, float64(j)+0.5)
				px, py := int(p.x), int(p.y)
				if !d.inBounds(px, py) {
					return nil
				}
				if d.image.Get(px, py) {
					bits.Set(i, j)
				}
			}
		}
	}

	return internal.NewDetectorResult(bits, detectorPoints(bottomLeft, topLeft, topRight, alignmentPattern))
}

// locateAlignment searches for the 1:1:1 alignment pattern near est within a
// window of ±3 module sizes. Port of the search radius of zxing-cpp
// LocateAlignmentPattern; the actual pattern matching reuses the
// alignmentPatternFinder.
func (d *Detector) locateAlignment(moduleSize float64, est fpPoint) *fpPoint {
	half := int(3 * moduleSize)
	if half < 4 {
		half = 4
	}
	startX := max(0, int(est.x)-half)
	startY := max(0, int(est.y)-half)
	endX := min(d.image.Width()-1, int(est.x)+half)
	endY := min(d.image.Height()-1, int(est.y)+half)
	if endX-startX < int(3*moduleSize) || endY-startY < int(3*moduleSize) {
		return nil
	}
	af := &alignmentPatternFinder{
		image:      d.image,
		startX:     startX,
		startY:     startY,
		width:      endX - startX,
		height:     endY - startY,
		moduleSize: moduleSize,
	}
	ap := af.find()
	if ap == nil {
		return nil
	}
	dx, dy := ap.X-est.x, ap.Y-est.y
	if math.Abs(dx) > float64(half)+1 || math.Abs(dy) > float64(half)+1 {
		return nil
	}
	return &fpPoint{ap.X, ap.Y}
}

// jitterCandidates samples grids whose anchors are nudged by ±1 pixel, one
// anchor at a time. The decoder validates every candidate, so these only
// convert "almost right" transforms into decodes.
func (d *Detector) jitterCandidates(topLeft, topRight, bottomLeft *FinderPattern, alignmentPattern *AlignmentPattern, dimension int) []*internal.DetectorResult {
	dimMinusThree := float64(dimension) - 3.5
	var brX, brY, srcBRX, srcBRY float64
	if alignmentPattern != nil {
		brX, brY = alignmentPattern.X, alignmentPattern.Y
		srcBRX, srcBRY = dimMinusThree-3, dimMinusThree-3
	} else {
		brX = (topRight.X - topLeft.X) + bottomLeft.X
		brY = (topRight.Y - topLeft.Y) + bottomLeft.Y
		srcBRX, srcBRY = dimMinusThree, dimMinusThree
	}

	anchors := [4]fpPoint{
		{topLeft.X, topLeft.Y},
		{topRight.X, topRight.Y},
		{bottomLeft.X, bottomLeft.Y},
		{brX, brY},
	}
	srcCorners := [4]fpPoint{
		{3.5, 3.5},
		{dimMinusThree, 3.5},
		{3.5, dimMinusThree},
		{srcBRX, srcBRY},
	}

	var res []*internal.DetectorResult
	sampler := &transform.DefaultGridSampler{}
	for a := range anchors {
		for dy := -1.0; dy <= 1.0; dy++ {
			for dx := -1.0; dx <= 1.0; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				pts := anchors
				pts[a].x += dx
				pts[a].y += dy
				xform := transform.QuadrilateralToQuadrilateral(
					srcCorners[0].x, srcCorners[0].y,
					srcCorners[1].x, srcCorners[1].y,
					srcCorners[3].x, srcCorners[3].y,
					srcCorners[2].x, srcCorners[2].y,
					pts[0].x, pts[0].y, pts[1].x, pts[1].y,
					pts[3].x, pts[3].y, pts[2].x, pts[2].y)
				bits, err := sampler.SampleGridTransform(d.image, dimension, dimension, xform)
				if err != nil {
					continue
				}
				res = append(res, internal.NewDetectorResult(bits, detectorPoints(bottomLeft, topLeft, topRight, alignmentPattern)))
			}
		}
	}
	return res
}

func transformPoint(t *transform.PerspectiveTransform, x, y float64) fpPoint {
	p := []float64{x, y}
	t.TransformPoints(p)
	return fpPoint{p[0], p[1]}
}

func (d *Detector) inBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < d.image.Width() && y < d.image.Height()
}
