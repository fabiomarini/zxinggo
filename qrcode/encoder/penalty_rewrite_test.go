package encoder

import (
	"math/rand"
	"testing"
)

// naive reference: old Get-based semantics
func refRule1(m *ByteMatrix) int {
	p := 0
	for i := 0; i < m.Height; i++ {
		n, prev := 0, byte(255)
		for j := 0; j < m.Width; j++ {
			b := m.Get(j, i)
			if b == prev {
				n++
			} else {
				if n >= 5 {
					p += 3 + n - 5
				}
				n = 1
				prev = b
			}
		}
		if n >= 5 {
			p += 3 + n - 5
		}
	}
	for i := 0; i < m.Width; i++ {
		n, prev := 0, byte(255)
		for j := 0; j < m.Height; j++ {
			b := m.Get(i, j)
			if b == prev {
				n++
			} else {
				if n >= 5 {
					p += 3 + n - 5
				}
				n = 1
				prev = b
			}
		}
		if n >= 5 {
			p += 3 + n - 5
		}
	}
	return p
}

func refRule2(m *ByteMatrix) int {
	p := 0
	for y := 0; y < m.Height-1; y++ {
		for x := 0; x < m.Width-1; x++ {
			v := m.Get(x, y)
			if v == m.Get(x+1, y) && v == m.Get(x, y+1) && v == m.Get(x+1, y+1) {
				p += 3
			}
		}
	}
	return p
}

func refRule3(m *ByteMatrix) int {
	p := 0
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			if x+6 < m.Width && m.Get(x, y) == 1 && m.Get(x+1, y) == 0 && m.Get(x+2, y) == 1 &&
				m.Get(x+3, y) == 1 && m.Get(x+4, y) == 1 && m.Get(x+5, y) == 0 && m.Get(x+6, y) == 1 {
				lw := x+10 < m.Width && m.Get(x+7, y) == 0 && m.Get(x+8, y) == 0 && m.Get(x+9, y) == 0 && m.Get(x+10, y) == 0
				tw := x >= 4 && m.Get(x-1, y) == 0 && m.Get(x-2, y) == 0 && m.Get(x-3, y) == 0 && m.Get(x-4, y) == 0
				if lw || tw {
					p += 40
				}
			}
			if y+6 < m.Height && m.Get(x, y) == 1 && m.Get(x, y+1) == 0 && m.Get(x, y+2) == 1 &&
				m.Get(x, y+3) == 1 && m.Get(x, y+4) == 1 && m.Get(x, y+5) == 0 && m.Get(x, y+6) == 1 {
				lw := y+10 < m.Height && m.Get(x, y+7) == 0 && m.Get(x, y+8) == 0 && m.Get(x, y+9) == 0 && m.Get(x, y+10) == 0
				tw := y >= 4 && m.Get(x, y-1) == 0 && m.Get(x, y-2) == 0 && m.Get(x, y-3) == 0 && m.Get(x, y-4) == 0
				if lw || tw {
					p += 40
				}
			}
		}
	}
	return p
}

func refRule4(m *ByteMatrix) int {
	d := 0
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			if m.Get(x, y) == 1 {
				d++
			}
		}
	}
	t := m.Height * m.Width
	return abs(d*2-t) * 10 / t * 10
}

func TestPenaltyRewriteMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, size := range []int{21, 25, 29, 45, 77, 101} {
		for trial := 0; trial < 20; trial++ {
			m := NewByteMatrix(size, size)
			for i := range m.Data {
				switch rng.Intn(10) {
				case 0, 1, 2:
					m.Data[i] = 0
				case 3, 4, 5:
					m.Data[i] = 1
				default:
					m.Data[i] = 0xFF
				}
			}
			if got, want := applyMaskPenaltyRule1(m), refRule1(m); got != want {
				t.Fatalf("rule1 size %d: got %d want %d", size, got, want)
			}
			if got, want := applyMaskPenaltyRule2(m), refRule2(m); got != want {
				t.Fatalf("rule2 size %d: got %d want %d", size, got, want)
			}
			if got, want := applyMaskPenaltyRule3(m), refRule3(m); got != want {
				t.Fatalf("rule3 size %d: got %d want %d", size, got, want)
			}
			if got, want := applyMaskPenaltyRule4(m), refRule4(m); got != want {
				t.Fatalf("rule4 size %d: got %d want %d", size, got, want)
			}
		}
	}
}
