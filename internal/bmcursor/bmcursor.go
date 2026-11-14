// Package bmcursor ports the BitMatrixCursor.h machinery from zxing-cpp
// (core/src/BitMatrixCursor.h): integer/float cursors that walk a BitMatrix
// edge-to-edge, the FastEdgeToEdgeCounter and the Spiral point generator.
package bmcursor

import (
	"math"

	"github.com/fabiomarini/zxinggo/bitutil"
)

// PointI is an integer coordinate point (zxing-cpp PointI).
type PointI struct{ X, Y int }

// PointF is a floating point coordinate point (zxing-cpp PointF).
type PointF struct{ X, Y float64 }

// PointIF converts a PointF to a PointI by truncation, like the explicit
// PointT(PointT<T> other) constructor in zxing-cpp.
func PointIF(p PointF) PointI { return PointI{int(p.X), int(p.Y)} }

// PointFI converts a PointI to a PointF.
func PointFI(p PointI) PointF { return PointF{float64(p.X), float64(p.Y)} }

// Add returns a + b.
func (a PointI) Add(b PointI) PointI { return PointI{a.X + b.X, a.Y + b.Y} }

// Sub returns a - b.
func (a PointI) Sub(b PointI) PointI { return PointI{a.X - b.X, a.Y - b.Y} }

// Mul returns s * a.
func (a PointI) Mul(s int) PointI { return PointI{s * a.X, s * a.Y} }

// Neg returns -a.
func (a PointI) Neg() PointI { return PointI{-a.X, -a.Y} }

// Add returns a + b.
func (a PointF) Add(b PointF) PointF { return PointF{a.X + b.X, a.Y + b.Y} }

// Sub returns a - b.
func (a PointF) Sub(b PointF) PointF { return PointF{a.X - b.X, a.Y - b.Y} }

// Mul returns s * a.
func (a PointF) Mul(s float64) PointF { return PointF{s * a.X, s * a.Y} }

// Neg returns -a.
func (a PointF) Neg() PointF { return PointF{-a.X, -a.Y} }

// DotI returns the dot product of two integer points.
func DotI(a, b PointI) int { return a.X*b.X + a.Y*b.Y }

// DotF returns the dot product of two float points.
func DotF(a, b PointF) float64 { return a.X*b.X + a.Y*b.Y }

// LengthF returns the L2 norm of a float point.
func LengthF(a PointF) float64 { return math.Sqrt(DotF(a, a)) }

// DistanceF returns the distance between two float points.
func DistanceF(a, b PointF) float64 { return LengthF(a.Sub(b)) }

// DistanceFI returns the distance between two integer points.
func DistanceFI(a, b PointI) float64 { return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)) }

// CenteredI returns the pixel center of an integer pixel coordinate.
func CenteredI(p PointI) PointF { return PointF{float64(p.X) + 0.5, float64(p.Y) + 0.5} }

// CenteredF returns the center of the pixel containing p.
func CenteredF(p PointF) PointF {
	return PointF{math.Floor(p.X) + 0.5, math.Floor(p.Y) + 0.5}
}

// BresenhamI normalizes d to bresenham steps (components in {-1,0,1}).
func BresenhamI(d PointI) PointI {
	m := max(abs(d.X), abs(d.Y))
	if m == 0 {
		return PointI{}
	}
	return PointI{d.X / m, d.Y / m}
}

// BresenhamF normalizes d to bresenham steps (components in {-1,0,1}).
func BresenhamF(d PointF) PointF {
	m := math.Max(math.Abs(d.X), math.Abs(d.Y))
	if m == 0 {
		return PointF{}
	}
	return PointF{d.X / m, d.Y / m}
}

// RightI returns the point right of d (zxing-cpp right()).
func RightI(d PointI) PointI { return PointI{-d.Y, d.X} }

// LeftI returns the point left of d (zxing-cpp left()).
func LeftI(d PointI) PointI { return PointI{d.Y, -d.X} }

// CrossI returns the cross product of two integer points.
func CrossI(a, b PointI) int { return a.X*b.Y - b.X*a.Y }

// CrossF returns the cross product of two float points.
func CrossF(a, b PointF) float64 { return a.X*b.Y - b.X*a.Y }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Value is the BitMatrixCursor::Value tri-state pixel value.
type Value int

// Pixel values: invalid (outside image), white, black.
const (
	Invalid Value = -1
	White   Value = 0
	Black   Value = 1
)

// IsValid reports whether the value refers to a pixel inside the image.
func (v Value) IsValid() bool { return v != Invalid }

// IsBlack reports whether the value is a black pixel.
func (v Value) IsBlack() bool { return v == Black }

// IsWhite reports whether the value is a white pixel.
func (v Value) IsWhite() bool { return v == White }

// TestAt returns the value of the image pixel at p (Invalid outside).
func TestAt(img *bitutil.BitMatrix, p PointI) Value {
	if p.X < 0 || p.Y < 0 || p.X >= img.Width() || p.Y >= img.Height() {
		return Invalid
	}
	if img.Get(p.X, p.Y) {
		return Black
	}
	return White
}

// TestAtF returns the value of the image pixel at p. The float position must
// be inside the image (zxing-cpp isIn(PointF) semantics), the pixel itself is
// read with truncation (get(PointF) semantics).
func TestAtF(img *bitutil.BitMatrix, p PointF) Value {
	if !(0 <= p.X && p.X < float64(img.Width()) && 0 <= p.Y && p.Y < float64(img.Height())) {
		return Invalid
	}
	if img.Get(int(p.X), int(p.Y)) {
		return Black
	}
	return White
}

// IsInF reports whether the float point is inside the image.
func IsInF(img *bitutil.BitMatrix, p PointF) bool {
	return 0 <= p.X && p.X < float64(img.Width()) && 0 <= p.Y && p.Y < float64(img.Height())
}

// Direction is a left/right direction (zxing-cpp Direction enum).
type Direction int

// Left and right directions.
const (
	Left  Direction = -1
	Right Direction = 1
)

// CursorI is a discrete cursor over a BitMatrix (BitMatrixCursorI).
type CursorI struct {
	Img *bitutil.BitMatrix
	P   PointI // current position
	D   PointI // current direction
}

// NewCursorI creates a cursor at p facing direction d.
func NewCursorI(img *bitutil.BitMatrix, p, d PointI) CursorI {
	return CursorI{Img: img, P: p, D: d}
}

// Front returns the current direction.
func (c CursorI) Front() PointI { return c.D }

// Back returns the opposite of the current direction.
func (c CursorI) Back() PointI { return c.D.Neg() }

// Left returns the point left of the current direction.
func (c CursorI) Left() PointI { return LeftI(c.D) }

// Right returns the point right of the current direction.
func (c CursorI) Right() PointI { return RightI(c.D) }

// DirectionPoint returns the direction vector for dir.
func (c CursorI) DirectionPoint(dir Direction) PointI {
	return RightI(c.D).Mul(int(dir))
}

// TurnBack turns the cursor around.
func (c *CursorI) TurnBack() { c.D = c.Back() }

// TurnLeft turns the cursor left.
func (c *CursorI) TurnLeft() { c.D = c.Left() }

// TurnRight turns the cursor right.
func (c *CursorI) TurnRight() { c.D = c.Right() }

// Turn turns the cursor to dir.
func (c *CursorI) Turn(dir Direction) { c.D = c.DirectionPoint(dir) }

// SetDirection points the cursor along d (PointI overload: identity).
func (c *CursorI) SetDirection(d PointI) { c.D = d }

// Step advances the cursor by s steps; returns whether it is still inside.
func (c *CursorI) Step(s int) bool {
	c.P = c.P.Add(c.D.Mul(s))
	return TestAt(c.Img, c.P).IsValid()
}

// TurnedBack returns a copy of the cursor facing backwards.
func (c CursorI) TurnedBack() CursorI {
	return CursorI{Img: c.Img, P: c.P, D: c.Back()}
}

// StepToEdge advances the cursor one step behind the nth next edge or
// outside the image/range. It returns the number of steps taken or 0 if the
// cursor moved outside of range (ported from BitMatrixCursor::stepToEdge).
func (c *CursorI) StepToEdge(nth, rng int, backup, acceptImageBorder bool) int {
	steps := 0
	lv := TestAt(c.Img, c.P)
	for nth != 0 && (rng == 0 || steps < rng) && lv.IsValid() {
		steps++
		v := TestAt(c.Img, c.P.Add(c.D.Mul(steps)))
		if lv != v {
			nth--
		}
		lv = v
	}
	if backup && nth == 0 {
		steps--
	}
	c.P = c.P.Add(c.D.Mul(steps))
	if nth == 0 && (acceptImageBorder || lv.IsValid()) {
		return steps
	}
	return 0
}

// StepAlongEdge moves one step along the current edge, turning around
// corners (ported from BitMatrixCursor::stepAlongEdge).
func (c *CursorI) StepAlongEdge(dir Direction, skipCorner bool) bool {
	edgeAt := func(d PointI) bool {
		v := TestAt(c.Img, c.P)
		return TestAt(c.Img, c.P.Add(d)) != v
	}
	edgeDir := c.DirectionPoint(dir)
	if !edgeAt(edgeDir) {
		c.Turn(dir)
	} else if edgeAt(c.Front()) {
		c.Turn(-dir)
		if edgeAt(c.Front()) {
			c.Turn(-dir)
			if edgeAt(c.Front()) {
				return false
			}
		}
	}

	ret := c.Step(1)

	if ret && skipCorner && !edgeAt(c.DirectionPoint(dir)) {
		c.Turn(dir)
		ret = c.Step(1)
	}
	return ret
}

// CursorF is a float cursor over a BitMatrix (BitMatrixCursorF). The
// direction is always bresenham-normalized (components in {-1,0,1}).
type CursorF struct {
	Img *bitutil.BitMatrix
	P   PointF
	D   PointF
}

// NewCursorF creates a float cursor at p facing bresenham-normalized d.
func NewCursorF(img *bitutil.BitMatrix, p, d PointF) CursorF {
	return CursorF{Img: img, P: p, D: BresenhamF(d)}
}

// StepToEdge advances the float cursor to the nth next edge (ported from
// BitMatrixCursor::stepToEdge for the PointF specialization).
func (c *CursorF) StepToEdge(nth, rng int, backup, acceptImageBorder bool) int {
	steps := 0
	lv := TestAtF(c.Img, c.P)
	for nth != 0 && (rng == 0 || steps < rng) && lv.IsValid() {
		steps++
		v := TestAtF(c.Img, c.P.Add(c.D.Mul(float64(steps))))
		if lv != v {
			nth--
		}
		lv = v
	}
	if backup && nth == 0 {
		steps--
	}
	c.P = c.P.Add(c.D.Mul(float64(steps)))
	if nth == 0 && (acceptImageBorder || lv.IsValid()) {
		return steps
	}
	return 0
}

// EdgeCounter is the FastEdgeToEdgeCounter from BitMatrixCursor.h: a very
// cheap edge-to-edge step counter along a fixed direction.
type EdgeCounter struct {
	img           *bitutil.BitMatrix
	p, d          PointI
	stepsToBorder int
}

// NewEdgeCounter creates a counter starting at the cursor position.
func NewEdgeCounter(cur CursorI) EdgeCounter {
	const big = 1 << 30
	d := cur.D
	maxStepsX, maxStepsY := big, big
	if d.X > 0 {
		maxStepsX = cur.Img.Width() - 1 - cur.P.X
	} else if d.X < 0 {
		maxStepsX = cur.P.X
	}
	if d.Y > 0 {
		maxStepsY = cur.Img.Height() - 1 - cur.P.Y
	} else if d.Y < 0 {
		maxStepsY = cur.P.Y
	}
	stepsToBorder := maxStepsX
	if maxStepsY < stepsToBorder {
		stepsToBorder = maxStepsY
	}
	return EdgeCounter{img: cur.Img, p: cur.P, d: d, stepsToBorder: stepsToBorder}
}

// StepToNextEdge moves to the next pixel with a different value, at most
// range steps; returns the steps taken or 0 if the range was exhausted
// before the edge (or the counter started outside the image).
func (c *EdgeCounter) StepToNextEdge(rng int) int {
	maxSteps := c.stepsToBorder
	if rng < maxSteps {
		maxSteps = rng
	}
	steps := 0
	if maxSteps < 0 { // already outside the image
		return 0
	}
	base := c.img.Get(c.p.X, c.p.Y)
	for {
		steps++
		if steps > maxSteps {
			if maxSteps == c.stepsToBorder {
				break // hit the image border: accept
			}
			return 0
		}
		if c.img.Get(c.p.X+steps*c.d.X, c.p.Y+steps*c.d.Y) != base {
			break
		}
	}
	c.p = c.p.Add(c.d.Mul(steps))
	c.stepsToBorder -= steps
	return steps
}

// Spiral returns the points in a spiral pattern around the origin, up to the
// given radius (ported from BitMatrixCursor.h Spiral()).
func Spiral(radius int) []PointI {
	res := []PointI{{0, 0}}
	for r := 1; r <= radius; r++ {
		for k := range 2 * r {
			res = append(res, PointI{r, -(r - 1) + k}) // right -> down
		}
		for k := range 2 * r {
			res = append(res, PointI{(r - 1) - k, r}) // bottom -> left
		}
		for k := range 2 * r {
			res = append(res, PointI{-r, (r - 1) - k}) // left -> up
		}
		for k := range 2 * r {
			res = append(res, PointI{-(r - 1) + k, -r}) // top -> right
		}
	}
	return res
}
