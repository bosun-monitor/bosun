package txtg

import (
	"log"
)

// Different edge styles for boxes
var Edge = [][4]rune{{'+', '+', '+', '+'}, {'.', '.', '\'', '\''}, {'/', '\\', '\\', '/'}}

// A Text Buffer
type TextBuf struct {
	Buf  []rune // the internal buffer.  Point (x,y) is mapped to x + y*(W+1)
	W, H int    // Width and Height
}

// Set up a new TextBuf with width w and height h.
func NewTextBuf(w, h int) (tb *TextBuf) {
	tb = new(TextBuf)
	tb.W, tb.H = w, h
	tb.Buf = make([]rune, (w+1)*h)
	for i, _ := range tb.Buf {
		tb.Buf[i] = ' '
	}
	for i := 0; i < h; i++ {
		tb.Buf[i*(w+1)+w] = '\n'
	}
	// tb.Buf[0], tb.Buf[(w+1)*h-1] = 'X', 'X'
	return
}

// Put character c at (x,y)
func (tb *TextBuf) Put(x, y int, c rune) {
	if x < 0 || y < 0 || x >= tb.W || y >= tb.H || c < ' ' {
		// debug.Printf("Ooooops Put(): %d, %d, %d='%c' \n", x, y, c, c)
		return
	}
	i := (tb.W+1)*y + x
	tb.Buf[i] = c
}

// Draw rectangle of width w and height h from corner at (x,y).
// Use one of the corner style defined in Edge.
// Interior is filled with charater fill iff != 0.
func (tb *TextBuf) Rect(x, y, w, h int, style int, fill rune) {
	style = style % len(Edge)

	if h < 0 {
		h = -h
		y -= h
	}
	if w < 0 {
		w = -w
		x -= w
	}

	tb.Put(x, y, Edge[style][0])
	tb.Put(x+w, y, Edge[style][1])
	tb.Put(x, y+h, Edge[style][2])
	tb.Put(x+w, y+h, Edge[style][3])
	for i := 1; i < w; i++ {
		tb.Put(x+i, y, '-')
		tb.Put(x+i, y+h, '-')
	}
	for i := 1; i < h; i++ {
		tb.Put(x+w, y+i, '|')
		tb.Put(x, y+i, '|')
		if fill > 0 {
			for j := x + 1; j < x+w; j++ {
				tb.Put(j, y+i, fill)
			}
		}
	}
}

func (tb *TextBuf) Block(x, y, w, h int, fill rune) {
	if h < 0 {
		h = -h
		y -= h
	}
	if w < 0 {
		w = -w
		x -= w
	}
	for i := 0; i < w; i++ {
		for j := 0; j <= h; j++ {
			tb.Put(x+i, y+j, fill)
		}
	}
}

// Return real character len of s (rune count).
func StrLen(s string) (n int) {
	for _, _ = range s {
		n++
	}
	return
}

// Print text txt at (x,y). Horizontal display for align in [-1,1],
// vasrtical alignment for align in [2,4]
// align: -1: left; 0: centered; 1: right; 2: top, 3: center, 4: bottom
func (tb *TextBuf) Text(x, y int, txt string, align int) {
	if align <= 1 {
		switch align {
		case 0:
			x -= StrLen(txt) / 2
		case 1:
			x -= StrLen(txt)
		}
		i := 0
		for _, r := range txt {
			tb.Put(x+i, y, r)
			i++
		}
	} else {
		switch align {
		case 3:
			y -= StrLen(txt) / 2
		case 2:
			x -= StrLen(txt)
		}
		i := 0
		for _, r := range txt {
			tb.Put(x, y+i, r)
			i++
		}
	}
}

// Paste buf at (x,y)
func (tb *TextBuf) Paste(x, y int, buf *TextBuf) {
	s := buf.W + 1
	for i := 0; i < buf.W; i++ {
		for j := 0; j < buf.H; j++ {
			tb.Put(x+i, y+j, buf.Buf[i+s*j])
		}
	}
}

func (tb *TextBuf) Line(x0, y0, x1, y1 int, symbol rune) {
	// handle trivial cases first
	if x0 == x1 {
		if y0 > y1 {
			y0, y1 = y1, y0
		}
		for ; y0 <= y1; y0++ {
			tb.Put(x0, y0, symbol)
		}
		return
	}
	if y0 == y1 {
		if x0 > x1 {
			x0, x1 = x1, x0
		}
		for ; x0 <= x1; x0++ {
			tb.Put(x0, y0, symbol)
		}
		return
	}
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	err, e2 := dx+dy, 0
	for {
		tb.Put(x0, y0, symbol)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 = 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}

	}
}

// Convert to plain (utf8) string.
func (tb *TextBuf) String() string {
	return string(tb.Buf)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func sign(a int) int {
	if a < 0 {
		return -1
	}
	if a == 0 {
		return 0
	}
	return 1
}

// Debugging and tracing
type debugging bool

const debug debugging = true

func (d debugging) Printf(fmt string, args ...interface{}) {
	if d {
		log.Printf(fmt, args...)
	}
}
