package imgg

import (
	"image"
	"image/color"
	"log"
	"math"

	"bosun.org/_third_party/github.com/golang/freetype"
	"bosun.org/_third_party/github.com/golang/freetype/truetype"
	"bosun.org/_third_party/github.com/llgcode/draw2d"
	"bosun.org/_third_party/github.com/llgcode/draw2d/draw2dimg"
	"bosun.org/_third_party/github.com/vdobler/chart"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
)

var (
	dpi         = 72.0
	defaultFont *truetype.Font
)

func init() {
	var err error
	defaultFont, err = freetype.ParseFont(defaultFontData())
	if err != nil {
		panic(err)
	}
}

// ImageGraphics writes plot to an image.RGBA
type ImageGraphics struct {
	Image  *image.RGBA // The image the plots are drawn onto.
	x0, y0 int
	w, h   int
	bg     color.RGBA
	gc     draw2d.GraphicContext
	font   *truetype.Font
	fs     map[chart.FontSize]float64
}

// New creates a new ImageGraphics including an image.RGBA of dimension w x h
// with background bgcol. If font is nil it will use a builtin font.
// If fontsize is empty useful default are used.
func New(width, height int, bgcol color.RGBA, font *truetype.Font, fontsize map[chart.FontSize]float64) *ImageGraphics {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	gc := draw2dimg.NewGraphicContext(img)
	gc.SetLineJoin(draw2d.BevelJoin)
	gc.SetLineCap(draw2d.SquareCap)
	gc.SetStrokeColor(image.Black)
	gc.SetFillColor(bgcol)
	gc.Translate(0.5, 0.5)
	gc.Clear()
	if font == nil {
		font = defaultFont
	}
	if len(fontsize) == 0 {
		fontsize = ConstructFontSizes(13)
	}
	return &ImageGraphics{Image: img, x0: 0, y0: 0, w: width, h: height,
		bg: bgcol, gc: gc, font: font, fs: fontsize}
}

// AddTo returns a new ImageGraphics which will write to (width x height) sized
// area starting at (x,y) on the provided image img. The rest of the parameters
// are the same as in New().
func AddTo(img *image.RGBA, x, y, width, height int, bgcol color.RGBA, font *truetype.Font, fontsize map[chart.FontSize]float64) *ImageGraphics {
	gc := draw2dimg.NewGraphicContext(img)
	gc.SetLineJoin(draw2d.BevelJoin)
	gc.SetLineCap(draw2d.SquareCap)
	gc.SetStrokeColor(image.Black)
	gc.SetFillColor(bgcol)
	gc.Translate(float64(x)+0.5, float64(y)+0.5)
	gc.ClearRect(x, y, x+width, y+height)
	if font == nil {
		font = defaultFont
	}
	if len(fontsize) == 0 {
		fontsize = ConstructFontSizes(13)
	}

	return &ImageGraphics{Image: img, x0: x, y0: y, w: width, h: height, bg: bgcol, gc: gc, font: font, fs: fontsize}
}

func (ig *ImageGraphics) Options() chart.PlotOptions { return nil }

func (ig *ImageGraphics) Begin() {}
func (ig *ImageGraphics) End()   {}

func (ig *ImageGraphics) Background() (r, g, b, a uint8) { return ig.bg.R, ig.bg.G, ig.bg.B, ig.bg.A }
func (ig *ImageGraphics) Dimensions() (int, int)         { return ig.w, ig.h }
func (ig *ImageGraphics) FontMetrics(font chart.Font) (fw float32, fh int, mono bool) {
	h := ig.relFontsizeToPixel(font.Size)
	// typical width is 0.6 * height
	fw = float32(0.6 * h)
	fh = int(h + 0.5)
	mono = true
	return
}
func (ig *ImageGraphics) TextLen(s string, font chart.Font) int {
	c := freetype.NewContext()
	c.SetDPI(dpi)
	c.SetFont(ig.font)
	fontsize := ig.relFontsizeToPixel(font.Size)
	c.SetFontSize(fontsize)

	// really draw it
	width, err := c.DrawString(s, freetype.Pt(0, 0))
	if err != nil {
		return 10 * len(s) // BUG
	}
	return int(width.X+32)>>6 + 1
}

func (ig *ImageGraphics) setStyle(style chart.Style) {
	ig.gc.SetStrokeColor(style.LineColor)
	ig.gc.SetLineWidth(float64(style.LineWidth))
	orig := dashPattern[style.LineStyle]
	pattern := make([]float64, len(orig))
	copy(pattern, orig)
	for i := range pattern {
		pattern[i] *= math.Sqrt(float64(style.LineWidth))
	}
	ig.gc.SetLineDash(pattern, 0)
}

func (ig *ImageGraphics) Line(x0, y0, x1, y1 int, style chart.Style) {
	if style.LineWidth <= 0 {
		style.LineWidth = 1
	}
	ig.setStyle(style)
	ig.gc.MoveTo(float64(x0), float64(y0))
	ig.gc.LineTo(float64(x1), float64(y1))
	ig.gc.Stroke()
}

var dashPattern map[chart.LineStyle][]float64 = map[chart.LineStyle][]float64{
	chart.SolidLine:      nil, // []float64{10},
	chart.DashedLine:     []float64{10, 4},
	chart.DottedLine:     []float64{4, 3},
	chart.DashDotDotLine: []float64{10, 3, 3, 3, 3, 3},
	chart.LongDashLine:   []float64{10, 8},
	chart.LongDotLine:    []float64{4, 8},
}

func (ig *ImageGraphics) Path(x, y []int, style chart.Style) {
	ig.setStyle(style)
	ig.gc.MoveTo(float64(x[0]), float64(y[0]))
	for i := 1; i < len(x); i++ {
		ig.gc.LineTo(float64(x[i]), float64(y[i]))
	}
	ig.gc.Stroke()
}

func (ig *ImageGraphics) relFontsizeToPixel(rel chart.FontSize) float64 {
	if s, ok := ig.fs[rel]; ok {
		return s
	}
	return 12
}

func ConstructFontSizes(basesize float64) map[chart.FontSize]float64 {
	size := make(map[chart.FontSize]float64)
	for rs := int(chart.TinyFontSize); rs <= int(chart.HugeFontSize); rs++ {
		size[chart.FontSize(rs)] = basesize * math.Pow(1.2, float64(rs))
	}
	return size
}

func (ig *ImageGraphics) Text(x, y int, t string, align string, rot int, f chart.Font) {
	if len(align) == 1 {
		align = "c" + align
	}

	var col color.Color
	if f.Color != nil {
		col = f.Color
	} else {
		col = color.RGBA{0, 0, 0, 0xff}
	}

	textImage := ig.textBox(t, f, col)
	bounds := textImage.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	var centerX, centerY int

	if rot != 0 {
		alpha := float64(rot) / 180 * math.Pi
		cos := math.Cos(alpha)
		sin := math.Sin(alpha)

		ax, ay := float64(w), float64(h) // anchor point
		switch align[0] {
		case 'b':
		case 'c':
			ay /= 2
		case 't':
			ay = 0
		}
		switch align[1] {
		case 'l':
			ax = 0
		case 'c':
			ax /= 2
		case 'r':
		}
		dx := float64(ax)*cos + float64(ay)*sin
		dy := -float64(ax)*sin + float64(ay)*cos
		trans := f64.Aff3{
			+cos, +sin, float64(x+ig.x0) - dx,
			-sin, +cos, float64(y+ig.y0) - dy,
		}
		draw.BiLinear.Transform(ig.Image, trans,
			textImage, textImage.Bounds(), draw.Over, nil)
		return
	} else {
		centerX, centerY = w/2, h/2
		switch align[0] {
		case 'b':
			centerY = h
		case 't':
			centerY = 0
		}
		switch align[1] {
		case 'l':
			centerX = 0
		case 'r':
			centerX = w
		}
	}

	bounds = textImage.Bounds()
	w, h = bounds.Dx(), bounds.Dy()
	x -= centerX
	y -= centerY
	x += ig.x0
	y += ig.y0

	tcol := image.NewUniform(col)
	draw.DrawMask(ig.Image, image.Rect(x, y, x+w, y+h), tcol, image.ZP,
		textImage, textImage.Bounds().Min, draw.Over)
}

// textBox renders t into a tight fitting image
func (ig *ImageGraphics) textBox(t string, font chart.Font, textCol color.Color) image.Image {
	// Initialize the context.
	bg := image.NewUniform(color.Alpha{0})
	fg := image.NewUniform(textCol)
	width := ig.TextLen(t, font)
	size := ig.relFontsizeToPixel(font.Size)

	c := freetype.NewContext()
	c.SetDPI(dpi)
	c.SetFont(ig.font)
	c.SetFontSize(size)
	bb := ig.font.Bounds(c.PointToFixed(float64(size)))
	bbDelta := bb.Max.Sub(bb.Min)

	height := int(bbDelta.Y+32) >> 6
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), bg, image.ZP, draw.Src)
	c.SetDst(canvas)
	c.SetSrc(fg)
	c.SetClip(canvas.Bounds())
	// Draw the text.
	extent, err := c.DrawString(t, fixed.Point26_6{X: 0, Y: bb.Max.Y})
	if err != nil {
		log.Println(err)
		return nil
	}

	// Ugly heuristic hack: font bounds are pretty high resulting in white top border: Trim.
	topskip := 1
	if size > 15 {
		topskip = 2
	} else if size > 20 {
		topskip = 3
	}
	return canvas.SubImage(image.Rect(0, topskip, int(extent.X)>>6, height))
}

func (ig *ImageGraphics) paint(x, y int, R, G, B uint32, alpha uint32) {
	r, g, b, a := ig.Image.At(x, y).RGBA()
	r >>= 8
	g >>= 8
	b >>= 8
	a >>= 8
	r *= alpha
	g *= alpha
	b *= alpha
	a *= alpha
	r += R * (0xff - alpha)
	g += G * (0xff - alpha)
	b += B * (0xff - alpha)
	r >>= 8
	g >>= 8
	b >>= 8
	a >>= 8
	ig.Image.Set(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)})
}

func (ig *ImageGraphics) Symbol(x, y int, style chart.Style) {
	chart.GenericSymbol(ig, x, y, style)
}

func (ig *ImageGraphics) Rect(x, y, w, h int, style chart.Style) {
	ig.setStyle(style)
	stroke := func() { ig.gc.Stroke() }
	if style.FillColor != nil {
		ig.gc.SetFillColor(style.FillColor)
		stroke = func() { ig.gc.FillStroke() }
	}
	ig.gc.MoveTo(float64(x), float64(y))
	ig.gc.LineTo(float64(x+w), float64(y))
	ig.gc.LineTo(float64(x+w), float64(y+h))
	ig.gc.LineTo(float64(x), float64(y+h))
	ig.gc.LineTo(float64(x), float64(y))
	stroke()
}

func (ig *ImageGraphics) Wedge(ix, iy, iro, iri int, phi, psi float64, style chart.Style) {
	ig.setStyle(style)
	stroke := func() { ig.gc.Stroke() }
	if style.FillColor != nil {
		ig.gc.SetFillColor(style.FillColor)
		stroke = func() { ig.gc.FillStroke() }
	}

	ecc := 1.0                           // eccentricity
	x, y := float64(ix), float64(iy)     // center as float
	ro, ri := float64(iro), float64(iri) // radius outer and inner as float
	roe, rie := ro*ecc, ri*ecc           // inner and outer radius corrected by ecc

	xao, yao := math.Cos(phi)*roe+x, y+math.Sin(phi)*ro
	// xco, yco := math.Cos(psi)*roe+x, y-math.Sin(psi)*ro
	xai, yai := math.Cos(phi)*rie+x, y+math.Sin(phi)*ri
	xci, yci := math.Cos(psi)*rie+x, y+math.Sin(psi)*ri

	// outbound straight line
	if ri > 0 {
		ig.gc.MoveTo(xai, yai)
	} else {
		ig.gc.MoveTo(x, y)
	}
	ig.gc.LineTo(xao, yao)

	// outer arc
	ig.gc.ArcTo(x, y, ro, roe, phi, psi-phi)

	// inbound straight line
	if ri > 0 {
		ig.gc.LineTo(xci, yci)
		ig.gc.ArcTo(x, y, ri, rie, psi, phi-psi)
	} else {
		ig.gc.LineTo(x, y)
	}
	stroke()
}

func (ig *ImageGraphics) XAxis(xr chart.Range, ys, yms int, options chart.PlotOptions) {
	chart.GenericXAxis(ig, xr, ys, yms, options)
}
func (ig *ImageGraphics) YAxis(yr chart.Range, xs, xms int, options chart.PlotOptions) {
	chart.GenericYAxis(ig, yr, xs, xms, options)
}

func (ig *ImageGraphics) Scatter(points []chart.EPoint, plotstyle chart.PlotStyle, style chart.Style) {
	chart.GenericScatter(ig, points, plotstyle, style)
}

func (ig *ImageGraphics) Boxes(boxes []chart.Box, width int, style chart.Style) {
	chart.GenericBoxes(ig, boxes, width, style)
}

func (ig *ImageGraphics) Key(x, y int, key chart.Key, options chart.PlotOptions) {
	chart.GenericKey(ig, x, y, key, options)
}

func (ig *ImageGraphics) Bars(bars []chart.Barinfo, style chart.Style) {
	chart.GenericBars(ig, bars, style)
}

func (ig *ImageGraphics) Rings(wedges []chart.Wedgeinfo, x, y, ro, ri int) {
	chart.GenericRings(ig, wedges, x, y, ro, ri, 1)
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

var _ chart.Graphics = &ImageGraphics{}
